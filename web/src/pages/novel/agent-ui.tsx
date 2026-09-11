import { Loader2, CheckCircle2, XCircle, ChevronDown } from "lucide-react";
import { useEffect, useState } from "react";
import { Streamdown } from "streamdown";
import type { NovelAgentConfirmation, NovelAgentJob } from "@/services/api/novel-agent";

// InkOS Studio 风格的小说 Agent UI 基件：确认卡、任务时间线卡、消息气泡。
// 仅做视觉与交互复刻，不复制 InkOS 源码；数据仍来自 Vergestar Novel Agent API。

export function formatJobElapsed(startedAt: string | number, finishedAt?: string | number): string {
    const start = typeof startedAt === "number" ? startedAt : new Date(startedAt).getTime();
    if (!Number.isFinite(start)) return "";
    const end = finishedAt ? (typeof finishedAt === "number" ? finishedAt : new Date(finishedAt).getTime()) : Date.now();
    const secs = Math.max(0, Math.round((end - start) / 1000));
    return secs < 60 ? `${secs}s` : `${Math.floor(secs / 60)}m ${secs % 60}s`;
}

function useElapsedTicker(startedAt: string | undefined, active: boolean): string {
    const [, setTick] = useState(0);
    useEffect(() => {
        if (!active || !startedAt) return;
        const timer = window.setInterval(() => setTick((value) => value + 1), 1000);
        return () => window.clearInterval(timer);
    }, [active, startedAt]);
    return startedAt ? formatJobElapsed(startedAt) : "";
}

function TaskStatusBadge({ status }: { status: NovelAgentJob["status"] }) {
    if (status === "queued" || status === "running") {
        return (
            <span className="inline-flex items-center gap-1 text-xs text-primary">
                <Loader2 className="size-3 animate-spin" />
                <span>执行中</span>
            </span>
        );
    }
    if (status === "succeeded") {
        return (
            <span className="inline-flex items-center gap-1 text-xs text-emerald-600 dark:text-emerald-400">
                <CheckCircle2 className="size-3" />
                <span>已完成</span>
            </span>
        );
    }
    return (
        <span className="inline-flex items-center gap-1 text-xs text-destructive">
            <XCircle className="size-3" />
            <span>失败</span>
        </span>
    );
}

/** 从 Job 日志推导 InkOS 式阶段清单：所有"阶段：N"行按序去重，前面的视为已完成，最后一项为当前阶段。 */
function deriveJobStages(job: NovelAgentJob): Array<{ label: string; state: "done" | "active" | "failed" }> {
    const labels: string[] = [];
    for (const log of job.logs || []) {
        const match = log.match(/^\s*(?:[-*]\s*)?(阶段[^：:]*[：:]|Stage\s*\d[^：:]*)\s*(.+)$/);
        const label = match ? match[2].trim() : /^阶段/.test(log.trim()) ? log.trim().replace(/^阶段[：:]?\s*/, "阶段：") : "";
        if (label && !labels.includes(label)) labels.push(label);
    }
    if (!labels.length && job.stage) labels.push(job.stage);
    if (job.status === "succeeded") return labels.map((label) => ({ label, state: "done" as const }));
    if (job.status === "failed") return labels.map((label, index) => ({ label, state: index === labels.length - 1 ? ("failed" as const) : ("done" as const) }));
    return labels.map((label, index) => ({ label, state: index === labels.length - 1 ? ("active" as const) : ("done" as const) }));
}

function StageRow({ state, label }: { state: "done" | "active" | "failed"; label: string }) {
    return (
        <li className={`flex items-center gap-2 rounded-md px-2 py-1 text-sm ${state === "active" ? "bg-primary/5 text-foreground" : "text-muted-foreground"}`}>
            {state === "done" ? (
                <CheckCircle2 className="size-3.5 shrink-0 text-emerald-600 dark:text-emerald-400" />
            ) : state === "active" ? (
                <Loader2 className="size-3.5 shrink-0 animate-spin text-primary" />
            ) : (
                <XCircle className="size-3.5 shrink-0 text-destructive" />
            )}
            <span className="truncate">{label}</span>
        </li>
    );
}

/** InkOS 风格的生产任务卡：标题 + 耗时 + 状态徽章 + 阶段时间线 + 逐章进度 + 日志。 */
export function AgentTaskCard({ job, title, onCancel, targetChapters }: { job: NovelAgentJob; title: string; onCancel?: (jobId: string) => void; targetChapters?: number }) {
    const [logsOpen, setLogsOpen] = useState(false);
    const active = job.status === "queued" || job.status === "running";
    const elapsed = useElapsedTicker(job.createdAt, active);
    const stages = deriveJobStages(job);
    // 逐章进度（T3）：多章写作时从日志提取已落盘的章号（"第 N 章"去重），显示 N/M 进度点。
    const chapterNumbers = new Set<number>();
    for (const log of job.logs || []) {
        const match = log.match(/第\s*(\d+)\s*章/);
        if (match) chapterNumbers.add(Number(match[1]));
    }
    const completedChapters = [...chapterNumbers].sort((a, b) => a - b);
    const showChapterProgress = Boolean(targetChapters && targetChapters > 1);
    const recentLogs = (job.logs || []).slice(-6);
    return (
        <section className="w-full max-w-[90%] rounded-xl border border-border/60 bg-surface-card/60 p-3" aria-label="生产任务">
            <div className="flex items-center justify-between gap-2">
                <button type="button" className="flex flex-1 items-center gap-2 text-left" onClick={() => setLogsOpen((value) => !value)} aria-expanded={logsOpen}>
                    <span className="text-sm font-semibold">{title}</span>
                    {elapsed ? <span className="text-xs tabular-nums text-muted-foreground">{elapsed}</span> : null}
                    <TaskStatusBadge status={job.status} />
                    <ChevronDown className={`ml-auto size-4 text-muted-foreground transition-transform ${logsOpen ? "rotate-180" : ""}`} />
                </button>
                {active && onCancel ? (
                    <button type="button" className="rounded-md border border-border/60 px-2 py-1 text-xs text-muted-foreground transition-colors hover:border-destructive/40 hover:text-destructive" onClick={() => onCancel(job.id)}>
                        取消
                    </button>
                ) : null}
            </div>
            {stages.length ? (
                <ul className="mt-2 grid gap-0.5">
                    {stages.map((stage) => (
                        <StageRow key={stage.label} state={stage.state} label={stage.label} />
                    ))}
                </ul>
            ) : null}
            {showChapterProgress ? (
                <div className="mt-2 flex flex-wrap items-center gap-1" aria-label="逐章进度">
                    <span className="text-xs text-muted-foreground">章节进度 {completedChapters.length}/{targetChapters}</span>
                    <span className="flex flex-wrap items-center gap-1">
                        {Array.from({ length: Math.min(targetChapters ?? 0, 20) }, (_, index) => {
                            const chapter = index + 1;
                            const done = completedChapters.includes(chapter);
                            return (
                                <span
                                    key={chapter}
                                    title={`第 ${chapter} 章${done ? "已落盘" : ""}`}
                                    className={`inline-flex h-4 w-4 items-center justify-center rounded text-[9px] tabular-nums ${done ? "bg-emerald-500/15 text-emerald-600 dark:text-emerald-400" : "bg-surface-active text-muted-foreground/50"}`}
                                >
                                    {chapter}
                                </span>
                            );
                        })}
                    </span>
                </div>
            ) : null}
            {logsOpen && (job.logs || []).length ? (
                <div className="mt-2 max-h-44 overflow-y-auto rounded-md bg-surface-active/60 px-2 py-1.5 text-xs leading-5 text-muted-foreground">
                    {(job.logs || []).slice(-40).map((log, index) => (
                        <p key={`${log}-${index}`} className="break-all">{log}</p>
                    ))}
                </div>
            ) : !logsOpen && recentLogs.length ? (
                <div className="mt-2 grid gap-0.5 text-xs leading-5 text-muted-foreground/70">
                    {recentLogs.slice(-2).map((log, index) => (
                        <p key={`${log}-${index}`} className="truncate">{log}</p>
                    ))}
                </div>
            ) : null}
            {job.status === "failed" && job.error ? (
                <p className="mt-2 text-xs leading-5 text-destructive">{job.error}</p>
            ) : null}
        </section>
    );
}

/** InkOS 风格确认卡：标题 + 摘要 + 完整方案 + 继续执行 / 取消。 */
export function AgentConfirmationCard({ confirmation, pending, onConfirm, onCancel }: {
    confirmation: NovelAgentConfirmation;
    pending?: boolean;
    onConfirm: () => void;
    onCancel: () => void;
}) {
    return (
        <section className="w-full max-w-[90%] rounded-xl border border-primary/25 bg-primary/[0.04] p-4" aria-label="确认动作">
            <p className="text-xs font-semibold uppercase tracking-wide text-primary/80">确认动作</p>
            <h4 className="mt-1.5 text-[15px] font-semibold leading-6">{confirmation.title}</h4>
            <p className="mt-1 text-sm leading-6 text-muted-foreground">{confirmation.summary}</p>
            {confirmation.instruction && confirmation.instruction !== confirmation.summary ? (
                <p className="mt-2 max-h-64 overflow-y-auto whitespace-pre-wrap rounded-lg bg-surface-active/50 px-3 py-2 text-[13px] leading-6 text-foreground/75">{confirmation.instruction}</p>
            ) : null}
            <div className="mt-3 flex items-center justify-end gap-2">
                <button type="button" disabled={pending} className="rounded-lg border border-border/60 px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:bg-surface-hover disabled:opacity-50" onClick={onCancel}>
                    取消
                </button>
                <button type="button" disabled={pending} className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-3.5 py-1.5 text-sm font-semibold text-primary-foreground transition-all hover:opacity-90 active:scale-95 disabled:opacity-50" onClick={onConfirm}>
                    {pending ? <Loader2 className="size-3.5 animate-spin" /> : null}
                    继续执行
                </button>
            </div>
        </section>
    );
}

/** InkOS 风格聊天气泡：用户右对齐浅色块，助手左对齐 Markdown 渲染（与 InkOS 同用 Streamdown）。 */
export function AgentBubble({ role, children }: { role: "user" | "assistant"; children: React.ReactNode }) {
    if (role === "user") {
        return (
            <div className="ml-auto w-fit max-w-[85%] rounded-lg bg-muted px-4 py-3 text-[15px] leading-[1.72] whitespace-pre-wrap text-foreground">
                {children}
            </div>
        );
    }
    return (
        <div className="max-w-[95%] text-[15px] leading-[1.78] text-foreground/85 [&_h1]:mt-3 [&_h1]:mb-1.5 [&_h1]:text-lg [&_h1]:font-semibold [&_h2]:mt-3 [&_h2]:mb-1.5 [&_h2]:text-base [&_h2]:font-semibold [&_h3]:mt-2.5 [&_h3]:mb-1 [&_h3]:text-[15px] [&_h3]:font-semibold [&_ol]:list-decimal [&_ol]:pl-5 [&_p]:my-1.5 [&_p:first-child]:mt-0 [&_p:last-child]:mb-0 [&_strong]:font-semibold [&_strong]:text-foreground [&_ul]:list-disc [&_ul]:pl-5 [&_li]:my-0.5">
            <Streamdown>{typeof children === "string" ? children : String(children ?? "")}</Streamdown>
        </div>
    );
}


// --- Play 开放世界游玩富 UI（T5）：建议行动解析为可点击按钮 ---

export interface PlaySuggestions {
    /** 去除建议行后的正文 */
    body: string;
    /** 提取出的建议行动 */
    actions: string[];
}

/** 从 Play 回复中解析【建议前往：X】【你可以：Y】等建议行（InkOS guided 语义）。 */
export function parsePlaySuggestions(text: string): PlaySuggestions {
    const actions: string[] = [];
    const body = text
        .split("\n")
        .filter((line) => {
            const match = line.match(/^\s*[\u3010\[【]?\s*(?:建议[^\u3010\[】\]]*[：:]|你可以[：:]|可选行动[：:])\s*([^\u3010\[】\]]+)\s*[\u3011\]]?\s*$/);
            if (match) {
                actions.push(match[1].trim());
                return false;
            }
            return true;
        })
        .join("\n")
        .trim();
    return { body, actions };
}
