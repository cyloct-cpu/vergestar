import { StateManager, type ChapterMeta } from "@actalk/inkos-core";
import { mkdir, readFile, readdir, rm, stat, writeFile } from "node:fs/promises";
import path from "node:path";
import { readBookArtifacts, type FullNovelAgentTurnResult } from "./full-agent.js";

// InkOS BookDetail 管理操作（通过/拒绝回滚/书籍设置/删除/真相文件/导出回传/质量工具）
// 的 Bridge 内部实现。与 studio 服务端同语义：approve/reject 只改章节索引，
// reject 回滚到上一章；设置写 book.json；删除为整目录移除。产物回传给后端
// 重投影，保持 DB 单一真相。

const workspaceRoot = process.env.VERGESTAR_NOVEL_WORKSPACE_ROOT || path.resolve("data");

function safeWorkspacePath(userId: string, sessionId: string): string {
    if (!/^[A-Za-z0-9_-]{1,64}$/.test(userId) || !/^[A-Za-z0-9_-]{1,120}$/.test(sessionId)) {
        throw new Error("工作区路径参数无效");
    }
    return path.join(workspaceRoot, userId, sessionId);
}

function assertSafeBookId(bookId: string): void {
    if (!bookId.trim() || bookId.includes("/") || bookId.includes("\\") || bookId.includes("..") || bookId.length > 160) {
        throw new Error("书籍 ID 无效");
    }
}

export async function approveChapter(userId: string, sessionId: string, bookId: string, chapterNumber: number): Promise<FullNovelAgentTurnResult["artifacts"]> {
    assertSafeBookId(bookId);
    if (!Number.isInteger(chapterNumber) || chapterNumber <= 0) throw new Error("章节号无效");
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const state = new StateManager(projectRoot);
    const index = await state.loadChapterIndex(bookId);
    if (!index.some((chapter) => chapter.number === chapterNumber)) {
        throw new Error(`第 ${chapterNumber} 章不存在，无法通过`);
    }
    const updated = index.map((chapter) => (chapter.number === chapterNumber ? { ...chapter, status: "approved" as const } : chapter)) as ReadonlyArray<ChapterMeta>;
    await state.saveChapterIndex(bookId, updated);
    return await readBookArtifacts(projectRoot, bookId, sessionId, []);
}

export async function rejectChapter(userId: string, sessionId: string, bookId: string, chapterNumber: number): Promise<{ ok: true; chapterNumber: number; status: "rejected"; rolledBackTo: number; discarded: unknown; artifacts: FullNovelAgentTurnResult["artifacts"] }> {
    assertSafeBookId(bookId);
    if (!Number.isInteger(chapterNumber) || chapterNumber <= 0) throw new Error("章节号无效");
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const state = new StateManager(projectRoot);
    const index = await state.loadChapterIndex(bookId);
    if (!index.some((chapter) => chapter.number === chapterNumber)) {
        throw new Error(`第 ${chapterNumber} 章不存在，无法拒绝`);
    }
    const rollbackTarget = chapterNumber - 1;
    const discarded = await state.rollbackToChapter(bookId, rollbackTarget);
    return { ok: true, chapterNumber, status: "rejected", rolledBackTo: rollbackTarget, discarded, artifacts: await readBookArtifacts(projectRoot, bookId, sessionId, []) };
}

export interface BookSettingsUpdate {
    readonly chapterWordCount?: number;
    readonly targetChapters?: number;
    readonly status?: string;
    readonly chapterReviewMode?: "auto" | "manual";
}

export async function updateBookSettings(userId: string, sessionId: string, bookId: string, update: BookSettingsUpdate): Promise<Record<string, unknown>> {
    assertSafeBookId(bookId);
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const state = new StateManager(projectRoot);
    const config = await state.loadBookConfig(bookId);
    if (typeof update.chapterWordCount === "number" && update.chapterWordCount > 0) config.chapterWordCount = update.chapterWordCount;
    if (typeof update.targetChapters === "number" && update.targetChapters > 0) config.targetChapters = update.targetChapters;
    if (typeof update.status === "string" && update.status) (config as Record<string, unknown>).status = update.status;
    if (update.chapterReviewMode === "auto" || update.chapterReviewMode === "manual") {
        const writing = (config.writing && typeof config.writing === "object" ? { ...config.writing } : {}) as Record<string, unknown>;
        writing.reviewMode = update.chapterReviewMode;
        (config as Record<string, unknown>).writing = writing;
    }
    await state.saveBookConfig(bookId, config);
    return config as unknown as Record<string, unknown>;
}

export async function deleteBook(userId: string, sessionId: string, bookId: string): Promise<{ ok: true; bookId: string }> {
    assertSafeBookId(bookId);
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const state = new StateManager(projectRoot);
    await rm(state.bookDir(bookId), { recursive: true, force: true });
    return { ok: true, bookId };
}

// --- 真相文件读写（T2）---

// 与 InkOS Studio 同一白名单：story/ 下的扁平文件、outline/ 权威大纲、角色卡。
const TRUTH_ALLOWED = new Set([
    "author_intent.md", "current_focus.md", "story_bible.md", "book_rules.md", "volume_outline.md",
    "current_state.md", "particle_ledger.md", "pending_hooks.md", "chapter_summaries.md",
    "subplot_board.md", "emotional_arcs.md", "character_matrix.md", "style_guide.md",
    "parent_canon.md", "fanfic_canon.md",
    "outline/story_frame.md", "outline/volume_map.md", "outline/节奏原则.md", "outline/rhythm_principles.md",
]);

function resolveTruthPath(projectRoot: string, bookId: string, truthPath: string): string | null {
    if (!truthPath || truthPath.includes("\\") || truthPath.includes("..") || truthPath.includes("\0") || truthPath.startsWith("/")) return null;
    const normalized = truthPath.replace(/\\/g, "/");
    // 兼容两种形式：相对 story/ 目录（pending_hooks.md，InkOS Studio 语义）
    // 与含 story/ 前缀的完整相对路径（story/pending_hooks.md，Vergestar FilesJSON 语义）。
    const relative = normalized.startsWith("story/") ? normalized.slice("story/".length) : normalized;
    const allowed = TRUTH_ALLOWED.has(relative)
        || TRUTH_ALLOWED.has(normalized)
        || /^roles\/(主要角色|次要角色|major|minor)\/[^/]+\.md$/.test(relative)
        || (relative.startsWith("outline/") && relative.endsWith(".md"));
    if (!allowed) return null;
    return path.join(projectRoot, "books", bookId, "story", ...relative.split("/"));
}

export async function readTruthFile(userId: string, sessionId: string, bookId: string, truthPath: string): Promise<{ content: string }> {
    assertSafeBookId(bookId);
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const resolved = resolveTruthPath(projectRoot, bookId, truthPath);
    if (!resolved) throw new Error("无效的真相文件路径");
    try {
        return { content: await readFile(resolved, "utf8") };
    } catch {
        return { content: "" };
    }
}

export async function writeTruthFile(userId: string, sessionId: string, bookId: string, truthPath: string, content: string): Promise<{ ok: true }> {
    assertSafeBookId(bookId);
    if (typeof content !== "string" || Buffer.byteLength(content, "utf8") > 512 * 1024) throw new Error("真相文件内容无效或超过 512KB");
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const resolved = resolveTruthPath(projectRoot, bookId, truthPath);
    if (!resolved) throw new Error("无效的真相文件路径");
    await mkdir(path.dirname(resolved), { recursive: true });
    await writeFile(resolved, content, "utf8");
    return { ok: true };
}

// --- 导出文件回传（T2）：列出并读取 exports/ 下的导出产物 ---

function resolveExportPath(projectRoot: string, exportPath: string): string | null {
    if (!exportPath || exportPath.includes("\\") || exportPath.includes("..") || exportPath.startsWith("/")) return null;
    const normalized = exportPath.replace(/\\/g, "/");
    if (!normalized.startsWith("exports/") || !(normalized.endsWith(".txt") || normalized.endsWith(".md") || normalized.endsWith(".epub"))) return null;
    return path.join(projectRoot, ...normalized.split("/"));
}

export async function listExportFiles(userId: string, sessionId: string): Promise<Array<{ path: string; size: number; modifiedAt: string }>> {
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const result: Array<{ path: string; size: number; modifiedAt: string }> = [];
    // exporter 产物有两处：exports/ 目录（旧约定）与工作区根的 *_export.{txt,md,epub}（sub_agent exporter 实际落点）。
    async function walk(dir: string, prefix: string): Promise<void> {
        let entries;
        try {
            entries = await readdir(dir, { withFileTypes: true });
        } catch {
            return;
        }
        for (const entry of entries) {
            const full = path.join(dir, entry.name);
            const rel = prefix ? prefix + "/" + entry.name : entry.name;
            if (entry.isDirectory()) {
                await walk(full, rel);
            } else if (/.(txt|md|epub)$/i.test(entry.name)) {
                const info = await stat(full);
                result.push({ path: prefix ? prefix + "/" + entry.name : entry.name, size: info.size, modifiedAt: info.mtime.toISOString() });
            }
        }
    }
    await walk(path.join(projectRoot, "exports"), "exports");
    try {
        const rootEntries = await readdir(projectRoot, { withFileTypes: true });
        for (const entry of rootEntries) {
            if (entry.isFile() && /_export.(txt|md|epub)$/i.test(entry.name)) {
                const info = await stat(path.join(projectRoot, entry.name));
                result.push({ path: entry.name, size: info.size, modifiedAt: info.mtime.toISOString() });
            }
        }
    } catch {
        // 工作区根不可读时忽略
    }
    return result;
}

export async function readExportFile(userId: string, sessionId: string, exportPath: string): Promise<{ content: Buffer }> {
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const resolved = resolveExportPath(projectRoot, exportPath);
    if (!resolved) throw new Error("无效的导出文件路径");
    return { content: await readFile(resolved) };
}

// --- 质量工具（T2）：评估 / 沉淀 / AI 味检测 ---

export async function evaluateBook(userId: string, sessionId: string, bookId: string, chapters?: string): Promise<unknown> {
    assertSafeBookId(bookId);
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const core = await import("@actalk/inkos-core");
    const state = new core.StateManager(projectRoot);
    return await core.evaluateBookQuality({ state, bookId, ...(chapters ? { chapters } : {}) });
}

export async function consolidateBook(userId: string, sessionId: string, bookId: string, llm: { baseUrl: string; apiKey: string; model: string; apiFormat: "chat" | "responses"; temperature?: number }): Promise<unknown> {
    assertSafeBookId(bookId);
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const core = await import("@actalk/inkos-core");
    const client = core.createLLMClient({ provider: "custom", service: "custom", configSource: "env", baseUrl: llm.baseUrl, apiKey: llm.apiKey, model: llm.model, temperature: llm.temperature ?? 0.3, thinkingBudget: 0, apiFormat: llm.apiFormat, stream: false });
    const state = new core.StateManager(projectRoot);
    const consolidator = new core.ConsolidatorAgent({ client, model: llm.model, projectRoot });
    return await consolidator.consolidate(state.bookDir(bookId));
}

export async function detectBookChapter(userId: string, sessionId: string, bookId: string, chapterNumber: number, content: string): Promise<{ error?: string; result?: unknown }> {
    assertSafeBookId(bookId);
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const core = await import("@actalk/inkos-core");
    const state = new core.StateManager(projectRoot);
    const projectConfig = await state.loadProjectConfig().catch(() => ({}) as Record<string, unknown>);
    const detection = (projectConfig as { detection?: Record<string, unknown> }).detection;
    if (!detection || detection.enabled !== true || typeof detection.apiUrl !== "string" || !detection.apiUrl) {
        return { error: "未配置 AI 味检测服务：请在项目设置中启用 detection（gptzero/originality/自定义 apiUrl）后重试" };
    }
    if (!core.DetectionConfigSchema) return { error: "核心库缺少检测配置 Schema" };
    const parsed = core.DetectionConfigSchema.safeParse(detection);
    if (!parsed.success) return { error: "检测配置无效" };
    const result = await core.detectChapter(parsed.data, content, chapterNumber);
    return { result };
}
