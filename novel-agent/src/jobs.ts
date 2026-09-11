import { randomUUID } from "node:crypto";
import { EventEmitter } from "node:events";
import { mkdirSync, readFileSync, readdirSync, renameSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";

import { abortNovelAgentSession, runFullNovelAgentTurn, type FullNovelAgentTurnInput, type FullNovelAgentTurnResult } from "./full-agent.js";

export type NovelAgentJobStatus = "queued" | "running" | "succeeded" | "failed";

export type NovelAgentJob = {
    id: string;
    status: NovelAgentJobStatus;
    stage: string;
    logs: string[];
    result?: FullNovelAgentTurnResult;
    error?: string;
    createdAt: string;
    updatedAt: string;
};

const jobs = new Map<string, NovelAgentJob>();

// 流式事件总线（T3 SSE）：每个 Job 保留有界事件缓冲（重放）+ 实时 emitter。
type JobStreamEvent = { event: string; data: unknown };
const jobStreams = new Map<string, { emitter: EventEmitter; buffer: JobStreamEvent[] }>();

function pushJobStreamEvent(jobId: string, event: string, data: unknown): void {
    const stream = jobStreams.get(jobId);
    if (!stream) return;
    stream.buffer.push({ event, data });
    if (stream.buffer.length > 4000) stream.buffer.splice(0, 1000);
    stream.emitter.emit("event", { event, data });
}

export function getJobStreamEvents(jobId: string): JobStreamEvent[] {
    return jobStreams.get(jobId)?.buffer ?? [];
}

export function subscribeJobStream(jobId: string, listener: (event: JobStreamEvent) => void): () => void {
    const stream = jobStreams.get(jobId);
    if (!stream) return () => undefined;
    stream.emitter.on("event", listener);
    return () => stream.emitter.off("event", listener);
}
// 单写入锁：写书类生产任务按 bookId（无 bookId 时按会话）互斥，避免双写。
const productionLocks = new Map<string, boolean>();

const BOOK_WRITING_INTENTS = new Set(["write_next", "draft_next", "compose_chapter", "revise_chapter", "repair_state", "create_script", "create_storyboard", "create_book", "short_run", "fanfic_init", "continuation_import", "spinoff_create", "style_imitation", "translation_create", "play_start", "play_step"]);

function productionLockKey(input: FullNovelAgentTurnInput): string {
    return input.bookId ? `book:${input.bookId}` : `session:${input.sessionId}`;
}
const jobOwners = new Map<string, string>();
const jobControllers = new Map<string, AbortController>();
const jobAbortTargets = new Map<string, { userId: string; sessionId: string }>();
const jobStoreRoot = path.resolve(process.env.VERGESTAR_NOVEL_JOB_ROOT || path.join(process.cwd(), ".jobs"));

type PersistedNovelAgentJob = { job: NovelAgentJob; userId: string };

loadPersistedJobs();

export function startNovelAgentJob(input: FullNovelAgentTurnInput): NovelAgentJob {
    const now = new Date().toISOString();
    const job: NovelAgentJob = { id: randomUUID(), status: "queued", stage: "排队中", logs: [], createdAt: now, updatedAt: now };
    jobs.set(job.id, job);
    jobStreams.set(job.id, { emitter: new EventEmitter(), buffer: [] });
    jobOwners.set(job.id, input.userId);
    persistJob(job, input.userId);
    const controller = new AbortController();
    jobControllers.set(job.id, controller);
    jobAbortTargets.set(job.id, { userId: input.userId, sessionId: input.sessionId });
    void executeNovelAgentJob(job, { ...input, abortSignal: controller.signal });
    return publicJob(job);
}

export function getNovelAgentJob(id: string, userId?: string): NovelAgentJob | undefined {
    if (userId && jobOwners.get(id) !== userId) return undefined;
    const job = jobs.get(id);
    return job ? publicJob(job) : undefined;
}

async function executeNovelAgentJob(job: NovelAgentJob, input: FullNovelAgentTurnInput) {
    const controller = jobControllers.get(job.id)!;
    const wantsLock = input.confirmedIntent ? BOOK_WRITING_INTENTS.has(input.confirmedIntent) : Boolean(input.bookId);
    const lockKey = productionLockKey(input);
    if (wantsLock && productionLocks.get(lockKey)) {
        job.status = "failed";
        job.error = "该书已有生产任务在运行，请等待完成或取消后再试";
        job.stage = "任务被单写入锁拒绝";
        job.updatedAt = new Date().toISOString();
        persistJob(job, input.userId);
        jobControllers.delete(job.id);
        jobAbortTargets.delete(job.id);
        return;
    }
    if (wantsLock) productionLocks.set(lockKey, true);
    job.status = "running";
    updateJob(job, "启动 InkOS Agent Harness");
    try {
        job.result = await runFullNovelAgentTurn({
            ...input,
            onLog: (message: string) => {
                updateJob(job, message);
                pushJobStreamEvent(job.id, "log", message);
            },
            onStreamEvent: (streamEvent) => pushJobStreamEvent(job.id, streamEvent.type, streamEvent),
        });
        job.status = "succeeded";
        updateJob(job, "执行完成");
        pushJobStreamEvent(job.id, "done", { status: "succeeded" });
    } catch (error) {
        job.status = "failed";
        job.error = controller.signal.aborted ? "任务已取消" : (error instanceof Error ? error.message : String(error));
        updateJob(job, controller.signal.aborted ? "任务已取消" : "执行失败");
        pushJobStreamEvent(job.id, "done", { status: "failed", error: job.error });
    } finally {
        if (wantsLock) productionLocks.delete(lockKey);
        jobControllers.delete(job.id);
        jobAbortTargets.delete(job.id);
    }
}

function updateJob(job: NovelAgentJob, message: string) {
    job.stage = message;
    job.logs = [...job.logs, message].slice(-100);
    job.updatedAt = new Date().toISOString();
    const owner = jobOwners.get(job.id);
    if (owner) persistJob(job, owner);
}

function publicJob(job: NovelAgentJob): NovelAgentJob {
    return { ...job, logs: [...job.logs], ...(job.result ? { result: job.result } : {}), ...(job.error ? { error: job.error } : {}) };
}

function loadPersistedJobs() {
    mkdirSync(jobStoreRoot, { recursive: true });
    for (const fileName of readdirSync(jobStoreRoot, { withFileTypes: true }).filter((entry) => entry.isFile() && entry.name.endsWith(".json"))) {
        try {
            const persisted = JSON.parse(readFileSync(path.join(jobStoreRoot, fileName.name), "utf8")) as PersistedNovelAgentJob;
            if (!persisted || !persisted.job || typeof persisted.job.id !== "string" || typeof persisted.userId !== "string") continue;
            const job = persisted.job;
            // A process cannot safely resume an in-flight model call without
            // persisting credentials. Expose an explicit retryable terminal
            // state instead of leaving a stale spinner after a restart.
            if (job.status === "queued" || job.status === "running") {
                job.status = "failed";
                job.error = "服务重启导致任务中断，请重新确认创建";
                job.stage = "任务已中断，可重新确认创建";
                job.updatedAt = new Date().toISOString();
                persistJob(job, persisted.userId);
            }
            jobs.set(job.id, job);
            jobOwners.set(job.id, persisted.userId);
        } catch {
            // Ignore one corrupt historical snapshot; the current process can
            // still serve healthy jobs and will not trust its contents.
        }
    }
}

function persistJob(job: NovelAgentJob, userId: string) {
    mkdirSync(jobStoreRoot, { recursive: true });
    const filePath = path.join(jobStoreRoot, `${job.id}.json`);
    const tempPath = `${filePath}.${process.pid}.tmp`;
    writeFileSync(tempPath, JSON.stringify({ job: publicJob(job), userId }), "utf8");
    // The store is process-local metadata. Rename replaces the old snapshot
    // in one filesystem operation, so a crash cannot expose partial JSON.
    renameSync(tempPath, filePath);
}

export function listUserNovelAgentJobs(userId: string): NovelAgentJob[] {
    const result: NovelAgentJob[] = [];
    for (const [id, job] of jobs) {
        if (jobOwners.get(id) !== userId) continue;
        result.push(publicJob(job));
    }
    return result.sort((left, right) => right.createdAt.localeCompare(left.createdAt));
}

export function persistedNovelAgentJobRoot() {
    return jobStoreRoot;
}

export function cancelNovelAgentJob(id: string, userId?: string): boolean {
    if (userId && jobOwners.get(id) !== userId) return false;
    const controller = jobControllers.get(id);
    if (!controller) return false;
    controller.abort();
    // Planning turns run through InkOS's cached pi-agent session. The local
    // AbortController only reaches production pipelines, so also abort and evict
    // that session; otherwise a cancelled write would resume upstream and later
    // commit InkOS files after the UI already showed a failed/cancelled job.
    const target = jobAbortTargets.get(id);
    if (target) {
        try {
            abortNovelAgentSession(target.userId, target.sessionId);
        } catch {
            // The controller remains the fallback for turns without a cached agent.
        }
    }
    return true;
}


