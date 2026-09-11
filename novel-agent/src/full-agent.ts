import { cp, mkdir, readFile, readdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import {
    abortAgentSession,
    createAndPersistBookSession,
    createInteractionToolsFromDeps,
    createLLMClient,
    createLogger,
    createStderrSink,
    loadBookSession,
    migrateBookSession,
    PipelineRunner,
    runAgentSession,
    StateManager,
    type AgentSessionConfig,
    type LLMConfig,
    type PipelineConfig,
    type ReviseMode,
    type ReviseResult,
} from "@actalk/inkos-core";

import { runScriptCreation, runStoryboardCreation, runShortFictionProduction } from "@actalk/inkos-core";

import type { NovelAgentTurnInput } from "./agent-turn.js";
export type FullNovelAgentTurnInput = NovelAgentTurnInput & {
    userId: string;
    sessionId: string;
    mode?: string;
    bookId?: string;
    confirmedIntent?: "create_book" | "write_next" | "repair_state" | "revise_chapter" | "create_script" | "create_storyboard" | "short_run" | "draft_next" | "plan_chapter" | "compose_chapter" | "fanfic_init" | "continuation_import" | "spinoff_create" | "style_imitation" | "translation_create" | "interactive_film_create" | "play_start" | "play_step" | "interactive_film_create";
    confirmedActionPayload?: Record<string, unknown>;
    abortSignal?: AbortSignal;
    onLog?: (message: string) => void;
    onStreamEvent?: (event: { type: string; text?: string; toolName?: string; isError?: boolean }) => void;
    playMode?: "open" | "guided";
};

export type FullNovelAgentTurnResult = {
    session: {
        id: string;
        title: string;
        mode: string;
        status: "active";
        createdAt: string;
        updatedAt: string;
    };
    text: string;
    events: Array<{ type: string; tool?: string; itemType?: string }>;
    artifacts?: {
        bookId: string;
        agentSessionId: string;
        title: string;
        bookJson: string;
        chapterIndex: string;
        files: Record<string, string>;
        productionFiles?: Record<string, string>;
        productionUnitId?: string;
    };
    confirmation?: {
        action: "create_book" | "write_next" | "repair_state" | "short_run" | "fanfic_init" | "continuation_import" | "spinoff_create" | "style_imitation" | "translation_create";
        title: string;
        summary: string;
        instruction: string;
        requestedSkills: string[];
        actionPayload: Record<string, unknown>;
    };
};

export async function restoreChapterSnapshot(userId: string, sessionId: string, bookId: string, chapterNumber: number) {
    if (!bookId.trim() || bookId.includes("/") || bookId.includes("\\") || bookId.includes("..") || !/^[A-Za-z0-9_-]{1,120}$/.test(sessionId) || !Number.isInteger(chapterNumber) || chapterNumber < 0) {
        throw new Error("章节状态回溯参数无效");
    }
    const projectRoot = safeWorkspacePath(userId, sessionId);
    const state = new StateManager(projectRoot);
    const restored = await state.restoreState(bookId, chapterNumber);
    if (!restored) {
        throw new Error(`InkOS 第 ${chapterNumber} 章状态快照不存在，无法同步版本回溯`);
    }
    return await readBookArtifacts(projectRoot, bookId, sessionId, []);
}

export function abortNovelAgentSession(userId: string, sessionId: string): boolean {
    return abortAgentSession(safeWorkspacePath(userId, sessionId), sessionId);
}

const workspaceRoot = process.env.VERGESTAR_NOVEL_WORKSPACE_ROOT || path.resolve("data");
const vendorRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../vendor/inkos-core");

export async function runFullNovelAgentTurn(input: FullNovelAgentTurnInput): Promise<FullNovelAgentTurnResult> {
    const projectRoot = safeWorkspacePath(input.userId, input.sessionId);
    const llm = asInkosLLMConfig(input);
    await ensureInkosWorkspace(projectRoot, llm);
    const client = createLLMClient(llm);
    // 书籍设置里的审查模式（book.json writing.reviewMode）优先于默认自动审查。
    let bookChapterReviewMode: "auto" | "manual" = "auto";
    if (input.bookId) {
        try {
            const bookConfig = await new StateManager(projectRoot).loadBookConfig(input.bookId);
            const writing = bookConfig.writing && typeof bookConfig.writing === "object" ? bookConfig.writing as Record<string, unknown> : undefined;
            if (writing && (writing.reviewMode === "manual" || writing.reviewMode === "auto")) bookChapterReviewMode = writing.reviewMode;
        } catch {
            // 书籍尚未创建或读取失败时按默认自动审查。
        }
    }
    const pipelineConfig: PipelineConfig = {
        client,
        model: llm.model,
        projectRoot,
        defaultLLMConfig: llm,
        foundationReviewRetries: 2,
        writingReviewRetries: 1,
        chapterReviewMode: bookChapterReviewMode,
        logger: createLogger({ tag: "vergestar-novel", sinks: [createStderrSink({ minLevel: "info", enableColors: false }), { write: (entry) => input.onLog?.(entry.message) }] }),
    };
    const pipeline = new PipelineRunner(pipelineConfig);
    const sessionKind = input.confirmedIntent === "create_book" ? "book-create" : input.confirmedIntent === "short_run" ? "short" : input.confirmedIntent === "interactive_film_create" ? "interactive-film" : (input.confirmedIntent === "play_start" || input.confirmedIntent === "play_step") ? "play" : input.bookId ? "book" : "chat";
    if (input.bookId) {
        const existingSession = await loadBookSession(projectRoot, input.sessionId);
        if (existingSession?.bookId === null) {
            await migrateBookSession(projectRoot, input.sessionId, input.bookId);
        }
    }
    await createAndPersistBookSession(projectRoot, input.bookId ?? null, input.sessionId, sessionKind);

    if (input.confirmedIntent === "short_run") {
        // 短篇是独立生产物，不绑定 InkOS 书籍；复用核心原生 runShortFictionProduction，
        // 产物落在 shorts/<storyId>/。投影给 Vergestar 时合成最小书籍元数据，
        // 让短篇像小说项目一样出现在书架与真相面板，但不接入 write_next 等章节操作。
        const payload = (input.confirmedActionPayload?.shortRun ?? {}) as Record<string, unknown>;
        const payloadDirection = typeof payload.direction === "string" ? payload.direction.trim() : "";
        const fallbackDirection = typeof input.confirmedActionPayload?.instruction === "string" ? input.confirmedActionPayload.instruction.trim() : "";
        const direction = payloadDirection || fallbackDirection;
        if (!direction) throw new Error("短篇小说生产缺少创作方向");
        const title = typeof payload.title === "string" && payload.title.trim() ? payload.title.trim() : "";
        const language = payload.language === "en" ? "en" as const : "zh" as const;
        const chapterCount = typeof payload.chapters === "number" && Number.isInteger(payload.chapters) ? payload.chapters : undefined;
        const charsPerChapter = typeof payload.charsPerChapter === "number" && Number.isInteger(payload.charsPerChapter) ? payload.charsPerChapter : undefined;
        input.onLog?.("阶段：加载 InkOS 短篇生产链路");
        const result = await runShortFictionProduction({
            projectRoot,
            title: title || undefined,
            direction,
            runtimes: {
                planner: pipeline.createAgentContext("short-outline"),
                outlineReview: pipeline.createAgentContext("short-outline-review"),
                writer: pipeline.createAgentContext("short-writer"),
                draftReview: pipeline.createAgentContext("short-draft-review"),
                revise: pipeline.createAgentContext("short-revise"),
                package: pipeline.createAgentContext("short-package"),
            },
            storyId: typeof payload.storyId === "string" && payload.storyId.trim() ? payload.storyId.trim() : undefined,
            chapterCount,
            charsPerChapter,
            language,
            signal: input.abortSignal,
            onProgress: (message) => input.onLog?.(message),
        });
        const productionFiles: Record<string, string> = {};
        for (const artifactPath of [result.outlinePath, result.outlineReviewPath, result.draftReviewPath, result.finalMarkdownPath, result.finalJsonPath, result.salesPackagePath, result.coverPromptPath]) {
            if (!artifactPath) continue;
            try {
                productionFiles[artifactPath] = await readFile(path.join(projectRoot, artifactPath), "utf8");
            } catch {
                // 单个产物缺失不阻断整部短篇的投影。
            }
        }
        const finalMarkdown = productionFiles[result.finalMarkdownPath] ?? "";
        const wordCount = finalMarkdown.replace(/\s/g, "").length;
        const shortBookId = `short-${result.storyId}`.replace(/[^A-Za-z0-9_-]/g, "-").slice(0, 100);
        const shortTitle = title || result.storyId;
        const syntheticBookJson = JSON.stringify({ title: shortTitle, kind: "short", language, storyId: result.storyId, targetChapters: chapterCount ?? null, chapterWordCount: charsPerChapter ?? null, finalMarkdownPath: result.finalMarkdownPath });
        const now = new Date().toISOString();
        return {
            session: { id: input.sessionId, title: shortTitle, mode: sessionKind, status: "active", createdAt: now, updatedAt: now },
            text: `短篇小说《${shortTitle}》已完成：全文约 ${wordCount} 字（${result.outlinePath ? "大纲、审核记录、" : ""}正文、销售包已就绪${result.coverError ? "；封面生成失败：" + result.coverError : ""}）。`,
            events: [{ type: "short_fiction_created" }],
            artifacts: {
                bookId: shortBookId,
                agentSessionId: input.sessionId,
                title: shortTitle,
                bookJson: syntheticBookJson,
                chapterIndex: JSON.stringify({ chapters: [] }),
                files: {},
                productionFiles,
            },
        };
    }
    if (input.confirmedIntent === "draft_next" || input.confirmedIntent === "plan_chapter" || input.confirmedIntent === "compose_chapter") {
        // InkOS BookDetail 的三个手动操作：仅草稿（写完即停，不审计）、规划、成文。
        // 与 write_next 的差别是跳过确认规划——按钮即意图，直接走原生 Pipeline。
        if (!input.bookId) throw new Error("该操作缺少绑定的 InkOS 书籍");
        const payload = input.confirmedActionPayload ?? {};
        const context = typeof payload.context === "string" && payload.context.trim() ? payload.context.trim() : undefined;
        const wordCount = typeof payload.wordCount === "number" && payload.wordCount > 0 ? payload.wordCount : undefined;
        input.onLog?.("阶段：加载 InkOS 生产链路");
        let resultText = "";
        if (input.confirmedIntent === "draft_next") {
            const draft = await pipeline.writeDraft(input.bookId, context, wordCount);
            resultText = "已仅草稿写完第 " + draft.chapterNumber + " 章《" + draft.title + "》，" + draft.wordCount + " 字；未审计、未修订，可在章节面板继续审计或修订。";
        } else if (input.confirmedIntent === "plan_chapter") {
            const plan = await pipeline.planChapter(input.bookId, context);
            resultText = "已完成第 " + plan.chapterNumber + " 章规划：" + plan.goal;
        } else {
            const composed = await pipeline.composeChapter(input.bookId, context);
            resultText = "已按规划成文第 " + composed.chapterNumber + " 章：" + composed.goal;
        }
        const artifacts = await readBookArtifacts(projectRoot, input.bookId, input.sessionId, []);
        const now = new Date().toISOString();
        return {
            session: { id: input.sessionId, title: input.message.trim().slice(0, 40) || input.confirmedIntent, mode: sessionKind, status: "active", createdAt: now, updatedAt: now },
            text: resultText,
            events: [{ type: input.confirmedIntent + "_complete" }],
            artifacts,
        };
    }
    if (input.confirmedIntent === "create_script" || input.confirmedIntent === "create_storyboard") {
        if (!input.bookId) throw new Error("影视生产缺少绑定的 InkOS 书籍");
        const isScript = input.confirmedIntent === "create_script";
        const payload = (input.confirmedActionPayload?.[isScript ? "scriptCreate" : "storyboardCreate"] ?? {}) as Record<string, unknown>;
        const payloadTitle = typeof payload.title === "string" ? payload.title.trim() : "";
        const fallbackTitle = typeof input.confirmedActionPayload?.title === "string" ? input.confirmedActionPayload.title.trim() : "";
        const title = payloadTitle || fallbackTitle;
        if (!title) throw new Error("影视生产缺少项目标题");
        const chapterNumber = typeof payload.chapterNumber === "number" && Number.isInteger(payload.chapterNumber) && payload.chapterNumber > 0 ? payload.chapterNumber : 1;
        const chapterFile = await latestChapterFile(projectRoot, input.bookId, chapterNumber);
        const source = await readFile(chapterFile.absolutePath, "utf8");
        const runtime = pipeline.createAgentContext(isScript ? "script-creation" : "storyboard-creation", input.bookId);
        input.onLog?.(isScript ? "阶段：加载 InkOS 剧本生产链路" : "阶段：加载 InkOS 分镜生产链路");
        if (isScript) {
            const result = await runScriptCreation({
                projectRoot, runtime, title, language: "zh" as const,
                instruction: typeof payload.instruction === "string" && payload.instruction.trim() ? payload.instruction.trim() : `根据第 ${chapterNumber} 章改编为竖屏短剧剧本。保留主线冲突、人物边界和关键伏笔。`,
                sourceKind: "novel chapter", targetFormat: "vertical_short_drama", sourceText: source,
                episodeCount: typeof payload.episodeCount === "number" ? payload.episodeCount : 1,
                episodeDuration: typeof payload.episodeDuration === "string" ? payload.episodeDuration : "1-2 minutes",
                projectId: `${slugForProduction(`book-${input.bookId}-ch-${String(chapterNumber).padStart(4, "0")}`)}`,
                outDir: "dramas",
                onProgress: (message) => input.onLog?.(message),
            });
            const [script, spec] = await Promise.all([
                readFile(path.join(projectRoot, result.scriptPath), "utf8"),
                readFile(path.join(projectRoot, result.specPath), "utf8"),
            ]);
            const bookArtifacts = await readBookArtifacts(projectRoot, input.bookId, input.sessionId, []);
            if (!bookArtifacts) throw new Error("InkOS 书籍元数据读取失败");
            const now = new Date().toISOString();
            return {
                session: { id: input.sessionId, title, mode: sessionKind, status: "active", createdAt: now, updatedAt: now },
                text: `已完成第 ${chapterNumber} 章剧本改编：${result.projectId}。`,
                events: [{ type: "script_created" }],
                artifacts: { ...bookArtifacts, productionFiles: { [result.scriptPath]: script, [result.specPath]: spec } },
            };
        }
        const result = await runStoryboardCreation({
            projectRoot, runtime, title, language: "zh" as const,
            instruction: typeof payload.instruction === "string" && payload.instruction.trim() ? payload.instruction.trim() : `根据第 ${chapterNumber} 章生成竖屏短剧分镜，逐镜头包含画面、动作、台词、镜头与图像提示词。`,
            sourceKind: "novel chapter", sourceText: source,
            visualStyle: typeof payload.visualStyle === "string" ? payload.visualStyle : "modern cinematic short drama, natural light, clean composition",
            aspectRatio: typeof payload.aspectRatio === "string" ? payload.aspectRatio : "9:16",
            granularity: typeof payload.granularity === "string" ? payload.granularity : "scene and key shots",
            maxShots: typeof payload.maxShots === "number" ? payload.maxShots : 12,
            projectId: `${slugForProduction(`book-${input.bookId}-ch-${String(chapterNumber).padStart(4, "0")}`)}`,
            outDir: "storyboards",
            onProgress: (message) => input.onLog?.(message),
        });
        const [storyboard, imagePrompts, assetsRaw] = await Promise.all([
            readFile(path.join(projectRoot, result.storyboardPath), "utf8"),
            readFile(path.join(projectRoot, result.imagePromptsPath), "utf8"),
            readFile(path.join(projectRoot, result.assetsManifestPath), "utf8"),
        ]);
        const bookArtifacts = await readBookArtifacts(projectRoot, input.bookId, input.sessionId, []);
            if (!bookArtifacts) throw new Error("InkOS 书籍元数据读取失败");
        const storyboardAssets = JSON.parse(assetsRaw) as { assets?: unknown[] };
        const now = new Date().toISOString();
        return {
            session: { id: input.sessionId, title, mode: sessionKind, status: "active", createdAt: now, updatedAt: now },
            text: `已完成第 ${chapterNumber} 章分镜生成：${result.projectId}，共 ${storyboardAssets.assets?.length ?? 0} 个镜头。`,
            events: [{ type: "storyboard_created" }],
            artifacts: { ...bookArtifacts, productionFiles: { [result.storyboardPath]: storyboard, [result.imagePromptsPath]: imagePrompts, [result.assetsManifestPath]: assetsRaw }, productionUnitId: typeof input.confirmedActionPayload?.unitId === "string" ? input.confirmedActionPayload.unitId : undefined },
        };
    }
    if (input.confirmedIntent === "revise_chapter") {
        if (!input.bookId) throw new Error("章节修订缺少绑定的 InkOS 书籍");
        const payload = (input.confirmedActionPayload?.reviseChapter ?? {}) as {
            chapterNumber?: unknown;
            mode?: unknown;
            instruction?: unknown;
        };
        const chapterNumber = typeof payload.chapterNumber === "number" && Number.isInteger(payload.chapterNumber) && payload.chapterNumber > 0
            ? payload.chapterNumber
            : undefined;
        const mode = typeof payload.mode === "string" && payload.mode ? (payload.mode as ReviseMode) : "spot-fix";
        const instruction = typeof payload.instruction === "string" && payload.instruction.trim()
            ? payload.instruction.trim()
            : "请根据最新审稿问题修订目标章节，保留主线事实和人物边界，优先修复警告与轻微矛盾。";
        input.onLog?.("阶段：加载 InkOS Reviser 生产链路");
        const revision: ReviseResult = await pipeline.runWithAgentContext(
            { activatedSkills: [], signal: input.abortSignal },
            () => pipeline.reviseDraft(input.bookId!, chapterNumber, mode, instruction),
        );
        const artifacts = await readBookArtifacts(projectRoot, input.bookId, input.sessionId, []);
        const now = new Date().toISOString();
        return {
            session: { id: input.sessionId, title: input.message.trim().slice(0, 40), mode: sessionKind, status: "active", createdAt: now, updatedAt: now },
            text: revision.applied
                ? `已按审稿意见修订第 ${revision.chapterNumber} 章，状态 ${revision.status}，修复 ${revision.fixedIssues.length} 项问题。`
                : `修订门禁保留第 ${revision.chapterNumber} 章原文：${revision.skippedReason ?? revision.status}`,
            events: [{ type: "revision_complete" }],
            artifacts,
        };
    }

    const events: Array<{ type: string; tool?: string; itemType?: string }> = [];
    const config: AgentSessionConfig = {
        sessionId: input.sessionId,
        bookId: input.bookId ?? null,
        sessionKind,
        actionSource: input.confirmedIntent ? "button" : "free-text",
        ...(input.playMode ? { playMode: input.playMode } : {}),
        ...(input.confirmedIntent ? { requestedIntent: input.confirmedIntent } : {}),
        ...(input.confirmedActionPayload ? { actionPayload: input.confirmedActionPayload as never } : {}),
        requestedSkills: input.requestedSkills,
        language: "zh",
        pipeline,
        projectRoot,
        model: client._piModel as NonNullable<typeof client._piModel>,
        apiKey: input.model.apiKey,
        ...(input.bookId && !input.confirmedIntent
            ? {
                backgroundTaskContext: input.mode === "repair-state"
                    ? "这是 Vergestar 章节状态修复的规划回合。当前书籍已经存在，用户要求保留正文，只修复状态、摘要和伏笔并重新审稿。必须读取当前章节状态后调用 propose_action，action=repair_state；确认卡的 actionPayload 可包含 repairState.chapterNumber。本回合绝对不要调用 sub_agent、writer、auditor 或 reviser，也不要只返回普通文字。"
                    : input.mode === "book-audit"
                        ? "这是 Vergestar 的书籍审计回合。当前书籍已经存在，用户要求审计。直接调用 sub_agent(agent=\"auditor\") 对最新章节执行审计（用户指明了章节号时传入 chapterNumber），完成后用中文总结审计结论、评分和问题清单。不要调用 writer/reviser/exporter，不要调用 propose_action，不要修改正文。"
                        : input.mode === "book-export"
                            ? "这是 Vergestar 的全书导出回合。当前书籍已经存在，用户要求导出。直接调用 sub_agent(agent=\"exporter\") 以默认 txt 格式导出当前书籍，完成后用中文报告导出结果和文件的相对路径。不要调用 propose_action，不要修改正文。"
                            : input.mode === "book-radar"
                                ? "这是 Vergestar 的市场趋势研究回合。围绕当前书籍的题材和定位，使用可用的研究工具做一次市场趋势扫描，并以中文总结要点、机会与竞品参考。不要调用 propose_action，不要修改任何书籍文件。"
                                : "这是 Vergestar Writer 的规划回合。当前书籍已经存在，用户请求的是规划写下一章，不是建书、结构草案或普通讨论。必须读取当前书籍状态后，调用 propose_action，action=write_next；确认卡的 actionPayload 必须包含 writeNext.chapterCount（默认 1）。书籍文件位于 books/<书籍ID>/ 目录之下：读状态用 books/<书籍ID>/story/current_state.md 与 books/<书籍ID>/story/chapter_summaries.md（注意路径带 books/<书籍ID>/ 前缀，不要直接读 story/ 开头的路径）。本回合绝对不要调用 draft_structure、create_book、sub_agent、writer、auditor 或 reviser，也不要只返回普通文字。",
            }
            : {}),
        // Planning turns for an existing book must not be able to invoke
        // Writer/Auditor/Reviser directly. A confirmed action turn is the
        // only path that re-enables InkOS production mutation tools. Book
        // command turns (audit/export) are explicit user instructions and run
        // the corresponding sub-agent directly, matching InkOS book sessions.
                ...(input.confirmedIntent === "write_next" && input.bookId
            ? {
                backgroundTaskContext:
                    '这是 Vergestar 的写作执行回合。调用 sub_agent(agent=writer) 时必须省略 bookId 参数（会话已绑定当前书，传错会导致校验失败）。按确认卡方案完成第 N 章写作生产。',
            }
            : {}),
        ...(input.mode === "book-cover" && input.bookId
            ? {
                backgroundTaskContext:
                    "这是 Vergestar 的封面生成回合。当前书籍已经存在，用户要求生成封面。直接调用 generate_cover 工具，actionPayload 带书名与卖点（可先用 read 工具读取 brief.md 或 book.json 提取卖点）。完成后用中文报告封面生成结果与文件路径。不要调用 propose_action，不要修改正文。",
            }
            : {}),
        suppressProductionTools: !input.confirmedIntent && !["book-audit", "book-export", "book-cover"].includes(input.mode ?? ""),
        onEvent: (event) => {
            const value = event as { type?: unknown; toolName?: unknown; item?: { type?: unknown }; assistantMessageEvent?: { type?: unknown; delta?: unknown }; isError?: unknown };
            events.push({
                type: typeof value.type === "string" ? value.type : "unknown",
                ...(typeof value.toolName === "string" ? { tool: value.toolName } : {}),
                ...(typeof value.item?.type === "string" ? { itemType: value.item.type } : {}),
            });
            // 流式输出（T3）：把 token 增量与工具执行事件转发给宿主。
            if (!input.onStreamEvent) return;
            const eventType = typeof value.type === "string" ? value.type : "";
            if (eventType === "message_update") {
                const ame = value.assistantMessageEvent as { type?: string; delta?: string } | undefined;
                if (ame?.type === "text_delta" && typeof ame.delta === "string" && ame.delta) {
                    input.onStreamEvent({ type: "delta", text: ame.delta });
                }
                return;
            }
            if (eventType === "tool_execution_start" && typeof value.toolName === "string") {
                input.onStreamEvent({ type: "tool", toolName: value.toolName });
                return;
            }
            if (eventType === "tool_execution_end" && typeof value.toolName === "string") {
                input.onStreamEvent({ type: "tool_end", toolName: value.toolName, isError: value.isError === true });
                return;
            }
        },
    };
    const result = await runAgentSession(config, input.message);
    if (result.errorMessage) {
        // Do not turn an upstream/provider failure into a fake successful
        // proposal. The UI needs a retryable error and the job needs to be
        // marked failed, otherwise users are left with a dead confirmation
        // card or an indefinite spinner.
        throw new Error(`InkOS Agent 执行失败：${result.errorMessage}`);
    }
    // A failed tool result inside a pi-agent turn is not surfaced as
    // errorMessage: the final assistant message can be empty while the
    // transcript only records isError on the toolResult. In confirmed
    // production turns this must fail loudly, otherwise jobs.ts marks the
    // mutation as succeeded and the UI hides the real error.
    if (input.confirmedIntent) {
        const toolError = firstToolResultError(result.messages);
        if (toolError) {
            throw new Error(`InkOS 生产工具执行失败：${toolError}`);
        }
    }
    const confirmation = input.confirmedIntent ? undefined : await readProposedAction(projectRoot, input.sessionId);
    const productionResultText = latestToolResultText(result.messages);
    const artifacts = input.confirmedIntent ? await readBookArtifacts(projectRoot, input.bookId, input.sessionId, result.messages) : undefined;
    const now = new Date().toISOString();
    return {
        session: {
            id: input.sessionId,
            title: titleFromAction(input.confirmedActionPayload) || input.message.trim().slice(0, 40),
            mode: sessionKind,
            status: "active",
            createdAt: now,
            updatedAt: now,
        },
        text: result.responseText || productionResultText || confirmation?.summary || "已生成待确认的创作动作。",
        events,
        ...(artifacts ? { artifacts } : {}),
        ...(confirmation ? { confirmation } : {}),
    };
}

const exportedFoundationFiles = [
    "story/author_intent.md",
    "story/brief.md",
    "story/book_rules.md",
    "story/character_matrix.md",
    "story/current_state.md",
    "story/pending_hooks.md",
    "story/emotional_arcs.md",
    "story/chapter_summaries.md",
    "story/subplot_board.md",
    "story/story_bible.md",
    "story/style_guide.md",
    "story/outline/story_frame.md",
    "story/outline/volume_map.md",
] as const;

export async function readBookArtifacts(projectRoot: string, requestedBookId: string | undefined, agentSessionId: string, messages: unknown): Promise<FullNovelAgentTurnResult["artifacts"]> {
    const bookId = requestedBookId?.trim() || createdBookID(messages);
    if (!bookId) return undefined;
    const bookDir = path.join(projectRoot, "books", bookId);
    const bookJson = await readFile(path.join(bookDir, "book.json"), "utf8");
    const files: Record<string, string> = {};
    for (const relativePath of exportedFoundationFiles) {
        files[relativePath] = await readFile(path.join(bookDir, relativePath), "utf8").catch(() => "");
    }
    const roleRoot = path.join(bookDir, "story", "roles");
    for (const category of await readdir(roleRoot, { withFileTypes: true }).catch(() => [])) {
        if (!category.isDirectory()) continue;
        for (const entry of await readdir(path.join(roleRoot, category.name), { withFileTypes: true }).catch(() => [])) {
            if (!entry.isFile() || !entry.name.endsWith(".md")) continue;
            const relativePath = path.posix.join("story/roles", category.name, entry.name);
            files[relativePath] = await readFile(path.join(roleRoot, category.name, entry.name), "utf8");
            if (Object.keys(files).length >= 64) break;
        }
        if (Object.keys(files).length >= 64) break;
    }
    const chaptersRoot = path.join(bookDir, "chapters");
    files["chapters/index.json"] = await readFile(path.join(chaptersRoot, "index.json"), "utf8").catch(() => "[]");
    for (const entry of await readdir(chaptersRoot, { withFileTypes: true }).catch(() => [])) {
        if (!entry.isFile() || !entry.name.endsWith(".md") || entry.name.startsWith(".")) continue;
        const relativePath = path.posix.join("chapters", entry.name);
        files[relativePath] = await readFile(path.join(chaptersRoot, entry.name), "utf8");
        if (Object.keys(files).length >= 96) break;
    }
    const parsed = JSON.parse(bookJson) as { title?: unknown };
    return {
        bookId,
        agentSessionId,
        title: typeof parsed.title === "string" && parsed.title.trim() ? parsed.title.trim() : bookId,
        bookJson,
        chapterIndex: files["chapters/index.json"] || "[]",
        files,
    };
}

function createdBookID(messages: unknown): string | undefined {
    if (!Array.isArray(messages)) return undefined;
    for (const item of [...messages].reverse()) {
        if (!item || typeof item !== "object" || (item as { role?: unknown }).role !== "toolResult") continue;
        const details = (item as { details?: unknown }).details;
        if (!details || typeof details !== "object") continue;
        const bookId = (details as { bookId?: unknown }).bookId;
        if ((details as { kind?: unknown }).kind === "book_created" && typeof bookId === "string" && bookId.trim()) return bookId.trim();
    }
    return undefined;
}

function titleFromAction(actionPayload?: Record<string, unknown>): string | undefined {
    const createBook = actionPayload?.createBook;
    if (!createBook || typeof createBook !== "object") return undefined;
    const title = (createBook as Record<string, unknown>).title;
    return typeof title === "string" && title.trim() ? title.trim() : undefined;
}

function firstToolResultError(messages: unknown): string | undefined {
    if (!Array.isArray(messages)) return undefined;
    for (const item of [...messages].reverse()) {
        if (!item || typeof item !== "object") continue;
        const message = (item as { message?: unknown }).message;
        const candidates = [item, ...(message && typeof message === "object" ? [message] : [])];
        for (const candidate of candidates) {
            const value = candidate as { role?: unknown; isError?: unknown; content?: unknown };
            if (value.role !== "toolResult" || value.isError !== true) continue;
            if (Array.isArray(value.content)) {
                const text = value.content
                    .filter((block): block is { type: "text"; text: string } => Boolean(block) && typeof block === "object"
                        && (block as { type?: unknown }).type === "text" && typeof (block as { text?: unknown }).text === "string")
                    .map((block) => block.text.trim())
                    .filter(Boolean)
                    .join("\n");
                if (text) return text;
            }
            return "工具返回错误，但未提供错误详情。";
        }
    }
    return undefined;
}


function latestToolResultText(messages: unknown): string | undefined {
    if (!Array.isArray(messages)) return undefined;
    for (const item of [...messages].reverse()) {
        if (!item || typeof item !== "object" || (item as { role?: unknown }).role !== "toolResult") continue;
        const content = (item as { content?: unknown }).content;
        if (!Array.isArray(content)) continue;
        const text = content
            .filter((block): block is { type: "text"; text: string } => Boolean(block) && typeof block === "object" && (block as { type?: unknown }).type === "text" && typeof (block as { text?: unknown }).text === "string")
            .map((block) => block.text.trim())
            .filter(Boolean)
            .join("\n");
        if (text) return text;
    }
    return undefined;
}

export async function readProposedAction(projectRoot: string, sessionId: string): Promise<FullNovelAgentTurnResult["confirmation"]> {
    const transcriptPath = path.join(projectRoot, ".inkos", "sessions", `${sessionId}.jsonl`);
    const raw = await readFile(transcriptPath, "utf8");
    const parsedLines = raw.split(/\r?\n/).filter(Boolean).flatMap((line) => {
        try {
            return [{ raw: line, event: JSON.parse(line) as { seq?: unknown; type?: unknown; role?: unknown; message?: { role?: unknown; details?: unknown } } }];
        } catch {
            return [];
        }
    });
    // A book session can contain several old proposals. Only inspect the
    // current request's transcript tail; otherwise a stale create_book or
    // draft_structure card can be returned as the Writer plan.
    const latestRequestStarted = [...parsedLines].reverse().find(({ event }) => event.type === "request_started");
    const latestRequestSeq = typeof latestRequestStarted?.event.seq === "number" ? latestRequestStarted.event.seq : undefined;
    const lines = parsedLines
        .filter(({ event }) => latestRequestSeq === undefined || (typeof event.seq === "number" && event.seq > latestRequestSeq))
        .reverse();
    for (const line of lines) {
        const event = line.event;
        // InkOS transcript events store the transcript envelope role at the
        // top level, while some versions also only expose the message role.
        // Accept both shapes so proposals remain renderable across upstream
        // Core upgrades.
        const role = event.role ?? event.message?.role;
        if (event.type !== "message" || role !== "toolResult") continue;
        const details = event.message?.details;
        if (!details || typeof details !== "object") continue;
        const value = details as Record<string, unknown>;
        const EXTRACTABLE_ACTIONS = new Set(["create_book", "write_next", "repair_state", "short_run", "fanfic_init", "continuation_import", "spinoff_create", "style_imitation", "translation_create", "interactive_film_create", "play_start", "play_step"]);
if (value.kind !== "proposed_action" || typeof value.action !== "string" || !EXTRACTABLE_ACTIONS.has(value.action)) continue;
        const action = value.action as "create_book" | "write_next" | "repair_state" | "short_run" | "fanfic_init" | "continuation_import" | "spinoff_create" | "style_imitation" | "translation_create";
        const title = typeof value.title === "string" ? value.title : action === "write_next" ? "写下一章" : action === "repair_state" ? "修复章节状态" : action === "short_run" ? "创作短篇小说" : "创建长篇小说";
        const summary = typeof value.summary === "string" ? value.summary : action === "write_next" ? "确认后开始写作下一章。" : action === "repair_state" ? "确认后保留正文，重建章节状态并重新审稿。" : action === "short_run" ? "确认后开始创作完整短篇。" : "确认后开始创建小说。";
        const instruction = typeof value.instruction === "string" ? value.instruction : "";
        const actionPayload = value.actionPayload && typeof value.actionPayload === "object" ? value.actionPayload as Record<string, unknown> : {};
        const requestedSkills = Array.isArray(value.requestedSkills) ? value.requestedSkills.filter((item): item is string => typeof item === "string") : [];
        return { action, title, summary, instruction, requestedSkills, actionPayload };
    }
    return undefined;
}

export async function ensureInkosWorkspace(projectRoot: string, llm: LLMConfig): Promise<void> {
    await mkdir(projectRoot, { recursive: true });
    // InkOS pipeline reads genre profiles from the project root. Copy once into
    // a per-session directory so no user can affect another user's canon.
    await cp(path.join(vendorRoot, "genres"), path.join(projectRoot, "genres"), {
        recursive: true,
        force: false,
        errorOnExist: false,
    });
    const diskConfig = {
        name: "Vergestar Novel Agent",
        version: "0.1.0",
        language: "zh",
        llm: { ...llm, apiKey: "" },
        notify: [],
    };
    // Runtime credentials remain only in the in-memory PipelineConfig/client.
    await writeFile(path.join(projectRoot, "inkos.json"), JSON.stringify(diskConfig, null, 2), "utf8");
}

function asInkosLLMConfig(input: NovelAgentTurnInput): LLMConfig {
    return {
        provider: input.model.provider === "anthropic" ? "anthropic" : "custom",
        service: input.model.service || "custom",
        configSource: "env",
        baseUrl: input.model.baseUrl,
        apiKey: input.model.apiKey,
        model: input.model.model,
        temperature: input.model.temperature ?? 0.7,
        thinkingBudget: 0,
        apiFormat: input.model.apiFormat || "chat",
        stream: true,
    };
}

function slugForProduction(value: string): string {
    const text = value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
    return text || `production-${Date.now()}`;
}

async function latestChapterFile(projectRoot: string, bookId: string, chapterNumber: number): Promise<{ absolutePath: string; fileName: string }> {
    if (!bookId.trim() || bookId.includes("/") || bookId.includes("\\") || bookId.includes("..") || chapterNumber < 1 || chapterNumber > 10_000) {
        throw new Error("影视生产章节参数无效");
    }
    const chaptersRoot = path.join(projectRoot, "books", bookId, "chapters");
    const prefix = `${String(chapterNumber).padStart(4, "0")}_`;
    const entries = await readdir(chaptersRoot, { withFileTypes: true }).catch(() => []);
    const entry = entries.find((item) => item.isFile() && item.name.startsWith(prefix) && item.name.endsWith(".md"))
        ?? entries.find((item) => item.isFile() && /^\d+_.*\.md$/.test(item.name) && Number.parseInt(item.name.split("_", 1)[0] ?? "", 10) === chapterNumber);
    if (!entry) throw new Error(`InkOS 第 ${chapterNumber} 章正文不存在，无法进行影视生产`);
    return { absolutePath: path.join(chaptersRoot, entry.name), fileName: entry.name };
}

function safeWorkspacePath(userId: string, sessionId: string): string {
    const safe = (value: string, label: string) => {
        const normalized = value.trim();
        if (!/^[A-Za-z0-9_-]{1,80}$/.test(normalized)) throw new Error(`invalid ${label}`);
        return normalized;
    };
    return path.join(workspaceRoot, safe(userId, "user id"), safe(sessionId, "session id"));
}







