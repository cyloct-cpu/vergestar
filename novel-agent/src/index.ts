import express from "express";
import path from "node:path";

import { listNovelSkills, resolveNovelSkills } from "./skills.js";
import { runNovelAgentTurn } from "./agent-turn.js";
import { restoreChapterSnapshot, runFullNovelAgentTurn } from "./full-agent.js";
import { approveChapter, consolidateBook, deleteBook, detectBookChapter, evaluateBook, listExportFiles, readExportFile, readTruthFile, rejectChapter, updateBookSettings, writeTruthFile } from "./book-management.js";
import { deleteBookSession, renameBookSession } from "@actalk/inkos-core";
import { listUserNovelAgentJobs } from "./jobs.js";
import { cancelNovelAgentJob, getJobStreamEvents, getNovelAgentJob, startNovelAgentJob, subscribeJobStream } from "./jobs.js";

const host = process.env.VERGESTAR_NOVEL_AGENT_HOST || "127.0.0.1";
const port = Number(process.env.VERGESTAR_NOVEL_AGENT_PORT || "17421");
const internalToken = process.env.VERGESTAR_NOVEL_AGENT_TOKEN || "";
const app = express();

app.use(express.json({ limit: "256kb" }));
app.use((req, res, next) => {
    if (!internalToken || req.header("x-vergestar-novel-agent-token") === internalToken) {
        next();
        return;
    }
    res.status(401).json({ error: "unauthorized" });
});

const sessionWorkspaceRoot = process.env.VERGESTAR_NOVEL_WORKSPACE_ROOT || path.resolve("data");

function sessionProjectRoot(userId: string): string {
    if (!/^[A-Za-z0-9_-]{1,64}$/.test(userId)) throw new Error("用户标识无效");
    return path.join(sessionWorkspaceRoot, userId);
}

app.get("/health", async (_req, res) => {
    const skills = await listNovelSkills();
    res.json({ ok: true, upstream: "inkos-core", skillCount: skills.length });
});

app.get("/skills", async (_req, res, next) => {
    try {
        res.json({ skills: await listNovelSkills() });
    } catch (error) {
        next(error);
    }
});

app.post("/skills/resolve", async (req, res, next) => {
    try {
        const requestedSkills = Array.isArray(req.body?.requestedSkills)
            ? req.body.requestedSkills.filter((value: unknown): value is string => typeof value === "string")
            : [];
        res.json(await resolveNovelSkills(requestedSkills));
    } catch (error) {
        next(error);
    }
});

app.post("/turn", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        if (!body || typeof body.message !== "string" || !body.model || typeof body.model !== "object") {
            res.status(400).json({ error: "message and model are required" });
            return;
        }
        const requestedSkills = Array.isArray(body.requestedSkills)
            ? body.requestedSkills.filter((value): value is string => typeof value === "string")
            : [];
        const history = Array.isArray(body.history)
            ? body.history.filter((value): value is { role: "user" | "assistant"; content: string } => Boolean(value) && typeof value === "object" && (value.role === "user" || value.role === "assistant") && typeof value.content === "string").slice(-30)
            : [];
        const model = body.model as Record<string, unknown>;
        if (typeof model.baseUrl !== "string" || typeof model.apiKey !== "string" || typeof model.model !== "string") {
            res.status(400).json({ error: "model baseUrl, apiKey and model are required" });
            return;
        }
        const provider = model.provider === "anthropic" || model.provider === "openai" || model.provider === "custom" ? model.provider : "custom";
        const apiFormat = model.apiFormat === "responses" ? "responses" : "chat";
        res.json(await runNovelAgentTurn({
            message: body.message,
            history,
            requestedSkills,
            model: {
                provider,
                ...(typeof model.service === "string" ? { service: model.service } : {}),
                baseUrl: model.baseUrl,
                apiKey: model.apiKey,
                model: model.model,
                apiFormat,
                ...(typeof model.temperature === "number" ? { temperature: model.temperature } : {}),
            },
        }));
    } catch (error) {
        next(error);
    }
});

app.post("/agent/turn", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        if (!body || typeof body.message !== "string" || typeof body.userId !== "string" || typeof body.sessionId !== "string" || !body.model || typeof body.model !== "object") {
            res.status(400).json({ error: "userId, sessionId, message and model are required" });
            return;
        }
        const model = body.model as Record<string, unknown>;
        if (typeof model.baseUrl !== "string" || typeof model.apiKey !== "string" || typeof model.model !== "string") {
            res.status(400).json({ error: "model baseUrl, apiKey and model are required" });
            return;
        }
        const requestedSkills = Array.isArray(body.requestedSkills) ? body.requestedSkills.filter((value): value is string => typeof value === "string") : [];
        const confirmedIntent = body.confirmedIntent === "create_book" || body.confirmedIntent === "write_next" || body.confirmedIntent === "repair_state" || body.confirmedIntent === "revise_chapter" || body.confirmedIntent === "create_script" || body.confirmedIntent === "create_storyboard" || body.confirmedIntent === "short_run" || body.confirmedIntent === "draft_next" || body.confirmedIntent === "plan_chapter" || body.confirmedIntent === "compose_chapter" || body.confirmedIntent === "fanfic_init" || body.confirmedIntent === "continuation_import" || body.confirmedIntent === "spinoff_create" || body.confirmedIntent === "style_imitation" || body.confirmedIntent === "translation_create" || body.confirmedIntent === "interactive_film_create" || body.confirmedIntent === "play_start" || body.confirmedIntent === "play_step" ? body.confirmedIntent : undefined;
        const result = await runFullNovelAgentTurn({
            userId: body.userId,
            sessionId: body.sessionId,
            ...(typeof body.bookId === "string" ? { bookId: body.bookId } : {}),
            ...(typeof body.mode === "string" ? { mode: body.mode } : {}),
            ...(body.playMode === "open" || body.playMode === "guided" ? { playMode: body.playMode } : {}),
            message: body.message,
            requestedSkills,
            ...(confirmedIntent ? { confirmedIntent } : {}),
            ...(body.confirmedActionPayload && typeof body.confirmedActionPayload === "object" ? { confirmedActionPayload: body.confirmedActionPayload as Record<string, unknown> } : {}),
            model: {
                provider: model.provider === "anthropic" || model.provider === "openai" || model.provider === "custom" ? model.provider : "custom",
                ...(typeof model.service === "string" ? { service: model.service } : {}),
                baseUrl: model.baseUrl,
                apiKey: model.apiKey,
                model: model.model,
                apiFormat: model.apiFormat === "responses" ? "responses" : "chat",
                ...(typeof model.temperature === "number" ? { temperature: model.temperature } : {}),
            },
        });
        res.json(result);
    } catch (error) {
        next(error);
    }
});

app.post("/agent/jobs", (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        if (!body || typeof body.message !== "string" || typeof body.userId !== "string" || typeof body.sessionId !== "string" || !body.model || typeof body.model !== "object") {
            res.status(400).json({ error: "userId, sessionId, message and model are required" });
            return;
        }
        const model = body.model as Record<string, unknown>;
        if (typeof model.baseUrl !== "string" || typeof model.apiKey !== "string" || typeof model.model !== "string") {
            res.status(400).json({ error: "model baseUrl, apiKey and model are required" });
            return;
        }
        const requestedSkills = Array.isArray(body.requestedSkills) ? body.requestedSkills.filter((value): value is string => typeof value === "string") : [];
        const confirmedIntent = body.confirmedIntent === "create_book" || body.confirmedIntent === "write_next" || body.confirmedIntent === "repair_state" || body.confirmedIntent === "revise_chapter" || body.confirmedIntent === "create_script" || body.confirmedIntent === "create_storyboard" || body.confirmedIntent === "short_run" || body.confirmedIntent === "draft_next" || body.confirmedIntent === "plan_chapter" || body.confirmedIntent === "compose_chapter" || body.confirmedIntent === "fanfic_init" || body.confirmedIntent === "continuation_import" || body.confirmedIntent === "spinoff_create" || body.confirmedIntent === "style_imitation" || body.confirmedIntent === "translation_create" || body.confirmedIntent === "interactive_film_create" || body.confirmedIntent === "play_start" || body.confirmedIntent === "play_step" ? body.confirmedIntent : undefined;
        const job = startNovelAgentJob({
            userId: body.userId,
            sessionId: body.sessionId,
            ...(typeof body.bookId === "string" ? { bookId: body.bookId } : {}),
            ...(typeof body.mode === "string" ? { mode: body.mode } : {}),
            message: body.message,
            requestedSkills,
            ...(confirmedIntent ? { confirmedIntent } : {}),
            ...(body.confirmedActionPayload && typeof body.confirmedActionPayload === "object" ? { confirmedActionPayload: body.confirmedActionPayload as Record<string, unknown> } : {}),
            model: {
                provider: model.provider === "anthropic" || model.provider === "openai" || model.provider === "custom" ? model.provider : "custom",
                ...(typeof model.service === "string" ? { service: model.service } : {}),
                baseUrl: model.baseUrl,
                apiKey: model.apiKey,
                model: model.model,
                apiFormat: model.apiFormat === "responses" ? "responses" : "chat",
                ...(typeof model.temperature === "number" ? { temperature: model.temperature } : {}),
            },
        });
        res.status(202).json({ job });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/chapters/restore-snapshot", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const chapterNumber = typeof body?.chapterNumber === "number" ? body.chapterNumber : Number.NaN;
        if (!body || !bookId || !sessionId || !Number.isInteger(chapterNumber)) {
            res.status(400).json({ error: "bookId, sessionId and chapterNumber are required" });
            return;
        }
        const artifacts = await restoreChapterSnapshot(String(body.userId ?? ""), sessionId, bookId, chapterNumber);
        res.json({ artifacts });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/chapters/approve", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const chapterNumber = typeof body?.chapterNumber === "number" ? body.chapterNumber : Number.NaN;
        if (!bookId || !sessionId || !Number.isInteger(chapterNumber)) {
            res.status(400).json({ error: "bookId, sessionId and chapterNumber are required" });
            return;
        }
        const artifacts = await approveChapter(String(body.userId ?? ""), sessionId, bookId, chapterNumber);
        res.json({ artifacts });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/chapters/reject", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const chapterNumber = typeof body?.chapterNumber === "number" ? body.chapterNumber : Number.NaN;
        if (!bookId || !sessionId || !Number.isInteger(chapterNumber)) {
            res.status(400).json({ error: "bookId, sessionId and chapterNumber are required" });
            return;
        }
        const result = await rejectChapter(String(body.userId ?? ""), sessionId, bookId, chapterNumber);
        res.json(result);
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/settings", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        if (!bookId || !sessionId) {
            res.status(400).json({ error: "bookId and sessionId are required" });
            return;
        }
        const update: Record<string, unknown> = {};
        if (typeof body?.chapterWordCount === "number") update.chapterWordCount = body.chapterWordCount;
        if (typeof body?.targetChapters === "number") update.targetChapters = body.targetChapters;
        if (typeof body?.status === "string") update.status = body.status;
        if (body?.chapterReviewMode === "auto" || body?.chapterReviewMode === "manual") update.chapterReviewMode = body.chapterReviewMode;
        const config = await updateBookSettings(String(body.userId ?? ""), sessionId, bookId, update);
        res.json({ config });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/truth/read", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const truthPath = typeof body?.path === "string" ? body.path : "";
        if (!bookId || !sessionId || !truthPath) {
            res.status(400).json({ error: "bookId, sessionId and path are required" });
            return;
        }
        const result = await readTruthFile(String(body.userId ?? ""), sessionId, bookId, truthPath);
        res.json(result);
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/truth/write", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const truthPath = typeof body?.path === "string" ? body.path : "";
        const content = typeof body?.content === "string" ? body.content : "";
        if (!bookId || !sessionId || !truthPath || !body || typeof body.content !== "string") {
            res.status(400).json({ error: "bookId, sessionId, path and content are required" });
            return;
        }
        const result = await writeTruthFile(String(body.userId ?? ""), sessionId, bookId, truthPath, content);
        res.json(result);
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/exports/list", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        if (!sessionId) {
            res.status(400).json({ error: "sessionId is required" });
            return;
        }
        const files = await listExportFiles(String(body.userId ?? ""), sessionId);
        res.json({ files });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/radar", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const model = body?.model as { baseUrl?: string; apiKey?: string; model?: string; apiFormat?: "chat" | "responses"; temperature?: number } | undefined;
        if (!model || !model.baseUrl || !model.apiKey || !model.model) {
            res.status(400).json({ error: "model is required" });
            return;
        }
        const { createLLMClient, PipelineRunner, createLogger, createStderrSink } = await import("@actalk/inkos-core");
        const projectRoot = sessionProjectRoot(String(body.userId ?? ""));
        const llm = { provider: "custom" as const, service: "custom", configSource: "env" as const, baseUrl: model.baseUrl, apiKey: model.apiKey, model: model.model, temperature: model.temperature ?? 0.4, thinkingBudget: 0, apiFormat: model.apiFormat === "responses" ? "responses" as const : "chat" as const, stream: false };
        const client = createLLMClient(llm);
        const pipeline = new PipelineRunner({ client, model: model.model, projectRoot, defaultLLMConfig: llm, logger: createLogger({ tag: "vergestar-radar", sinks: [createStderrSink({ minLevel: "info", enableColors: false })] }) });
        const result = await pipeline.runRadar();
        res.json({ result });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/eval", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        if (!bookId || !sessionId) {
            res.status(400).json({ error: "bookId and sessionId are required" });
            return;
        }
        const chapters = typeof body?.chapters === "string" ? body.chapters : undefined;
        const result = await evaluateBook(String(body.userId ?? ""), sessionId, bookId, chapters);
        res.json({ eval: result });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/consolidate", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const model = body?.model as { baseUrl?: string; apiKey?: string; model?: string; apiFormat?: "chat" | "responses"; temperature?: number } | undefined;
        if (!bookId || !sessionId || !model || !model.baseUrl || !model.apiKey || !model.model) {
            res.status(400).json({ error: "bookId, sessionId and model are required" });
            return;
        }
        const result = await consolidateBook(String(body.userId ?? ""), sessionId, bookId, { baseUrl: model.baseUrl, apiKey: model.apiKey, model: model.model, apiFormat: model.apiFormat === "responses" ? "responses" : "chat", temperature: model.temperature });
        res.json({ result });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/detect", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const chapterNumber = typeof body?.chapterNumber === "number" ? body.chapterNumber : 0;
        const content = typeof body?.content === "string" ? body.content : "";
        if (!bookId || !sessionId || !chapterNumber || !content) {
            res.status(400).json({ error: "bookId, sessionId, chapterNumber and content are required" });
            return;
        }
        const result = await detectBookChapter(String(body.userId ?? ""), sessionId, bookId, chapterNumber, content);
        res.json(result);
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/exports/read", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        const exportPath = typeof body?.path === "string" ? body.path : "";
        if (!sessionId || !exportPath) {
            res.status(400).json({ error: "sessionId and path are required" });
            return;
        }
        const result = await readExportFile(String(body.userId ?? ""), sessionId, exportPath);
        res.json({ content: result.content.toString("base64"), encoding: "base64" });
    } catch (error) {
        next(error);
    }
});

app.post("/agent/books/delete", async (req, res, next) => {
    try {
        const body = req.body as Record<string, unknown>;
        const bookId = typeof body?.bookId === "string" ? body.bookId : "";
        const sessionId = typeof body?.sessionId === "string" ? body.sessionId : "";
        if (!bookId || !sessionId) {
            res.status(400).json({ error: "bookId and sessionId are required" });
            return;
        }
        const result = await deleteBook(String(body.userId ?? ""), sessionId, bookId);
        res.json(result);
    } catch (error) {
        next(error);
    }
});

app.get("/agent/jobs", (req, res) => {
    const owner = req.header("x-vergestar-novel-agent-user") || String(req.query.userId || "");
    if (!owner) {
        res.status(400).json({ error: "user header is required" });
        return;
    }
    res.json({ jobs: listUserNovelAgentJobs(owner).map((job) => ({ ...job, result: undefined })) });
});

app.put("/agent/sessions/:sessionId", async (req, res, next) => {
    try {
        const owner = req.header("x-vergestar-novel-agent-user") || String(req.body?.userId || "");
        const sessionId = req.params.sessionId;
        const title = typeof req.body?.title === "string" ? req.body.title.trim() : "";
        if (!owner || !sessionId || !title) {
            res.status(400).json({ error: "user, sessionId and title are required" });
            return;
        }
        const renamed = await renameBookSession(sessionProjectRoot(owner), sessionId, title.slice(0, 240));
        if (!renamed) {
            res.status(404).json({ error: "session not found" });
            return;
        }
        res.json({ ok: true });
    } catch (error) {
        next(error);
    }
});

app.delete("/agent/sessions/:sessionId", async (req, res, next) => {
    try {
        const owner = req.header("x-vergestar-novel-agent-user") || String(req.query.userId || "");
        const sessionId = req.params.sessionId;
        if (!owner || !sessionId) {
            res.status(400).json({ error: "user and sessionId are required" });
            return;
        }
        await deleteBookSession(sessionProjectRoot(owner), sessionId);
        res.json({ ok: true });
    } catch (error) {
        next(error);
    }
});
app.post("/agent/jobs/:jobId/cancel", (req, res) => {
    const jobId = req.params.jobId;
    const owner = req.header("x-vergestar-novel-agent-user");
    const cancelled = cancelNovelAgentJob(jobId, owner);
    res.json({ ok: cancelled });
});

app.get("/agent/jobs/:jobId", (req, res) => {
    const job = getNovelAgentJob(req.params.jobId, req.header("x-vergestar-novel-agent-user") || undefined);
    if (!job) {
        res.status(404).json({ error: "job not found" });
        return;
    }
    res.json({ job });
});

// The backend uses this authenticated, server-to-server projection to import
// completed InkOS artifacts. It intentionally exposes no workspace path and
// never returns model credentials.
app.get("/agent/jobs/:jobId/stream", (req, res) => {
    const jobId = req.params.jobId;
    const owner = req.header("x-vergestar-novel-agent-user") || String(req.query.userId || "");
    const job = getNovelAgentJob(jobId, owner || undefined);
    if (!job) {
        res.status(404).json({ error: "job not found" });
        return;
    }
    res.writeHead(200, {
        "content-type": "text/event-stream; charset=utf-8",
        "cache-control": "no-cache",
        connection: "keep-alive",
        "x-accel-buffering": "no",
    });
    const send = (event: string, data: unknown) => {
        res.write(`event: ${event}\ndata: ${JSON.stringify(data ?? {})}\n\n`);
    };
    // 重放缓冲（迟订阅者补齐上下文）
    for (const entry of getJobStreamEvents(jobId)) send(entry.event, entry.data);
    const terminal = job.status === "succeeded" || job.status === "failed";
    if (terminal) {
        send("done", { status: job.status, error: job.error });
        res.end();
        return;
    }
    const unsubscribe = subscribeJobStream(jobId, (entry) => {
        send(entry.event, entry.data);
        if (entry.event === "done") {
            unsubscribe();
            res.end();
        }
    });
    req.on("close", () => {
        unsubscribe();
    });
});

app.get("/agent/jobs/:jobId/artifacts", (req, res) => {
    const job = getNovelAgentJob(req.params.jobId, req.header("x-vergestar-novel-agent-user") || undefined);
    if (!job) {
        res.status(404).json({ error: "job not found" });
        return;
    }
    if (job.status !== "succeeded" || !job.result?.artifacts) {
        res.status(409).json({ error: "job artifacts are not ready" });
        return;
    }
    res.json({ artifacts: job.result.artifacts });
});

app.use((error: unknown, _req: express.Request, res: express.Response, _next: express.NextFunction) => {
    console.error("[bridge] unhandled error:", error instanceof Error ? error.stack : error);
    res.status(500).json({ error: error instanceof Error ? error.message : "novel agent unavailable" });
});

app.listen(port, host, () => {
    process.stdout.write(`Novel Agent listening on http://${host}:${port}\n`);
});


