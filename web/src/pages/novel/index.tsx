import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { App, Button, Form, Input, InputNumber, Modal, Popconfirm, Select, Tag } from "antd";
import { ArrowRight, BookOpenText, Check, CheckCircle2, FileOutput, FileText, Film, GitBranch, Hand, History, Lightbulb, Loader2, Pencil, Plus, RotateCcw, Save, SearchCheck, Send, Settings2, Sparkles, Trash2, TrendingUp, X, XCircle, Zap } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router";

import { AgentBubble, AgentConfirmationCard, AgentTaskCard, parsePlaySuggestions } from "@/pages/novel/agent-ui";
import { CollectionGrid, PageHeader, WorkspacePage } from "@/components/layout/workspace-page";
import { WorkspaceErrorState, WorkspaceLoadingState, WorkspaceState } from "@/components/layout/workspace-state";
import { ModelPicker } from "@/components/model-picker";
import { cancelNovelAgentJob, deleteNovelAgentSession, getNovelAgentJob, getNovelAgentSessionMessages, listNovelAgentActiveJobs, listNovelAgentJobHistory, listNovelAgentSessions, listNovelAgentSkills, renameNovelAgentSession, runNovelAgentTurn, searchNovelAgentSessions, startNovelAgentJob, type NovelAgentConfirmation, type NovelAgentJob, type NovelAgentSkill } from "@/services/api/novel-agent";
import { confirmProjectAssetCandidate, createProject, createProjectCharacter, createProjectUnit, createUnitWorkflow, getProject, linkShotAsset, listProjects, saveProjectShot, updateProjectUnit, type ProjectAssetCandidate, type ProjectDetail, type ProjectSummary, type ProjectUnit } from "@/services/api/projects";
import { submitBackendGenerationTask } from "@/services/api/generation-task";
import {
    createStoryBranch,
    createStoryMemory,
    createStoryReview,
    createStoryScene,
    createStorySceneShot,
    getStoryFoundation,
    listStoryBranches,
    listStoryChapterVersions,
    listStoryMemories,
    listStoryReviews,
    listStoryScenes,
    evaluateNovelBook,
    listNovelExports,
    readNovelExport,
    readNovelTruth,
    restoreStoryChapterVersion,
    runMarketRadar,
    writeNovelTruth,
    approveInkosChapter,
    deleteInkosBook,
    rejectInkosChapter,
    snapshotStoryChapter,
    updateInkosBookSettings,
    type StoryFoundationView,
} from "@/services/api/story";
import { resolveModelRequestConfig, selectableModelsByCapability, useConfigStore, useEffectiveConfig } from "@/stores/use-config-store";

type NovelForm = {
    name: string;
    idea: string;
    genre: string;
    language: "zh" | "en";
};

const genres = ["悬疑", "科幻", "奇幻", "言情", "现实", "武侠", "历史", "都市", "其他"];


function NovelJobCancelButton({ jobId }: { jobId: string }) {
    const message = App.useApp().message;
    const cancel = useMutation({
        mutationFn: () => cancelNovelAgentJob(jobId),
        onSuccess: () => message.info("已发送取消请求，等待 InkOS 安全停止"),
        onError: (error) => message.error(error instanceof Error ? error.message : "取消任务失败"),
    });
    return <Button size="small" danger className="mt-2" loading={cancel.isPending} onClick={() => cancel.mutate()}>取消任务</Button>;
}

function novelDescription(input: NovelForm) {
    return JSON.stringify({ version: 1, kind: "novel", idea: input.idea.trim(), genre: input.genre, language: input.language });
}

function novelMetadata(description: string) {
    try {
        const value = JSON.parse(description) as { kind?: unknown; idea?: unknown; genre?: unknown; language?: unknown };
        if (value.kind !== "novel") return null;
        return {
            idea: typeof value.idea === "string" ? value.idea : "",
            genre: typeof value.genre === "string" ? value.genre : "其他",
            language: value.language === "en" ? "en" : "zh",
        };
    } catch {
        return null;
    }
}

export default function NovelPage() {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const { message } = App.useApp();
    const [searchParams, setSearchParams] = useSearchParams();
    const [form] = Form.useForm<NovelForm>();
    const [selected, setSelected] = useState<ProjectSummary | null>(null);
    const createOpen = searchParams.get("create") === "1";
    const projects = useQuery({ queryKey: ["projects", "novels"], queryFn: () => listProjects() });
    const novels = useMemo(() => (projects.data?.projects || []).filter((item) => item.project.type === "novel" || novelMetadata(item.project.description)), [projects.data]);
    const create = useMutation({
        mutationFn: async (input: NovelForm) =>
            createProject({
                name: input.name.trim(),
                type: "novel",
                sourceType: "idea",
                aspectRatio: "16:9",
                description: novelDescription(input),
            }),
        onSuccess: ({ project }) => {
            void queryClient.invalidateQueries({ queryKey: ["projects"] });
            setSearchParams({}, { replace: true });
            form.resetFields();
            navigate(`/novel?project=${encodeURIComponent(project.id)}`);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "小说项目创建失败"),
    });

    const openProject = (summary: ProjectSummary) => {
        setSelected(summary);
        setSearchParams({ project: summary.project.id }, { replace: true });
    };

    const selectedProjectID = searchParams.get("project");
    const active = selected || novels.find((item) => item.project.id === selectedProjectID) || null;

    if (projects.isLoading)
        return (
            <WorkspacePage>
                <WorkspaceLoadingState label="正在加载小说项目" detail="读取你的故事和章节" />
            </WorkspacePage>
        );
    if (projects.isError)
        return (
            <WorkspacePage>
                <WorkspaceErrorState title="小说项目加载失败" onRetry={() => void projects.refetch()} />
            </WorkspacePage>
        );

    return (
        <WorkspacePage grid>
            {!active ? (
                <PageHeader
                    title="小说创作"
                    description="从一个想法开始，沉淀角色、章节与故事结构，再进入剧本和分镜生产。"
                    actions={
                        <Button type="primary" icon={<Plus className="size-4" />} onClick={() => setSearchParams({ create: "1" })}>
                            新建小说
                        </Button>
                    }
                />
            ) : null}
            {active ? (
                <NovelProjectPanel
                    projectId={active.project.id}
                    onBack={() => {
                        setSelected(null);
                        setSearchParams({}, { replace: true });
                    }}
                    onOpenDrama={() => navigate(`/projects/${active.project.id}/overview`)}
                />
            ) : (
                <NovelAgentHome novels={novels} onOpenProject={openProject} onCreateProject={() => setSearchParams({ create: "1" })} />
            )}
            <Modal title="新建小说项目" open={createOpen} onCancel={() => setSearchParams({}, { replace: true })} footer={null} destroyOnHidden>
                <Form form={form} layout="vertical" initialValues={{ genre: "悬疑", language: "zh" }} onFinish={(input) => create.mutate(input)}>
                    <Form.Item label="小说名称" name="name" rules={[{ required: true, whitespace: true, message: "请输入小说名称" }]}>
                        <Input maxLength={80} placeholder="例如：雾城来信" autoFocus />
                    </Form.Item>
                    <Form.Item label="创作想法" name="idea" rules={[{ required: true, whitespace: true, message: "写下一段故事想法" }]}>
                        <Input.TextArea rows={5} maxLength={5000} placeholder="主角、冲突、世界、你想传达的情绪，任何一个起点都可以。" />
                    </Form.Item>
                    <div className="grid grid-cols-2 gap-3">
                        <Form.Item label="类型" name="genre">
                            <Select options={genres.map((value) => ({ value, label: value }))} />
                        </Form.Item>
                        <Form.Item label="语言" name="language">
                            <Select
                                options={[
                                    { value: "zh", label: "中文" },
                                    { value: "en", label: "English" },
                                ]}
                            />
                        </Form.Item>
                    </div>
                    <div className="mt-2 flex justify-end gap-2">
                        <Button onClick={() => setSearchParams({}, { replace: true })}>取消</Button>
                        <Button htmlType="submit" type="primary" loading={create.isPending}>
                            创建小说
                        </Button>
                    </div>
                </Form>
            </Modal>
        </WorkspacePage>
    );
}

const creationModes = [
    { id: "inkos-long-writing", label: "长篇小说" },
    { id: "inkos-short-writing", label: "短篇小说" },
    { id: "inkos-script-writing", label: "剧本创作" },
    { id: "inkos-storyboard", label: "分镜创作" },
    { id: "inkos-interactive-film", label: "互动影游" },
    { id: "inkos-play-world", label: "开放世界" },
    { id: "inkos-story-review", label: "审稿修订" },
    { id: "inkos-translation", label: "翻译译介" },
];

type AgentMessage = { role: "assistant" | "user"; text: string; skills?: NovelAgentSkill[]; confirmation?: NovelAgentConfirmation; storyProjectId?: string; bookId?: string };
type AgentTurnInput = { message: string; confirmation?: NonNullable<AgentMessage["confirmation"]>; bookId?: string };

const activeNovelAgentJobStorageKey = "vergestar.novel-agent.active-job";

function readActiveNovelAgentJobID() {
    try {
        return window.localStorage.getItem(activeNovelAgentJobStorageKey) || "";
    } catch {
        return "";
    }
}

function writeActiveNovelAgentJobID(jobID: string) {
    try {
        window.localStorage.setItem(activeNovelAgentJobStorageKey, jobID);
    } catch {
        // Storage policy must not prevent a server-side job from running.
    }
}

function clearActiveNovelAgentJobID(jobID?: string) {
    try {
        if (!jobID || window.localStorage.getItem(activeNovelAgentJobStorageKey) === jobID) window.localStorage.removeItem(activeNovelAgentJobStorageKey);
    } catch {
        // Ignore cleanup failures; the job itself remains authoritative.
    }
}

function formatRelativeTime(iso: string) {
    const time = new Date(iso).getTime();
    if (!Number.isFinite(time)) return "";
    const minutes = Math.round((Date.now() - time) / 60000);
    if (minutes < 1) return "刚刚";
    if (minutes < 60) return `${minutes} 分钟`;
    const hours = Math.round(minutes / 60);
    if (hours < 24) return `${hours} 小时`;
    return `${Math.round(hours / 24)} 天`;
}

function NovelAgentHome({ novels, onOpenProject, onCreateProject }: { novels: ProjectSummary[]; onOpenProject: (summary: ProjectSummary) => void; onCreateProject: () => void }) {
    const { message } = App.useApp();
    const config = useEffectiveConfig();
    const queryClient = useQueryClient();
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const skillCatalog = useQuery({ queryKey: ["novel-agent", "skills"], queryFn: listNovelAgentSkills });
    const sessions = useQuery({ queryKey: ["novel-agent", "sessions"], queryFn: listNovelAgentSessions });
    const jobHistory = useQuery({ queryKey: ["novel-agent", "job-history"], queryFn: listNovelAgentJobHistory, refetchInterval: 15000 });
    const [sessionSearch, setSessionSearch] = useState("");
    const sessionSearchQuery = useQuery({
        queryKey: ["novel-agent", "session-search", sessionSearch],
        queryFn: () => searchNovelAgentSessions(sessionSearch, 1, 20),
        enabled: sessionSearch.trim().length > 0,
    });
    const sessionList = useMemo(
        () => (sessionSearch.trim() ? sessionSearchQuery.data?.sessions ?? [] : sessions.data?.sessions ?? []),
        [sessionSearch, sessionSearchQuery.data?.sessions, sessions.data?.sessions],
    );
    const [selectedSkillId, setSelectedSkillId] = useState("inkos-long-writing");
    const [sessionId, setSessionId] = useState("");
    const [restoredSessionId, setRestoredSessionId] = useState("");
    const [isNewSession, setIsNewSession] = useState(false);
    const [draft, setDraft] = useState("");
    const [jobProgress, setJobProgress] = useState<NovelAgentJob | null>(null);
    const [streamText, setStreamText] = useState("");
    const [streamTool, setStreamTool] = useState("");
    const [lastJob, setLastJob] = useState<NovelAgentJob | null>(null);
    const [lastRequestedChapters, setLastRequestedChapters] = useState(1);
    const [recoveredJobId, setRecoveredJobId] = useState(readActiveNovelAgentJobID);
    const [boundProjectId, setBoundProjectId] = useState("");
    const [messages, setMessages] = useState<AgentMessage[]>([{ role: "assistant", text: "说一个故事方向、标题灵感、人物压力或核心冲突。我会先使用合适的创作 Skill 组织方案；确认后才会开始建书、写作或生成影视产物。" }]);
    const sessionMessages = useQuery({
        queryKey: ["novel-agent", "session", sessionId, "messages"],
        queryFn: () => getNovelAgentSessionMessages(sessionId),
        enabled: Boolean(sessionId),
    });
    // InkOS 式右侧真相面板：会话绑定小说项目后展示章节 / 角色 / 核心文件。
    const activeProjectId = useMemo(() => {
        const session = sessionList.find((item) => item.id === sessionId);
        return session?.projectId || boundProjectId || "";
    }, [boundProjectId, sessionId, sessionList]);
    const activeNovel = useMemo(() => novels.find((item) => item.project.id === activeProjectId) || null, [activeProjectId, novels]);
    const bookDetail = useQuery({
        queryKey: ["project", activeProjectId, "novel-book-sidebar"],
        queryFn: () => getProject(activeProjectId),
        enabled: Boolean(activeProjectId),
    });
    const bookFoundation = useQuery({
        queryKey: ["story", activeProjectId, "foundation"],
        queryFn: () => getStoryFoundation(activeProjectId),
        enabled: Boolean(activeProjectId),
    });
    const bookUnits = bookDetail.data?.units || [];
    const bookCharacters = (bookDetail.data?.assets || []).filter((asset) => asset.category === "character");
    const bookCoreFiles = Object.keys(bookFoundation.data?.files || {}).filter((path) => path.startsWith("story/") || path.startsWith("chapters/") || path.startsWith("shorts/"));
    useEffect(() => {
        if (!recoveredJobId && !isNewSession && !sessionId && sessionList.length) setSessionId(sessionList[0].id);
    }, [isNewSession, recoveredJobId, sessionId, sessionList]);
    // 断线恢复（T3）：本地没有活动任务指针时，查 DB 镜像里该用户的 running 任务并接续轮询。
    const activeJobs = useQuery({ queryKey: ["novel-agent", "active-jobs"], queryFn: listNovelAgentActiveJobs, enabled: !recoveredJobId, refetchInterval: false, staleTime: 5_000 });
    const recoveredActiveId = activeJobs.data?.jobs?.find((job) => job.status === "queued" || job.status === "running")?.id;
    useEffect(() => {
        if (!recoveredActiveId || jobProgress || lastJob) return;
        writeActiveNovelAgentJobID(recoveredActiveId);
        setRecoveredJobId(recoveredActiveId);
    }, [recoveredActiveId, jobProgress, lastJob]);
    useEffect(() => {
        if (!sessionMessages.data || restoredSessionId === sessionId) return;
        const restored = sessionMessages.data.messages.map((item) => ({
            role: item.role,
            text: item.content,
            ...(item.skills?.length ? { skills: item.skills } : {}),
            ...(item.confirmation ? { confirmation: item.confirmation } : {}),
        }));
        if (restored.length) setMessages(restored);
        setRestoredSessionId(sessionId);
    }, [restoredSessionId, sessionId, sessionMessages.data]);
    const openSession = (nextSessionId: string) => {
        setIsNewSession(false);
        setSessionId(nextSessionId);
        setRestoredSessionId("");
    };
    const startNewSession = () => {
        setIsNewSession(true);
        setSessionId("");
        setRestoredSessionId("");
        setBoundProjectId("");
        setMessages([{ role: "assistant", text: "说一个故事方向、标题灵感、人物压力或核心冲突。我会先使用合适的创作 Skill 组织方案；确认后才会开始建书、写作或生成影视产物。" }]);
    };
    useEffect(() => {
        if (!recoveredJobId) return;
        let cancelled = false;
        const recover = async () => {
            let current: NovelAgentJob;
            try {
                current = (await getNovelAgentJob(recoveredJobId)).job;
            } catch {
                clearActiveNovelAgentJobID(recoveredJobId);
                return;
            }
            while (!cancelled && (current.status === "queued" || current.status === "running")) {
                setJobProgress(current);
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
                try {
                    current = (await getNovelAgentJob(recoveredJobId)).job;
                } catch {
                    return;
                }
            }
            if (cancelled) return;
            if (current.status === "succeeded" && current.result) {
                if (!current.storyProjectId) {
                    setJobProgress(current);
                    return;
                }
                clearActiveNovelAgentJobID(recoveredJobId);
                setSessionId(current.result.session.id);
                setMessages((items) => [...items, { role: "assistant", text: current.result!.text, skills: current.result!.skills }]);
            } else if (current.status === "failed") {
                clearActiveNovelAgentJobID(recoveredJobId);
                message.error(current.error || current.stage || "建书任务失败");
            }
            setJobProgress(null);
        };
        void recover();
        return () => {
            cancelled = true;
        };
    }, [message, recoveredJobId]);
    // 断线恢复轮询（T3）：接续 DB 里 running 的任务直到终态。
    useEffect(() => {
        if (!recoveredActiveId || jobProgress) return;
        let cancelled = false;
        let current = { id: recoveredActiveId, status: "running" } as NovelAgentJob;
        void (async () => {
            while (!cancelled) {
                try {
                    current = (await getNovelAgentJob(current.id)).job;
                } catch {
                    return;
                }
                if (cancelled) return;
                setJobProgress(current);
                if (current.status !== "queued" && current.status !== "running") break;
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
            }
            if (cancelled) return;
            setLastJob(current);
            setJobProgress(null);
            if (current.status === "succeeded" && current.result) {
                clearActiveNovelAgentJobID(current.id);
                if (current.storyProjectId) {
                    setBoundProjectId(current.storyProjectId);
                    void queryClient.invalidateQueries({ queryKey: ["projects"] });
                }
                setMessages((items) => [...items, { role: "assistant", text: current.result!.text, skills: current.result!.skills, ...(current.storyProjectId ? { storyProjectId: current.storyProjectId } : {}) }]);
            } else if (current.status === "failed") {
                clearActiveNovelAgentJobID(current.id);
                message.warning(current.error || "任务已结束（恢复自服务重启前的记录）");
            }
            void queryClient.invalidateQueries({ queryKey: ["novel-agent", "active-jobs"] });
            void queryClient.invalidateQueries({ queryKey: ["novel-agent", "job-history"] });
        })();
        return () => { cancelled = true; };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [recoveredActiveId]);
    // 流式输出（T3）：任务运行时订阅 SSE，token 增量实时渲染；轮询仍是状态真相。
    const streamJobId = jobProgress?.status === "running" || jobProgress?.status === "queued" ? jobProgress.id : "";
    useEffect(() => {
        if (!streamJobId) return;
        setStreamText("");
        setStreamTool("");
        const source = new EventSource(`/api/novel-agent/jobs/${streamJobId}/stream`);
        source.addEventListener("delta", (event) => {
            try {
                const data = JSON.parse((event as MessageEvent).data) as { text?: string };
                if (data.text) setStreamText((prev) => (prev + data.text).slice(-8000));
            } catch { /* 单个坏事件直接丢弃 */ }
        });
        source.addEventListener("tool", (event) => {
            try {
                const data = JSON.parse((event as MessageEvent).data) as { toolName?: string };
                if (data.toolName) setStreamTool(data.toolName);
            } catch { /* 忽略 */ }
        });
        source.addEventListener("tool_end", () => setStreamTool(""));
        const stop = () => { source.close(); };
        source.onerror = stop;
        return stop;
    }, [streamJobId]);
    // 任务终态：清空流式预览（最终消息由轮询路径追加）。
    useEffect(() => {
        if (!jobProgress) {
            setStreamText("");
            setStreamTool("");
        }
    }, [jobProgress]);
    const turn = useMutation({
        mutationFn: async ({ message: turnMessage, confirmation, bookId }: AgentTurnInput) => {
            const modelValue = config.textModel || config.model;
            if (!modelValue) throw new Error("请先在设置中选择文本模型");
            const requestConfig = resolveModelRequestConfig(config, modelValue);
            if (requestConfig.apiFormat !== "openai" || requestConfig.interfaceType === "claude-api" || requestConfig.interfaceType === "gemini-image" || requestConfig.interfaceType === "gemini-veo") {
                throw new Error("Novel Agent 首版仅支持 OpenAI 兼容文本渠道，请在设置中切换文本模型");
            }
            const input: Parameters<typeof runNovelAgentTurn>[0] = {
                ...(sessionId ? { sessionId } : {}),
                mode: selectedSkillId || "long-novel",
                message: turnMessage,
                requestedSkills: selectedSkillId ? [selectedSkillId] : [],
                ...(bookId ? { bookId } : {}),
                ...(confirmation ? { confirmedIntent: confirmation.action, confirmedActionPayload: confirmation.actionPayload } : {}),
                model: {
                    provider: "custom",
                    baseUrl: requestConfig.baseUrl,
                    apiKey: requestConfig.apiKey,
                    model: requestConfig.model,
                    apiFormat: requestConfig.interfaceType === "openai-responses" ? "responses" : "chat",
                    temperature: 0.7,
                },
            };
            if (confirmation) {
                const { job } = await startNovelAgentJob(input);
                writeActiveNovelAgentJobID(job.id);
                setLastJob(null);
                setJobProgress(job);
                let current = job;
                while (current.status === "queued" || current.status === "running") {
                    await new Promise((resolve) => window.setTimeout(resolve, 1500));
                    current = (await getNovelAgentJob(current.id)).job;
                    setJobProgress(current);
                }
                setLastJob(current);
                if (current.status === "failed") throw new Error(current.error || current.stage || "建书任务失败");
                if (!current.result) throw new Error("建书任务完成但没有返回结果");
                if (!current.storyProjectId) throw new Error("建书已完成，正在同步 Vergestar 小说项目；请刷新页面后继续同步");
                clearActiveNovelAgentJobID(current.id);
                return { ...current.result, storyProjectId: current.storyProjectId };
            }
            setJobProgress(null);
            return runNovelAgentTurn(input);
        },
        onSuccess: (result) => {
            setJobProgress(null);
            if (result.storyProjectId) {
                setBoundProjectId(result.storyProjectId);
                void queryClient.invalidateQueries({ queryKey: ["projects"] });
            }
            setMessages((items) => [...items, { role: "assistant", text: result.text, skills: result.skills, ...(result.confirmation ? { confirmation: result.confirmation } : {}), ...(result.storyProjectId ? { storyProjectId: result.storyProjectId } : {}) }]);
            setSessionId(result.session.id);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "Novel Agent 暂时不可用"),
    });
    const submit = () => {
        const text = draft.trim();
        if (!text || turn.isPending) return;
        setMessages((items) => [...items, { role: "user", text }]);
        // The submitted text already lives in the message list and mutation
        // argument, so the composer can reset while a long Agent turn runs.
        setDraft("");
        turn.mutate({ message: text });
    };
    const cancelJob = useMutation({
        mutationFn: (jobId: string) => cancelNovelAgentJob(jobId),
        onSuccess: () => message.info("已发送取消请求，等待 InkOS 安全停止"),
        onError: (error) => message.error(error instanceof Error ? error.message : "取消任务失败"),
    });
    // InkOS 书聊快捷命令（写下一章/审计/导出/市场雷达）：同款 chips，以异步 Job 执行。
    const runBookCommand = useMutation({
        mutationFn: async (input: { mode: string; message: string }) => {
            const bookId = bookFoundation.data?.inkosBookId;
            if (!bookId) throw new Error("当前会话还没有绑定的小说书籍");
            const modelValue = config.textModel || config.model;
            if (!modelValue) throw new Error("请先在设置中选择文本模型");
            const requestConfig = resolveModelRequestConfig(config, modelValue);
            if (requestConfig.apiFormat !== "openai" || requestConfig.interfaceType === "claude-api" || requestConfig.interfaceType === "gemini-image" || requestConfig.interfaceType === "gemini-veo") {
                throw new Error("Novel Agent 仅支持 OpenAI 兼容文本渠道，请在设置中切换文本模型");
            }
            const { job } = await startNovelAgentJob({
                ...(sessionId ? { sessionId } : {}),
                bookId,
                mode: input.mode,
                message: input.message,
                requestedSkills: ["inkos-long-writing"],
                model: {
                    provider: "custom",
                    baseUrl: requestConfig.baseUrl,
                    apiKey: requestConfig.apiKey,
                    model: requestConfig.model,
                    apiFormat: requestConfig.interfaceType === "openai-responses" ? "responses" : "chat",
                    temperature: 0.7,
                },
            });
            setLastJob(null);
            setJobProgress(job);
            let current = job;
            while (current.status === "queued" || current.status === "running") {
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
                current = (await getNovelAgentJob(current.id)).job;
                setJobProgress(current);
            }
            setLastJob(current);
            if (current.status === "failed") throw new Error(current.error || current.stage || "任务失败");
            if (!current.result) throw new Error("任务完成但没有返回结果");
            return current.result;
        },
        onSuccess: (result) => {
            setJobProgress(null);
            setMessages((items) => [...items, { role: "assistant", text: result.text, skills: result.skills, ...(result.confirmation ? { confirmation: result.confirmation, bookId: bookFoundation.data?.inkosBookId } : {}), ...(result.storyProjectId ? { storyProjectId: result.storyProjectId } : {}) }]);
        },
        onError: (error) => {
            setJobProgress(null);
            message.error(error instanceof Error ? error.message : "书籍命令执行失败");
        },
    });
    const renameSession = useMutation({
        mutationFn: ({ sessionId: sid, title }: { sessionId: string; title: string }) => renameNovelAgentSession(sid, title),
        onSuccess: () => {
            void sessions.refetch();
            message.success("会话已重命名");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "重命名失败"),
    });
    const deleteSession = useMutation({
        mutationFn: (sid: string) => deleteNovelAgentSession(sid),
        onSuccess: (_data, sid) => {
            if (sessionId === sid) startNewSession();
            void sessions.refetch();
            message.success("会话已删除");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "删除会话失败"),
    });
    const bookCommandChips = [
        { key: "write-next", icon: <Zap className="size-3" />, label: "写下一章", mode: "inkos-long-writing", message: "请读取当前小说的故事状态和章节进度，规划并提出写下一章的确认动作。不要直接写入正文。" },
        { key: "audit", icon: <SearchCheck className="size-3" />, label: "审计", mode: "book-audit", message: "审计" },
        { key: "export", icon: <FileOutput className="size-3" />, label: "导出", mode: "book-export", message: "导出全书" },
        { key: "radar", icon: <TrendingUp className="size-3" />, label: "市场雷达", mode: "book-radar", message: "扫描市场趋势" },
    ] as const;
    return (
        <section className={`grid min-h-[calc(100vh-11rem)] gap-4 ${activeProjectId ? "xl:grid-cols-[248px_minmax(0,1fr)_264px]" : "xl:grid-cols-[248px_minmax(0,1fr)]"}`}>
            <aside className="flex flex-col gap-4 rounded-xl border border-border/60 bg-surface p-3" aria-label="小说创作导航">
                <div>
                    <p className="px-1 text-xs font-semibold text-muted-foreground">开始创作</p>
                    <nav className="mt-1.5 grid gap-0.5">
                        {creationModes.map((mode) => (
                            <button
                                key={mode.id}
                                type="button"
                                className={`rounded-lg px-3 py-2 text-left text-sm transition-colors ${selectedSkillId === mode.id ? "bg-primary/10 font-medium text-foreground" : "text-foreground/65 hover:bg-surface-hover"}`}
                                onClick={() => { setSelectedSkillId(mode.id); startNewSession(); }}
                                aria-pressed={selectedSkillId === mode.id}
                            >
                                {mode.label}
                            </button>
                        ))}
                    </nav>
                </div>
                <div className="border-t border-border/60 pt-3">
                    <div className="flex items-center justify-between px-1">
                        <p className="text-xs font-semibold text-muted-foreground">我的创作</p>
                        <Button type="text" size="small" className="!px-1.5" icon={<Plus className="size-3.5" />} aria-label="新建小说" onClick={onCreateProject} />
                    </div>
                    <div className="mt-1.5 grid gap-0.5">
                        {novels.slice(0, 8).map((item) => (
                            <button
                                key={item.project.id}
                                type="button"
                                className="flex items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-sm text-foreground/70 transition-colors hover:bg-surface-hover"
                                onClick={() => onOpenProject(item)}
                            >
                                <span className="truncate">{item.project.name}</span>
                                <span className="shrink-0 text-xs text-muted-foreground/70">{item.unitCount} 章</span>
                            </button>
                        ))}
                        {!novels.length ? <p className="px-3 py-2 text-xs leading-5 text-muted-foreground/60">还没有书。先和 Agent 聊一个想法，确认后自动建书。</p> : null}
                    </div>
                </div>
                <details className="border-t border-border/60 pt-3">
                    <summary className="flex cursor-pointer items-center justify-between px-1 text-xs font-semibold text-muted-foreground">
                        <span>任务历史</span>
                        <span className="text-[11px] font-normal text-muted-foreground/60">{(jobHistory.data?.jobs || []).length} 条</span>
                    </summary>
                    <div className="mt-1.5 grid gap-0.5">
                        {(jobHistory.data?.jobs || []).slice(0, 8).map((job) => (
                            <div key={job.id} className="flex items-center justify-between gap-2 rounded-lg px-3 py-1.5 text-left text-xs">
                                <span className="flex min-w-0 items-center gap-1.5">
                                    {job.status === "running" || job.status === "queued" ? <Loader2 className="size-3 shrink-0 animate-spin text-primary" /> : job.status === "succeeded" ? <CheckCircle2 className="size-3 shrink-0 text-emerald-500" /> : <XCircle className="size-3 shrink-0 text-destructive" />}
                                    <span className="truncate text-foreground/65">{job.confirmedIntent || job.mode || "任务"} · {job.stage || ""}</span>
                                </span>
                                <span className="shrink-0 text-[11px] text-muted-foreground/60">{formatRelativeTime(job.createdAt)}</span>
                            </div>
                        ))}
                        {!(jobHistory.data?.jobs || []).length ? <p className="px-3 py-1.5 text-xs text-muted-foreground/60">还没有任务记录。</p> : null}
                    </div>
                </details>
                <div className="flex-1 border-t border-border/60 pt-3">
                    <div className="flex items-center justify-between px-1">
                        <p className="text-xs font-semibold text-muted-foreground">会话记录</p>
                        <Button type="text" size="small" className="!px-1.5" onClick={startNewSession}>新建会话</Button>
                    </div>
                    <Input size="small" className="mt-1.5" placeholder="搜索会话…" value={sessionSearch} onChange={(event) => setSessionSearch(event.target.value)} allowClear aria-label="搜索会话" />
                    <div className="mt-1.5 grid gap-0.5">
                        {sessionList.slice(0, 12).map((item) => (
                            <div key={item.id} className="group/session flex items-center gap-0.5">
                                <button
                                    type="button"
                                    className={`flex min-w-0 flex-1 items-center justify-between gap-2 rounded-lg px-3 py-2 text-left text-xs transition-colors ${sessionId === item.id ? "bg-primary/10 text-foreground" : "text-foreground/65 hover:bg-surface-hover"}`}
                                    onClick={() => openSession(item.id)}
                                >
                                    <span className="truncate">{item.title || "未命名会话"}</span>
                                    <span className="shrink-0 text-[11px] text-muted-foreground/60">{formatRelativeTime(item.updatedAt)}</span>
                                </button>
                                <span className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover/session:opacity-100">
                                    <button type="button" title="重命名" className="rounded p-1 text-muted-foreground hover:bg-surface-hover hover:text-foreground" onClick={() => { const title = window.prompt("新的会话名称", item.title || ""); if (title && title.trim()) renameSession.mutate({ sessionId: item.id, title: title.trim() }); }}>
                                        <Pencil className="size-3" />
                                    </button>
                                    <button type="button" title="删除会话" className="rounded p-1 text-muted-foreground hover:bg-destructive/10 hover:text-destructive" onClick={() => { if (window.confirm("删除该会话？聊天记录将一并删除。")) deleteSession.mutate(item.id); }}>
                                        <Trash2 className="size-3" />
                                    </button>
                                </span>
                            </div>
                        ))}
                        {!sessionList.length ? <p className="px-3 py-2 text-xs text-muted-foreground/60">还没有会话记录。</p> : null}
                    </div>
                </div>
            </aside>
            <div className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-border/60 bg-surface">
                <div className="flex items-center justify-between gap-3 border-b border-border/60 px-5 py-3.5">
                    <div>
                        <h2 className="text-[15px] font-semibold">Novel Agent</h2>
                        <p className="mt-0.5 text-xs text-muted-foreground">InkOS Core · Skill 驱动 · Vergestar 受控执行</p>
                    </div>
                    <Tag className="!m-0 !rounded-md" color="blue">{selectedSkillId || "通用创作"}</Tag>
                </div>
                <div className="min-h-0 flex-1 overflow-y-auto px-5 py-6 [scrollbar-gutter:stable]">
                    <div className="mx-auto flex w-full max-w-3xl flex-col gap-4">
                        {messages.map((item, index) => (
                            <div key={`${item.role}-${index}`} className="flex flex-col gap-2">
                                {item.role === "assistant" ? (() => {
                                    const parsed = parsePlaySuggestions(item.text);
                                    return <AgentBubble role="assistant">{parsed.body}</AgentBubble>;
                                })() : <AgentBubble role="user">{item.text}</AgentBubble>}
                                {item.role === "assistant" && parsePlaySuggestions(item.text).actions.length ? (
                                    <div className="flex max-w-[95%] flex-wrap gap-1.5" aria-label="建议行动">
                                        {parsePlaySuggestions(item.text).actions.map((action) => (
                                            <button
                                                key={action}
                                                type="button"
                                                disabled={turn.isPending || Boolean(jobProgress)}
                                                className="rounded-lg border border-primary/30 bg-primary/5 px-3 py-1.5 text-xs text-primary transition-all hover:bg-primary/10 disabled:opacity-40"
                                                onClick={() => {
                                                    setMessages((items) => [...items, { role: "user", text: action }]);
                                                    turn.mutate({ message: action });
                                                }}
                                            >
                                                {action}
                                            </button>
                                        ))}
                                    </div>
                                ) : null}
                                {item.skills?.length ? (
                                    <div className="flex max-w-[95%] flex-wrap gap-1">
                                        {item.skills.map((skill) => (
                                            <Tag key={skill.id} className="!m-0 !rounded-md !text-[11px]">{skill.name}</Tag>
                                        ))}
                                    </div>
                                ) : null}
                                {item.role === "assistant" && item.storyProjectId ? (
                                    <div className="flex max-w-[95%] flex-wrap gap-2">
                                        <Button type="primary" size="small" onClick={() => {
                                            const summary = novels.find((candidate) => candidate.project.id === item.storyProjectId);
                                            if (summary) onOpenProject(summary);
                                            else void queryClient.invalidateQueries({ queryKey: ["projects"] });
                                        }}>打开小说工作台</Button>
                                        <Button size="small" onClick={() => void queryClient.invalidateQueries({ queryKey: ["projects"] })}>刷新书架</Button>
                                    </div>
                                ) : null}
                                {item.confirmation ? (
                                    <AgentConfirmationCard
                                        confirmation={item.confirmation}
                                        pending={turn.isPending || Boolean(jobProgress)}
                                        onConfirm={() => {
                                            setMessages((current) => current.map((candidate, candidateIndex) => (candidateIndex === index ? { ...candidate, confirmation: undefined } : candidate)));
                                            setLastRequestedChapters(Number(item.confirmation!.actionPayload?.writeNext?.chapterCount ?? 1)); turn.mutate({ message: item.confirmation!.instruction, confirmation: item.confirmation, bookId: item.bookId });
                                        }}
                                        onCancel={() => {
                                            setMessages((current) => current.map((candidate, candidateIndex) => (candidateIndex === index ? { ...candidate, confirmation: undefined } : candidate)));
                                        }}
                                    />
                                ) : null}
                            </div>
                        ))}
                        {streamText ? (
                            <div className="flex flex-col gap-1">
                                <AgentBubble role="assistant">{streamText}</AgentBubble>
                                <p className="flex items-center gap-2 text-xs text-muted-foreground" role="status" aria-live="polite">
                                    {streamTool ? (<>正在调用工具：<span className="font-medium text-foreground/70">{streamTool}</span>…</>) : (<>正在生成<span className="inline-block size-1 animate-pulse rounded-full bg-primary" /></>)}
                                </p>
                            </div>
                        ) : null}
                        {jobProgress ? (
                            <AgentTaskCard job={jobProgress} title="生产任务" targetChapters={lastRequestedChapters} onCancel={(jobId) => cancelJob.mutate(jobId)} />
                        ) : lastJob ? (
                            <AgentTaskCard job={lastJob} title="生产任务" targetChapters={lastRequestedChapters} />
                        ) : turn.isPending ? (
                            <div className="flex max-w-[90%] items-center gap-2 text-sm text-muted-foreground" role="status" aria-live="polite">
                                <span className="inline-block size-1.5 animate-pulse rounded-full bg-primary" />
                                Novel Agent 正在组织创作方案…
                            </div>
                        ) : null}
                    </div>
                </div>
                {bookFoundation.data?.inkosBookId ? (
                    <div className="flex gap-2 overflow-x-auto border-t border-border/60 px-4 py-2" aria-label="书籍快捷操作">
                        {bookCommandChips.map((chip) => (
                            <button
                                key={chip.key}
                                type="button"
                                disabled={turn.isPending || runBookCommand.isPending}
                                className="flex shrink-0 items-center gap-1.5 rounded-lg border border-border/40 bg-surface-active/50 px-3 py-1.5 text-xs font-medium text-muted-foreground transition-all hover:border-primary/30 hover:bg-primary/5 hover:text-primary disabled:opacity-40"
                                onClick={() => {
                                    setMessages((items) => [...items, { role: "user", text: chip.label }]);
                                    runBookCommand.mutate({ mode: chip.mode, message: chip.message });
                                }}
                            >
                                {chip.icon}
                                {chip.label}
                            </button>
                        ))}
                    </div>
                ) : null}
                <div className="border-t border-border/60 p-3.5">
                    <div className="mx-auto max-w-3xl">
                        <div className="rounded-xl border border-border/60 bg-background/60 p-3 transition-colors focus-within:border-primary/40">
                            <Input.TextArea
                                value={draft}
                                rows={2}
                                maxLength={12000}
                                variant="borderless"
                                onChange={(event) => setDraft(event.target.value)}
                                onPressEnter={(event) => {
                                    if (!event.shiftKey) {
                                        event.preventDefault();
                                        submit();
                                    }
                                }}
                                placeholder="告诉我你想写什么——题材、世界观、主角、核心冲突"
                                aria-label="Novel Agent 输入"
                                className="!resize-none !px-0 !text-[15px]"
                            />
                            <div className="mt-1 flex items-center justify-between gap-3 border-t border-border/40 pt-2 text-xs text-muted-foreground">
                                <span className="truncate">当前 Skill：{skillCatalog.data?.skills.find((skill) => skill.id === selectedSkillId)?.name || selectedSkillId}</span>
                                <div className="flex items-center gap-2">
                                    <ModelPicker
                                        config={config}
                                        value={config.textModel || config.model}
                                        capability="text"
                                        variant="creation"
                                        showSelectedPrice={false}
                                        placeholder="选择模型"
                                        onChange={(value) => {
                                            updateConfig("textModel", value);
                                            updateConfig("model", value);
                                        }}
                                    />
                                    <Button type="primary" size="small" icon={<Send className="size-3.5" />} loading={turn.isPending} disabled={!draft.trim()} onClick={submit}>
                                        发送
                                    </Button>
                                </div>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
            {activeProjectId ? (
                <aside className="hidden xl:flex flex-col gap-3 overflow-y-auto rounded-xl border border-border/60 bg-surface p-3" aria-label="书籍真相面板">
                    <div className="rounded-xl bg-surface-card/60 p-1">
                        <div className="flex items-center justify-between px-3 py-2.5">
                            <span className="font-serif text-base font-medium">章节</span>
                            <span className="text-xs text-muted-foreground/70">{bookUnits.length}</span>
                        </div>
                        <div className="px-3 pb-3">
                            <ul className="max-h-44 space-y-0.5 overflow-y-auto">
                                {bookUnits.map((unit, index) => (
                                    <li key={unit.id} className="flex items-center gap-2 rounded px-1 py-1 text-sm leading-6 text-muted-foreground">
                                        <span className={`shrink-0 text-[13px] ${unit.status === "ready" ? "text-emerald-600 dark:text-emerald-400" : "text-muted-foreground"}`}>{unit.status === "ready" ? "✓" : "○"}</span>
                                        <span className="truncate flex-1">{String(index + 1).padStart(2, "0")} {unit.title}</span>
                                        <span className="shrink-0 text-[13px] tabular-nums text-muted-foreground/50">{(unit.wordCount || 0).toLocaleString()}</span>
                                    </li>
                                ))}
                                {!bookUnits.length ? <p className="py-2 text-sm italic text-muted-foreground/50">暂无章节</p> : null}
                            </ul>
                        </div>
                    </div>
                    <div className="rounded-xl bg-surface-card/60 p-1">
                        <div className="flex items-center justify-between px-3 py-2.5">
                            <span className="font-serif text-base font-medium">角色</span>
                            <span className="text-xs text-muted-foreground/70">{bookCharacters.length}</span>
                        </div>
                        <div className="px-3 pb-3">
                            <div className="grid gap-0.5">
                                {bookCharacters.slice(0, 9).map((asset) => (
                                    <div key={asset.id} className="flex items-center justify-between rounded px-1 py-1 text-sm leading-6 text-muted-foreground">
                                        <span className="truncate">{asset.title}</span>
                                        <span className={`shrink-0 text-[11px] ${asset.character?.definition?.roleCategory === "主要角色" ? "text-amber-500" : "text-muted-foreground/50"}`}>{asset.character?.definition?.roleCategory === "主要角色" ? "主要" : "次要"}</span>
                                    </div>
                                ))}
                                {!bookCharacters.length ? <p className="py-2 text-sm italic text-muted-foreground/50">角色会随建书进入这里。</p> : null}
                            </div>
                        </div>
                    </div>
                    <div className="rounded-xl bg-surface-card/60 p-1">
                        <div className="flex items-center justify-between px-3 py-2.5">
                            <span className="font-serif text-base font-medium">核心文件</span>
                            <span className="text-xs text-muted-foreground/70">{bookCoreFiles.length}</span>
                        </div>
                        <div className="px-3 pb-3">
                            <div className="grid gap-0.5">
                                {bookCoreFiles.slice(0, 10).map((path) => (
                                    <p key={path} className="truncate px-1 py-0.5 text-[13px] leading-6 text-muted-foreground/80">{path.split("/").at(-1)?.replace(/\.md$/, "")}</p>
                                ))}
                                {!bookCoreFiles.length ? <p className="py-2 text-sm italic text-muted-foreground/50">正在读取故事真相文件。</p> : null}
                            </div>
                        </div>
                    </div>
                    {activeNovel ? (
                        <Button className="w-full" type="primary" icon={<ArrowRight className="size-4" />} onClick={() => onOpenProject(activeNovel)}>打开小说工作台</Button>
                    ) : null}
                </aside>
            ) : null}
        </section>
    );
}

function NovelCard({ summary, onOpen }: { summary: ProjectSummary; onOpen: () => void }) {
    const meta = novelMetadata(summary.project.description);
    return (
        <article className="rounded-lg border border-border bg-surface p-4 shadow-sm">
            <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                    <h2 className="truncate text-base font-semibold">{summary.project.name}</h2>
                    <p className="mt-1 line-clamp-2 text-xs leading-5 text-foreground/58">{meta?.idea || "尚未填写创作想法"}</p>
                </div>
                <BookOpenText className="size-5 shrink-0 text-primary" />
            </div>
            <div className="mt-4 flex items-center justify-between">
                <Tag>{meta?.genre || "小说"}</Tag>
                <span className="text-xs text-foreground/45">{summary.unitCount} 章</span>
            </div>
            <Button className="mt-4 w-full" onClick={onOpen}>
                进入创作
            </Button>
        </article>
    );
}

function NovelProjectPanel({ projectId, onBack, onOpenDrama }: { projectId: string; onBack: () => void; onOpenDrama: () => void }) {
    const navigate = useNavigate();
    const queryClient = useQueryClient();
    const { message } = App.useApp();
    const config = useEffectiveConfig();
    const detail = useQuery({ queryKey: ["project", projectId, "novel-detail"], queryFn: () => getProject(projectId) });
    const foundation = useQuery({
        queryKey: ["story", projectId, "foundation"],
        queryFn: () => getStoryFoundation(projectId),
        enabled: detail.data?.project.sourceType === "inkos-agent",
    });
    const [selectedUnitId, setSelectedUnitId] = useState("");
    const [chapterTitle, setChapterTitle] = useState("");
    const [chapterText, setChapterText] = useState("");
    const [reviewSummary, setReviewSummary] = useState("");
    const [memoryText, setMemoryText] = useState("");
    const [branchTitle, setBranchTitle] = useState("");
    const [branchPlan, setBranchPlan] = useState("");
    const [sceneTitle, setSceneTitle] = useState("");
    const [sceneAction, setSceneAction] = useState("");
    const [characterName, setCharacterName] = useState("");
    const [studioView, setStudioView] = useState<"write" | "review" | "adapt" | "story">("write");
    const [isEditingChapter, setIsEditingChapter] = useState(false);
    const refresh = () => {
        void queryClient.invalidateQueries({ queryKey: ["project", projectId] });
        void queryClient.invalidateQueries({ queryKey: ["projects"] });
    };
    const createChapter = useMutation({
        mutationFn: () => createProjectUnit(projectId, { kind: "chapter", title: `第 ${(detail.data?.units.length || 0) + 1} 章`, sourceText: "" }),
        onSuccess: ({ unit }) => {
            refresh();
            setSelectedUnitId(unit.id);
            setChapterTitle(unit.title);
            setChapterText(unit.sourceText);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "章节创建失败"),
    });
    const saveChapter = useMutation({
        mutationFn: () => updateProjectUnit(projectId, selectedUnitId, { title: chapterTitle, sourceText: chapterText, status: "draft" }),
        onSuccess: ({ unit }) => {
            refresh();
            setChapterTitle(unit.title);
            setChapterText(unit.sourceText);
            message.success("章节已保存");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "章节保存失败"),
    });
    const project = detail.data?.project;
    const units = detail.data?.units || [];
    const activeUnit = units.find((unit) => unit.id === selectedUnitId) || units[0];
    const versions = useQuery({ queryKey: ["story", projectId, activeUnit?.id, "versions"], queryFn: () => listStoryChapterVersions(projectId, activeUnit!.id), enabled: Boolean(activeUnit) });
    const reviews = useQuery({ queryKey: ["story", projectId, activeUnit?.id, "reviews"], queryFn: () => listStoryReviews(projectId, activeUnit!.id), enabled: Boolean(activeUnit) });
    const memories = useQuery({ queryKey: ["story", projectId, "memories"], queryFn: () => listStoryMemories(projectId) });
    const branches = useQuery({ queryKey: ["story", projectId, "branches"], queryFn: () => listStoryBranches(projectId) });
    const scenes = useQuery({ queryKey: ["story", projectId, activeUnit?.id, "scenes"], queryFn: () => listStoryScenes(projectId, activeUnit!.id), enabled: Boolean(activeUnit) });
    const refreshStory = () => {
        void queryClient.invalidateQueries({ queryKey: ["story", projectId] });
        refresh();
    };
    const snapshot = useMutation({
        mutationFn: () => snapshotStoryChapter(projectId, activeUnit!.id),
        onSuccess: () => {
            refreshStory();
            message.success("已创建章节版本快照");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "版本快照失败"),
    });
    const restoreVersion = useMutation({
        mutationFn: (versionId: string) => restoreStoryChapterVersion(projectId, activeUnit!.id, versionId),
        onSuccess: ({ unit }) => {
            setChapterTitle(unit.title);
            setChapterText(unit.sourceText);
            refreshStory();
            message.success("章节已恢复到选定版本");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "版本恢复失败"),
    });
    const review = useMutation({
        mutationFn: () => createStoryReview(projectId, activeUnit!.id, { summary: reviewSummary, issues: [] }),
        onSuccess: () => {
            setReviewSummary("");
            refreshStory();
            message.success("审稿记录已保存");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "审稿记录保存失败"),
    });
    const memory = useMutation({
        mutationFn: () => createStoryMemory(projectId, { kind: "fact", content: memoryText }),
        onSuccess: () => {
            setMemoryText("");
            refreshStory();
            message.success("故事记忆已保存");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "故事记忆保存失败"),
    });
    const branch = useMutation({
        mutationFn: () => createStoryBranch(projectId, activeUnit!.id, { title: branchTitle, plan: { plan: branchPlan } }),
        onSuccess: () => {
            setBranchTitle("");
            setBranchPlan("");
            refreshStory();
            message.success("未来分支已创建，不会改变当前正史");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "创建分支失败"),
    });
    const scene = useMutation({
        mutationFn: () => createStoryScene(projectId, activeUnit!.id, { title: sceneTitle, action: sceneAction }),
        onSuccess: () => {
            setSceneTitle("");
            setSceneAction("");
            refreshStory();
            message.success("剧本场次已创建");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "场次创建失败"),
    });
    const sceneShot = useMutation({
        mutationFn: (sceneId: string) => createStorySceneShot(projectId, activeUnit!.id, sceneId, {}),
        onSuccess: () => {
            refreshStory();
            message.success("场次已生成镜头，可进入分镜制作继续完善");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "镜头创建失败"),
    });
    const character = useMutation({
        mutationFn: () => createProjectCharacter(projectId, { name: characterName, definition: {} }),
        onSuccess: () => {
            setCharacterName("");
            refreshStory();
            message.success("角色资产已创建");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "角色资产创建失败"),
    });
    const confirmScriptCharacterCandidate = useMutation({
        mutationFn: (candidateId: string) => confirmProjectAssetCandidate(projectId, candidateId),
        onSuccess: ({ asset }) => {
            refreshStory();
            message.success(`${asset.title} 已创建为角色资产，并已回填当前影视场次`);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "角色候选确认失败"),
    });
    const [reviewMode, setReviewMode] = useState<"auto" | "manual" | null>(null);
    const [settingsOpen, setSettingsOpen] = useState(false);
    const [settingsWordCount, setSettingsWordCount] = useState<number | null>(null);
    const [settingsTarget, setSettingsTarget] = useState<number | null>(null);
    const [settingsStatus, setSettingsStatus] = useState("active");
    const effectiveReviewMode = reviewMode ?? "auto";
    const toggleReviewMode = useMutation({
        mutationFn: async () => {
            const next = effectiveReviewMode === "manual" ? "auto" : "manual";
            await updateInkosBookSettings(projectId, { chapterReviewMode: next });
            return next;
        },
        onSuccess: (next) => {
            setReviewMode(next);
            message.success(next === "manual" ? "审查模式：手动·写完即停" : "审查模式：自动审校重写");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "审查模式切换失败"),
    });
    const approveChapter = useMutation({
        mutationFn: (chapterNumber: number) => approveInkosChapter(projectId, chapterNumber),
        onSuccess: (_data, chapterNumber) => {
            refreshStory();
            message.success(`第 ${chapterNumber} 章已通过并进入正史`);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "章节通过失败"),
    });
    const rejectChapter = useMutation({
        mutationFn: (chapterNumber: number) => rejectInkosChapter(projectId, chapterNumber),
        onSuccess: (_data, chapterNumber) => {
            refreshStory();
            message.success(`第 ${chapterNumber} 章已拒绝，回滚到第 ${chapterNumber - 1} 章状态`);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "章节拒绝失败"),
    });
    const saveBookSettings = useMutation({
        mutationFn: () => updateInkosBookSettings(projectId, {
            ...(settingsWordCount ? { chapterWordCount: settingsWordCount } : {}),
            ...(settingsTarget ? { targetChapters: settingsTarget } : {}),
            ...(settingsStatus ? { status: settingsStatus } : {}),
        }),
        onSuccess: () => {
            setSettingsOpen(false);
            message.success("书籍设置已保存");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "书籍设置保存失败"),
    });
    const removeBook = useMutation({
        mutationFn: () => deleteInkosBook(projectId),
        onSuccess: () => {
            message.success("小说已删除");
            onBack();
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "删除小说失败"),
    });
    useEffect(() => {
        if (!activeUnit) return;
        setSelectedUnitId(activeUnit.id);
        setChapterTitle(activeUnit.title);
        setChapterText(activeUnit.sourceText);
    }, [activeUnit?.id]);
    if (detail.isLoading) return <WorkspaceLoadingState label="正在打开小说" detail="读取故事与章节" />;
    if (detail.isError || !project) return <WorkspaceErrorState title="小说项目加载失败" onRetry={() => void detail.refetch()} />;
    const meta = novelMetadata(project.description);
    const latestVersion = versions.data?.versions.at(-1);
    const chapterContent = chapterText.trim() || latestVersion?.content || activeUnit?.sourceText || "";
    const latestReview = reviews.data?.reviews.at(-1);
    const characterAssets = (detail.data?.assets || []).filter((asset) => asset.category === "character");
    const coreFiles = Object.keys(foundation.data?.files || {}).filter((path) => path.startsWith("story/") || path.startsWith("chapters/") || path.startsWith("shorts/")).slice(0, 8);
    const totalWords = units.reduce((sum, unit) => sum + (unit.wordCount || 0), 0);
    // 短篇项目（shorts/ 产物合成）没有章节语义，隐藏 写下一章/仅草稿/新建章节/通过/拒绝。
    const isShortProject = Object.keys(foundation.data?.files || {}).some((path) => path.startsWith("shorts/"));
    const chapterStatusSymbol = (status: string) => status === "ready"
        ? { symbol: "✓", className: "text-emerald-600 dark:text-emerald-400" }
        : status === "draft" ? { symbol: "○", className: "text-muted-foreground" }
        : { symbol: "◆", className: "text-amber-500" };
    return (
        <section className="mt-5 space-y-6">
            <nav className="flex items-center gap-2 text-[13px] font-medium text-muted-foreground" aria-label="面包屑">
                <button type="button" className="transition-colors hover:text-foreground" onClick={onBack}>我的创作</button>
                <span className="text-border">/</span>
                <span className="text-foreground">{project.name}</span>
            </nav>
            <div className="flex flex-col justify-between gap-5 border-b border-border/50 pb-6 md:flex-row md:items-end">
                <div className="space-y-2">
                    <h2 className="font-serif text-3xl font-medium md:text-4xl">{project.name}</h2>
                    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm text-muted-foreground">
                        <span className="rounded bg-surface-active px-2 py-0.5 text-xs uppercase tracking-wider text-foreground/70">{meta?.genre || "小说"}</span>
                        <span className="flex items-center gap-1.5"><FileText className="size-3.5" />{units.length} 章</span>
                        <span className="flex items-center gap-1.5"><Zap className="size-3.5" />{totalWords.toLocaleString()} 字</span>
                    </div>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                    <button type="button" className="flex items-center gap-2 rounded-xl bg-primary px-5 py-2.5 text-sm font-bold text-primary-foreground shadow-lg shadow-primary/20 transition-all hover:scale-[1.02] active:scale-95 disabled:opacity-50" onClick={() => setStudioView("write")}>
                        <Zap className="size-4" />写下一章
                    </button>
                    <button type="button" className="flex items-center gap-2 rounded-xl border border-border/50 bg-surface-active px-4 py-2.5 text-sm font-medium text-foreground transition-all hover:bg-surface-hover" onClick={() => setStudioView("adapt")}>
                        <Film className="size-4" />剧本与分镜
                    </button>
                    <button type="button" className="flex items-center gap-2 rounded-xl border border-border/50 bg-surface-active px-4 py-2.5 text-sm font-medium text-foreground transition-all hover:bg-surface-hover" onClick={() => setStudioView("story")}>
                        <Lightbulb className="size-4" />故事资料
                    </button>
                    {foundation.data ? (
                        <>
                            <button type="button" title={effectiveReviewMode === "manual" ? "手动审查：写完即停，由你点 通过/拒绝。点此切回自动" : "自动审查：写完自动审校并按需重写。点此切到手动·写完即停"} className="flex items-center gap-2 rounded-xl border border-border/50 bg-surface-active px-4 py-2.5 text-sm font-medium text-foreground transition-all hover:bg-surface-hover disabled:opacity-50" disabled={toggleReviewMode.isPending} onClick={() => toggleReviewMode.mutate()}>
                                {effectiveReviewMode === "manual" ? <Hand className="size-4" /> : <Settings2 className="size-4" />}
                                审查：{effectiveReviewMode === "manual" ? "手动·写完即停" : "自动"}
                            </button>
                            <button type="button" className="flex items-center gap-2 rounded-xl border border-border/50 bg-surface-active px-4 py-2.5 text-sm font-medium text-foreground transition-all hover:bg-surface-hover" onClick={() => setSettingsOpen(true)}>
                                <Settings2 className="size-4" />书籍设置
                            </button>
                            <Popconfirm title="删除小说" description="会同时删除 InkOS 书籍工作区与 Vergestar 项目，正文无法恢复。确定删除？" okText="删除" cancelText="取消" okButtonProps={{ danger: true }} onConfirm={() => removeBook.mutate()}>
                                <button type="button" className="flex items-center gap-2 rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-2.5 text-sm font-bold text-destructive transition-all hover:bg-destructive hover:text-white disabled:opacity-50" disabled={removeBook.isPending}>
                                    <Trash2 className="size-4" />删除
                                </button>
                            </Popconfirm>
                        </>
                    ) : null}
                    <Button className="w-auto" icon={<ArrowRight className="size-4" />} onClick={() => navigate(`/canvas?projectId=${encodeURIComponent(projectId)}`)}>进入画布</Button>
                </div>
            </div>
            <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_264px]">
                <div className="min-w-0">
                    <div className="grid gap-3 sm:grid-cols-3">
                        <StoryStage title="故事基础" detail="创意、世界与角色" ready />
                        <StoryStage title="章节创作" detail={`${units.length} 章已建立`} ready={units.length > 0} />
                        <StoryStage title="影视转化" detail="剧本、分镜、画布" ready={Boolean(activeUnit)} />
                    </div>
                    <div className="mt-5 grid gap-4">
                        <nav className="flex flex-wrap gap-2" aria-label="创作流程">
                            {[
                                { id: "write" as const, label: "继续写作", icon: BookOpenText },
                                { id: "review" as const, label: "审稿与修订", icon: SearchCheck },
                                { id: "adapt" as const, label: "剧本与分镜", icon: Film },
                                { id: "story" as const, label: "故事资料", icon: Lightbulb },
                            ].map((item) => {
                                const Icon = item.icon;
                                const selectedView = studioView === item.id;
                                return (
                                    <button
                                        type="button"
                                        key={item.id}
                                        onClick={() => setStudioView(item.id)}
                                        className={`flex items-center gap-1.5 rounded-lg border px-4 py-2 text-xs font-bold transition-all ${selectedView ? "border-primary/40 bg-primary/10 text-foreground" : "border-border/50 bg-surface-active/50 text-muted-foreground hover:bg-surface-active hover:text-foreground"}`}
                                        aria-pressed={selectedView}
                                    >
                                        <Icon className="size-3.5" />{item.label}
                                    </button>
                                );
                            })}
                            <Button size="small" type="text" icon={<ArrowRight className="size-3.5" />} onClick={onOpenDrama}>打开章节工作台</Button>
                        </nav>
                        {activeUnit ? (
                            <>
                                {studioView === "write" ? <>
                                    <p className="text-xs font-medium text-primary">下一步</p>
                                    <h3 className="mt-1 text-lg font-semibold">继续写下一章</h3>
                                    <p className="mt-1 text-sm text-foreground/60">Writer 会读取当前故事状态，先提出计划，确认后才会写入正史。</p>
                                    {isShortProject ? <p className="mt-3 rounded-md border border-border/60 bg-surface-active px-3 py-2 text-xs text-muted-foreground">短篇项目为整篇生产，不支持逐章写作；可在「故事资料」阅读全文，或重新发起短篇生产。</p> : <NovelWriterPanel projectId={projectId} foundation={foundation.data} config={config} queryClient={queryClient} message={message} />}
                                    <div className="mt-5 border-t border-border pt-4">
                                        <div className="flex items-center justify-between gap-2"><div><p className="text-xs text-foreground/45">当前章节</p><h3 className="mt-1 text-base font-semibold">第 {activeUnit.position + 1} 章 · {activeUnit.title}</h3></div><Button size="small" onClick={() => setIsEditingChapter((value) => !value)}>{isEditingChapter ? "收起编辑" : "阅读与编辑"}</Button></div>
                                        {isEditingChapter ? <><Input className="mt-4" value={chapterTitle} onChange={(event) => setChapterTitle(event.target.value)} maxLength={240} aria-label="章节标题" /><Input.TextArea className="mt-3" rows={16} value={chapterText || chapterContent} onChange={(event) => setChapterText(event.target.value)} aria-label="章节正文" /><div className="mt-3 flex gap-2"><Button type="primary" icon={<Save className="size-4" />} loading={saveChapter.isPending} onClick={() => saveChapter.mutate()}>保存章节</Button><Button icon={<History className="size-4" />} loading={snapshot.isPending} onClick={() => snapshot.mutate()}>创建版本</Button></div></> : <p className="mt-4 max-h-52 overflow-hidden whitespace-pre-wrap text-sm leading-7 text-foreground/70">{chapterContent || "正文会在 Writer 完成后显示在这里。"}</p>}
                                    </div>
                                </> : null}
                                {studioView === "review" ? <>
                                    <p className="text-xs font-medium text-primary">质量门禁</p><h3 className="mt-1 text-lg font-semibold">先看审稿结论，再决定修订</h3><p className="mt-1 text-sm text-foreground/60">修订会重新审计并保留版本，不会静默覆盖正文。</p>
                                    {latestReview ? <p className="mt-5 whitespace-pre-wrap border-l-2 border-primary/50 pl-3 text-sm leading-6 text-foreground/70">{latestReview.summary}</p> : <p className="mt-5 text-sm text-foreground/55">当前章节没有待处理的审稿问题。</p>}
                                    <NovelChapterRevisePanel projectId={projectId} unit={activeUnit} foundation={foundation.data} config={config} queryClient={queryClient} message={message} reviewIssues={latestReview?.summary || ""} />
                                </> : null}
                                {studioView === "adapt" ? <>
                                    <p className="text-xs font-medium text-primary">影视改编</p><h3 className="mt-1 text-lg font-semibold">先生成剧本，再生成分镜，最后确认媒体任务</h3><p className="mt-1 text-sm text-foreground/60">角色和场次会自动带入下游，不需要重新整理素材。</p>
                                    <NovelChapterProductionPanel projectId={projectId} unit={activeUnit} foundation={foundation.data} config={config} queryClient={queryClient} message={message} />
                                    <NovelScriptCharacterCandidatesPanel candidates={(detail.data?.assetCandidates || []).filter((candidate) => candidate.unitId === activeUnit.id)} confirmingId={confirmScriptCharacterCandidate.isPending ? confirmScriptCharacterCandidate.variables || "" : ""} onConfirm={(candidateId) => confirmScriptCharacterCandidate.mutate(candidateId)} />
                                    <NovelShotGenerationPanel projectId={projectId} unitId={activeUnit.id} detail={detail.data!} scenes={scenes.data?.scenes || []} onRefresh={refresh} />
                                </> : null}
                                {studioView === "story" ? <>
                                    <p className="text-xs font-medium text-primary">故事资料</p><h3 className="mt-1 text-lg font-semibold">故事的长期记忆与可回溯资料</h3><p className="mt-1 text-sm text-foreground/60">这里保存核心文件、记忆、分支和手动补充的场次，不打断日常写作。</p>
                                    <StoryFoundationPanel projectId={projectId} foundation={foundation.data} isLoading={foundation.isLoading} queryClient={queryClient} message={message} config={config} />
                                    <details className="mt-5 border-t border-border pt-4"><summary className="cursor-pointer text-sm font-medium">管理记忆、分支与手动场次</summary><StoryToolsPanel versions={versions.data?.versions || []} reviews={reviews.data?.reviews.slice(0, 1) || []} memories={memories.data?.memories || []} branches={branches.data?.branches || []} scenes={scenes.data?.scenes || []} reviewSummary={reviewSummary} memoryText={memoryText} branchTitle={branchTitle} branchPlan={branchPlan} sceneTitle={sceneTitle} sceneAction={sceneAction} characterName={characterName} onReviewSummary={setReviewSummary} onMemoryText={setMemoryText} onBranchTitle={setBranchTitle} onBranchPlan={setBranchPlan} onSceneTitle={setSceneTitle} onSceneAction={setSceneAction} onCharacterName={setCharacterName} onSaveReview={() => review.mutate()} onSaveMemory={() => memory.mutate()} onCreateBranch={() => branch.mutate()} onCreateScene={() => scene.mutate()} onCreateCharacter={() => character.mutate()} onCreateSceneShot={(sceneId) => sceneShot.mutate(sceneId)} onRestore={(versionId) => restoreVersion.mutate(versionId)} /></details>
                                </> : null}
                                {studioView === "review" && activeUnit.status === "draft" && reviews.data?.reviews.some((item) => item.status === "failed") ? <NovelStateRepairPanel projectId={projectId} unit={activeUnit} foundation={foundation.data} config={config} queryClient={queryClient} message={message} /> : null}
                            </>
                        ) : studioView === "story" ? (
                            <>
                                <p className="text-xs font-medium text-primary">故事资料</p><h3 className="mt-1 text-lg font-semibold">故事的长期记忆与可回溯资料</h3>
                                <StoryFoundationPanel projectId={projectId} foundation={foundation.data} isLoading={foundation.isLoading} queryClient={queryClient} message={message} config={config} />
                            </>
                        ) : (
                            <>
                                <WorkspaceState
                                    compact
                                    title="还没有章节"
                                    description="与 InkOS 一致：可以直接仅草稿写第一章，或让 Agent 先规划。"
                                />
                                {isShortProject ? <p className="mt-3 rounded-md border border-border/60 bg-surface-active px-3 py-2 text-xs text-muted-foreground">短篇项目为整篇生产，不支持逐章写作；可在「故事资料」阅读全文，或重新发起短篇生产。</p> : <NovelWriterPanel projectId={projectId} foundation={foundation.data} config={config} queryClient={queryClient} message={message} />}
                            </>
                        )}
                    </div>
                </div>
                <aside className="space-y-3" aria-label="小说状态">
                    <div className="rounded-xl bg-surface-card/60 p-1">
                        <div className="flex items-center justify-between px-3 py-2.5">
                            <span className="font-serif text-base font-medium">章节</span>
                            <Button type="text" size="small" icon={<Plus className="size-4" />} loading={createChapter.isPending} disabled={isShortProject} onClick={() => createChapter.mutate()} aria-label="新建章节" />
                        </div>
                        <div className="px-3 pb-3">
                            <ul className="max-h-56 space-y-0.5 overflow-y-auto overflow-x-hidden">
                                {units.map((unit, index) => {
                                    const indicator = chapterStatusSymbol(unit.status);
                                    return (
                                        <li key={unit.id}>
                                            <button type="button" className={`group/chapter flex w-full items-center gap-2 rounded px-1 py-1 text-left text-[15px] leading-6 transition-colors hover:bg-surface-hover ${activeUnit?.id === unit.id ? "text-foreground" : "text-muted-foreground"}`} onClick={() => { setSelectedUnitId(unit.id); setChapterTitle(unit.title); setChapterText(unit.sourceText); }}>
                                                <span className={`shrink-0 text-[13px] ${indicator.className}`}>{indicator.symbol}</span>
                                                <span className="truncate flex-1">{String(index + 1).padStart(2, "0")} {unit.title}</span>
                                                <span className="shrink-0 text-[13px] tabular-nums text-muted-foreground/50">{(unit.wordCount || 0).toLocaleString()}</span>
                                                {unit.status !== "ready" || true ? (
                                                    <span className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-hover/chapter:opacity-100">
                                                        <button type="button" title="通过：进入正史" className="rounded p-0.5 text-emerald-600 hover:bg-emerald-500/10" disabled={approveChapter.isPending} onClick={(event) => { event.stopPropagation(); approveChapter.mutate(index + 1); }}><Check className="size-3.5" /></button>
                                                        <button type="button" title="拒绝：回滚并丢弃本章" className="rounded p-0.5 text-destructive hover:bg-destructive/10" disabled={rejectChapter.isPending} onClick={(event) => { event.stopPropagation(); rejectChapter.mutate(index + 1); }}><X className="size-3.5" /></button>
                                                    </span>
                                                ) : null}
                                            </button>
                                        </li>
                                    );
                                })}
                                {!units.length ? <p className="py-3 text-sm italic text-muted-foreground/50">暂无章节</p> : null}
                            </ul>
                        </div>
                    </div>
                    <div className="rounded-xl bg-surface-card/60 p-1">
                        <div className="px-3 py-2.5"><span className="font-serif text-base font-medium">角色</span></div>
                        <div className="px-3 pb-3">
                            <div className="grid gap-0.5">
                                {characterAssets.slice(0, 8).map((asset) => (
                                    <div key={asset.id} className="flex items-center justify-between rounded px-1 py-1 text-[15px] leading-6 text-muted-foreground">
                                        <span className="truncate">{asset.title}</span>
                                        <span className={`shrink-0 text-[11px] ${asset.character?.definition?.roleCategory === "主要角色" ? "text-amber-500" : "text-muted-foreground/50"}`}>{asset.character?.definition?.roleCategory === "主要角色" ? "主要" : "配角"}</span>
                                    </div>
                                ))}
                                {!characterAssets.length ? <p className="py-2 text-sm italic text-muted-foreground/50">角色会随 InkOS 建书和剧本确认进入这里。</p> : null}
                            </div>
                        </div>
                    </div>
                    <div className="rounded-xl bg-surface-card/60 p-1">
                        <div className="px-3 py-2.5"><span className="font-serif text-base font-medium">核心文件</span></div>
                        <div className="px-3 pb-3">
                            <div className="grid gap-0.5">
                                {coreFiles.map((path) => <p key={path} className="truncate px-1 py-1 text-[13px] leading-6 text-muted-foreground/80">{path.split("/").at(-1)?.replace(/\.md$/, "")}</p>)}
                                {!coreFiles.length ? <p className="py-2 text-sm italic text-muted-foreground/50">正在读取 InkOS 核心文件。</p> : null}
                            </div>
                            <Button className="mt-2" size="small" type="text" onClick={() => setStudioView("story")}>查看故事资料</Button>
                        </div>
                    </div>
                </aside>
            </div>
            <Modal title="书籍设置" open={settingsOpen} onCancel={() => setSettingsOpen(false)} footer={null} destroyOnHidden>
                <div className="grid gap-3">
                    <div>
                        <p className="text-xs text-muted-foreground">单章目标字数</p>
                        <InputNumber className="mt-1 w-full" min={200} max={20000} step={100} value={settingsWordCount} onChange={(value) => setSettingsWordCount(typeof value === "number" ? value : null)} placeholder="留空保持不变" />
                    </div>
                    <div>
                        <p className="text-xs text-muted-foreground">目标章节数</p>
                        <InputNumber className="mt-1 w-full" min={1} max={2000} step={1} value={settingsTarget} onChange={(value) => setSettingsTarget(typeof value === "number" ? value : null)} placeholder="留空保持不变" />
                    </div>
                    <div>
                        <p className="text-xs text-muted-foreground">书籍状态</p>
                        <Select className="mt-1 w-full" value={settingsStatus} onChange={setSettingsStatus} options={[
                            { value: "active", label: "连载中" },
                            { value: "paused", label: "暂停" },
                            { value: "outlining", label: "大纲中" },
                            { value: "completed", label: "完本" },
                            { value: "dropped", label: "弃稿" },
                        ]} />
                    </div>
                    <Button type="primary" loading={saveBookSettings.isPending} onClick={() => saveBookSettings.mutate()}>保存设置</Button>
                </div>
            </Modal>
        </section>
    );
}

function NovelChapterProductionPanel({
    projectId,
    unit,
    foundation,
    config,
    queryClient,
    message,
}: {
    projectId: string;
    unit: ProjectUnit;
    foundation?: StoryFoundationView;
    config: ReturnType<typeof useEffectiveConfig>;
    queryClient: ReturnType<typeof useQueryClient>;
    message: ReturnType<typeof App.useApp>["message"];
}) {
    const [maxShots, setMaxShots] = useState(12);
    const [job, setJob] = useState<NovelAgentJob | null>(null);
    const requestModel = () => {
        const modelValue = config.textModel || config.model;
        if (!modelValue) throw new Error("请先在设置中选择文本模型");
        const requestConfig = resolveModelRequestConfig(config, modelValue);
        if (requestConfig.apiFormat !== "openai" || requestConfig.interfaceType === "claude-api" || requestConfig.interfaceType === "gemini-image" || requestConfig.interfaceType === "gemini-veo") {
            throw new Error("影视生产需要 OpenAI 兼容文本渠道");
        }
        return {
            provider: "custom" as const,
            baseUrl: requestConfig.baseUrl,
            apiKey: requestConfig.apiKey,
            model: requestConfig.model,
            apiFormat: requestConfig.interfaceType === "openai-responses" ? "responses" as const : "chat" as const,
            temperature: 0.5,
        };
    };
    const run = useMutation({
        mutationFn: async (kind: "script" | "storyboard") => {
            if (!foundation?.agentSessionId || !foundation.inkosBookId) throw new Error("当前小说没有绑定 InkOS Agent 会话");
            const isScript = kind === "script";
            const title = unit.title;
            const instruction = isScript
                ? `把《${foundation.title}》第 ${unit.position + 1} 章改编为一集竖屏短剧剧本。保留主线冲突、人物边界和关键伏笔，人物台词短促有压迫感。`
                : `把《${foundation.title}》第 ${unit.position + 1} 章转化为 ${maxShots} 个以内竖屏短剧分镜。每个镜头包含场景、画面、动作、台词、镜头和可生成图像的 Prompt。`;
            const { job: initialJob } = await startNovelAgentJob({
                sessionId: foundation.agentSessionId,
                bookId: foundation.inkosBookId,
                mode: isScript ? "create-script" : "create-storyboard",
                message: `请对第 ${unit.position + 1} 章执行${isScript ? "剧本" : "分镜"}生产。`,
                requestedSkills: isScript ? ["inkos-script-writing"] : ["inkos-storyboard"],
                confirmedIntent: isScript ? "create_script" : "create_storyboard",
                confirmedActionPayload: {
                    unitId: unit.id,
                    title,
                    chapterNumber: unit.position + 1,
                    ...(isScript ? { scriptCreate: { title, chapterNumber: unit.position + 1, episodeCount: 1, instruction } } : { storyboardCreate: { title, chapterNumber: unit.position + 1, maxShots, aspectRatio: "9:16", instruction } }),
                },
                model: requestModel(),
            });
            setJob(initialJob);
            let current = initialJob;
            while (current.status === "queued" || current.status === "running") {
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
                current = (await getNovelAgentJob(current.id)).job;
                setJob(current);
            }
            if (current.status === "failed") throw new Error(current.error || current.stage || "影视生产任务失败");
            return current;
        },
        onSuccess: (current) => {
            setJob(null);
            void queryClient.invalidateQueries({ queryKey: ["project", projectId] });
            void queryClient.invalidateQueries({ queryKey: ["story", projectId] });
            message.success(current.result?.text || "InkOS 影视生产完成");
        },
        onError: (error) => {
            setJob(null);
            message.error(error instanceof Error ? error.message : "影视生产任务失败");
        },
    });
    const hasBinding = Boolean(foundation?.agentSessionId && foundation?.inkosBookId);
    return (
        <section className="mt-3 rounded-md border border-primary/20 bg-primary/5 p-3" aria-label="InkOS 影视生产">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                    <p className="text-sm font-medium">InkOS 影视生产</p>
                    <p className="mt-1 text-xs text-foreground/58">调用 InkOS 原生 Script / Storyboard Pipeline，先改编剧本，再生成镜头与图像 Prompt。</p>
                </div>
                <Select size="small" className="w-28" value={maxShots} onChange={setMaxShots} disabled={run.isPending}
                    options={[6, 8, 10, 12, 16, 24].map((value) => ({ value, label: `${value} 镜头` }))} />
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
                <Button type="primary" size="small" icon={<FileText className="size-4" />} loading={run.isPending && run.variables === "script"} disabled={run.isPending || !hasBinding} onClick={() => run.mutate("script")}>生成剧本</Button>
                <Button size="small" icon={<ArrowRight className="size-4" />} loading={run.isPending && run.variables === "storyboard"} disabled={run.isPending || !hasBinding} onClick={() => run.mutate("storyboard")}>生成分镜</Button>
            </div>
            {!hasBinding ? <p className="mt-2 text-xs text-warning">当前项目缺少 InkOS 会话绑定，需先由 Agent 创建或写入章节。</p> : null}
            {job ? (
                <div className="mt-3 rounded bg-background px-3 py-2 text-xs text-foreground/65" role="status" aria-live="polite">
                    <p>生产任务：{job.stage}</p>
                    {job.logs.slice(-3).map((log, index) => <p key={`${log}-${index}`} className="mt-1 text-foreground/45">{log}</p>)}
                    <NovelJobCancelButton jobId={job.id} />
                </div>
            ) : null}
        </section>
    );
}

function NovelChapterRevisePanel({
    projectId,
    unit,
    foundation,
    config,
    queryClient,
    message,
    reviewIssues,
}: {
    projectId: string;
    unit: ProjectUnit;
    foundation?: StoryFoundationView;
    config: ReturnType<typeof useEffectiveConfig>;
    queryClient: ReturnType<typeof useQueryClient>;
    message: ReturnType<typeof App.useApp>["message"];
    reviewIssues: string;
}) {
    const [mode, setMode] = useState<"spot-fix" | "polish">("spot-fix");
    const [job, setJob] = useState<NovelAgentJob | null>(null);
    const requestModel = () => {
        const modelValue = config.textModel || config.model;
        if (!modelValue) throw new Error("请先在设置中选择文本模型");
        const requestConfig = resolveModelRequestConfig(config, modelValue);
        if (requestConfig.apiFormat !== "openai" || requestConfig.interfaceType === "claude-api" || requestConfig.interfaceType === "gemini-image" || requestConfig.interfaceType === "gemini-veo") {
            throw new Error("章节修订需要 OpenAI 兼容文本渠道");
        }
        return {
            provider: "custom" as const,
            baseUrl: requestConfig.baseUrl,
            apiKey: requestConfig.apiKey,
            model: requestConfig.model,
            apiFormat: requestConfig.interfaceType === "openai-responses" ? "responses" as const : "chat" as const,
            temperature: 0.3,
        };
    };
    const revise = useMutation({
        mutationFn: async () => {
            if (!foundation?.agentSessionId || !foundation.inkosBookId) throw new Error("当前小说没有绑定 InkOS Agent 会话");
            const chapterNumber = unit.position + 1;
            const instruction = [
                `修订《》第 ${chapterNumber} 章。目标标题：${unit.title}。`,
                reviewIssues.trim() ? `以下为最新审稿问题：${reviewIssues.trim()}` : "请先执行一次 Auditor，再根据最新审稿问题修订。",
                "保留主线事实、人物边界和已有伏笔编号；优先修复警告、矛盾和时间锚点问题，不新增无效钩子。",
                `修订模式：${mode}。`,
            ].join("\n");
            const { job: initialJob } = await startNovelAgentJob({
                sessionId: foundation.agentSessionId,
                bookId: foundation.inkosBookId,
                mode: "revise-chapter",
                message: `请按审稿意见修订第 ${chapterNumber} 章。`,
                requestedSkills: ["inkos-long-writing"],
                confirmedIntent: "revise_chapter",
                confirmedActionPayload: { reviseChapter: { chapterNumber, mode, instruction } },
                model: requestModel(),
            });
            setJob(initialJob);
            let current = initialJob;
            while (current.status === "queued" || current.status === "running") {
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
                current = (await getNovelAgentJob(current.id)).job;
                setJob(current);
            }
            if (current.status === "failed") throw new Error(current.error || current.stage || "章节修订失败");
            return current;
        },
        onSuccess: (current) => {
            setJob(null);
            void queryClient.invalidateQueries({ queryKey: ["project", projectId] });
            void queryClient.invalidateQueries({ queryKey: ["story", projectId] });
            message.success(current.result?.text || `第 ${unit.position + 1} 章修订任务已完成`);
        },
        onError: (error) => {
            setJob(null);
            message.error(error instanceof Error ? error.message : "章节修订失败");
        },
    });
    return (
        <section className="mt-3 rounded-md border border-primary/20 bg-primary/5 p-3" aria-label="InkOS Reviser 章节修订">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                    <p className="text-xs font-medium">InkOS Reviser 章节修订</p>
                    <p className="mt-1 text-xs text-foreground/55">使用最新审稿问题执行修订；修订会重新审计并结算状态，通过质量门禁后才替换正文。</p>
                </div>
                <div className="flex items-center gap-2">
                    <Select
                        size="small"
                        className="w-24"
                        value={mode}
                        disabled={revise.isPending}
                        onChange={(value) => setMode(value)}
                        options={[
                            { value: "spot-fix", label: "定点修复" },
                            { value: "polish", label: "打磨润色" },
                        ]}
                    />
                    <Button size="small" type="primary" loading={revise.isPending} disabled={!reviewIssues.trim() || !foundation?.agentSessionId || !foundation?.inkosBookId} onClick={() => revise.mutate()}>
                        修订本章
                    </Button>
                </div>
            </div>
            {!reviewIssues.trim() ? <p className="mt-2 text-xs text-foreground/45">当前章节还没有审稿记录。</p> : null}
            {job ? (
                <div className="mt-2 rounded bg-background px-2 py-1 text-xs text-foreground/60" role="status" aria-live="polite">
                    <p>修订任务：{job.stage}</p>
                    {job.logs.slice(-2).map((log, index) => <p key={`${log}-${index}`} className="mt-1 text-foreground/45">{log}</p>)}
                    <NovelJobCancelButton jobId={job.id} />
                </div>
            ) : null}
        </section>
    );
}
function NovelStateRepairPanel({
    projectId,
    unit,
    foundation,
    config,
    queryClient,
    message,
}: {
    projectId: string;
    unit: ProjectUnit;
    foundation?: StoryFoundationView;
    config: ReturnType<typeof useEffectiveConfig>;
    queryClient: ReturnType<typeof useQueryClient>;
    message: ReturnType<typeof App.useApp>["message"];
}) {
    const [proposal, setProposal] = useState<NovelAgentConfirmation | null>(null);
    const [job, setJob] = useState<NovelAgentJob | null>(null);
    const requestModel = () => {
        const modelValue = config.textModel || config.model;
        if (!modelValue) throw new Error("请先在设置中选择文本模型");
        const requestConfig = resolveModelRequestConfig(config, modelValue);
        if (requestConfig.apiFormat !== "openai" || requestConfig.interfaceType === "claude-api" || requestConfig.interfaceType === "gemini-image" || requestConfig.interfaceType === "gemini-veo") throw new Error("状态修复需要 OpenAI 兼容文本渠道");
        return { provider: "custom" as const, baseUrl: requestConfig.baseUrl, apiKey: requestConfig.apiKey, model: requestConfig.model, apiFormat: requestConfig.interfaceType === "openai-responses" ? "responses" as const : "chat" as const, temperature: 0.2 };
    };
    const plan = useMutation({
        mutationFn: async () => {
            if (!foundation?.agentSessionId || !foundation.inkosBookId) throw new Error("当前小说没有绑定 InkOS Agent 会话");
            const { job: initialJob } = await startNovelAgentJob({ sessionId: foundation.agentSessionId, bookId: foundation.inkosBookId, mode: "repair-state", message: `请修复第 ${unit.position + 1} 章的状态结算和伏笔一致性，保留正文不变，不重写章节。`, requestedSkills: ["inkos-long-writing"], model: requestModel() });
            setJob(initialJob);
            let current = initialJob;
            while (current.status === "queued" || current.status === "running") { await new Promise((resolve) => window.setTimeout(resolve, 1500)); current = (await getNovelAgentJob(current.id)).job; setJob(current); }
            if (current.status === "failed") throw new Error(current.error || current.stage || "状态修复规划失败");
            if (!current.result) throw new Error("状态修复规划完成但没有返回结果");
            return current.result;
        },
        onSuccess: (result) => { setJob(null); if (result.confirmation?.action === "repair_state") setProposal(result.confirmation); else message.info(result.text || "Agent 暂未生成状态修复确认动作"); },
        onError: (error) => { setJob(null); message.error(error instanceof Error ? error.message : "状态修复规划失败"); },
    });
    const execute = useMutation({
        mutationFn: async (confirmation: NovelAgentConfirmation) => {
            if (!foundation?.agentSessionId || !foundation.inkosBookId) throw new Error("当前小说缺少 InkOS 绑定");
            const { job: initialJob } = await startNovelAgentJob({ sessionId: foundation.agentSessionId, bookId: foundation.inkosBookId, mode: "repair-state", message: confirmation.instruction, requestedSkills: ["inkos-long-writing"], confirmedIntent: "repair_state", confirmedActionPayload: confirmation.actionPayload, model: requestModel() });
            setJob(initialJob);
            let current = initialJob;
            while (current.status === "queued" || current.status === "running") { await new Promise((resolve) => window.setTimeout(resolve, 1500)); current = (await getNovelAgentJob(current.id)).job; setJob(current); }
            if (current.status === "failed") throw new Error(current.error || current.stage || "状态修复任务失败");
            return current;
        },
        onSuccess: () => { setProposal(null); setJob(null); void queryClient.invalidateQueries({ queryKey: ["project", projectId] }); void queryClient.invalidateQueries({ queryKey: ["story", projectId] }); message.success("章节状态已修复，正文保持不变"); },
        onError: (error) => { setJob(null); message.error(error instanceof Error ? error.message : "状态修复任务失败"); },
    });
    return <div className="mt-3 rounded-md border border-warning/35 bg-warning/5 p-3"><p className="text-xs font-medium">本章状态需要修复</p><p className="mt-1 text-xs text-foreground/60">InkOS 已保留正文，但发现状态或伏笔与正文存在不一致。修复只重建状态，不改写正文。</p><Button className="mt-2" size="small" loading={plan.isPending} disabled={Boolean(execute.isPending) || !foundation?.agentSessionId} onClick={() => plan.mutate()}>规划状态修复</Button>{proposal ? <div className="mt-2 rounded bg-background p-2"><p className="text-xs font-semibold">{proposal.title}</p><p className="mt-1 whitespace-pre-wrap text-xs text-foreground/60">{proposal.summary}</p><Button className="mt-2" size="small" type="primary" loading={execute.isPending} onClick={() => execute.mutate(proposal)}>确认修复状态</Button></div> : null}{job ? <div className="mt-2 text-xs text-foreground/55" role="status"><p>修复任务：{job.stage}</p><NovelJobCancelButton jobId={job.id} /></div> : null}</div>;
}

function NovelWriterPanel({
    projectId,
    foundation,
    config,
    queryClient,
    message,
}: {
    projectId: string;
    foundation?: StoryFoundationView;
    config: ReturnType<typeof useEffectiveConfig>;
    queryClient: ReturnType<typeof useQueryClient>;
    message: ReturnType<typeof App.useApp>["message"];
}) {
    const [proposal, setProposal] = useState<NovelAgentConfirmation | null>(null);
    const [job, setJob] = useState<NovelAgentJob | null>(null);
    const requestModel = () => {
        const modelValue = config.textModel || config.model;
        if (!modelValue) throw new Error("请先在设置中选择文本模型");
        const requestConfig = resolveModelRequestConfig(config, modelValue);
        if (requestConfig.apiFormat !== "openai" || requestConfig.interfaceType === "claude-api" || requestConfig.interfaceType === "gemini-image" || requestConfig.interfaceType === "gemini-veo") {
            throw new Error("Novel Agent 首版仅支持 OpenAI 兼容文本渠道，请在设置中切换文本模型");
        }
        return {
            provider: "custom" as const,
            baseUrl: requestConfig.baseUrl,
            apiKey: requestConfig.apiKey,
            model: requestConfig.model,
            apiFormat: requestConfig.interfaceType === "openai-responses" ? "responses" as const : "chat" as const,
            temperature: 0.7,
        };
    };
    const plan = useMutation({
        mutationFn: async () => {
            if (!foundation?.agentSessionId || !foundation.inkosBookId) throw new Error("当前小说还没有绑定 InkOS Agent 会话，请从 Agent 建书完成后重新打开项目");
            const { job: initialJob } = await startNovelAgentJob({
                sessionId: foundation.agentSessionId,
                bookId: foundation.inkosBookId,
                mode: "inkos-long-writing",
                message: "请读取当前小说的故事状态和章节进度，规划并提出写下一章的确认动作。不要直接写入正文。",
                requestedSkills: ["inkos-long-writing"],
                model: requestModel(),
            });
            setJob(initialJob);
            let current = initialJob;
            while (current.status === "queued" || current.status === "running") {
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
                current = (await getNovelAgentJob(current.id)).job;
                setJob(current);
            }
            if (current.status === "failed") throw new Error(current.error || current.stage || "写作计划生成失败");
            if (!current.result) throw new Error("写作计划完成但没有返回结果");
            return current.result;
        },
        onSuccess: (result) => {
            setJob(null);
            if (result.confirmation?.action === "write_next") setProposal(result.confirmation);
            else message.info(result.text || "Agent 暂未生成写作确认动作");
        },
        onError: (error) => {
            setJob(null);
            message.error(error instanceof Error ? error.message : "写作计划生成失败");
        },
    });
    // 断线恢复（T3 工作台侧）：本项目有 running 任务时自动接续轮询到终态。
    const activeRecovery = useQuery({ queryKey: ["novel-agent", "active-jobs", projectId], queryFn: listNovelAgentActiveJobs, staleTime: 10_000 });
    const recoveredJobId = useMemo(() => {
        if (job) return "";
        const found = (activeRecovery.data?.jobs || []).find((item) => (item.status === "running" || item.status === "queued") && item.bookId === foundation?.inkosBookId);
        return found?.id || "";
    }, [activeRecovery.data?.jobs, foundation?.inkosBookId, job]);
    useEffect(() => {
        if (!recoveredJobId) return;
        let cancelled = false;
        void (async () => {
            let current = { id: recoveredJobId, status: "running" } as NovelAgentJob;
            while (!cancelled) {
                try {
                    current = (await getNovelAgentJob(current.id)).job;
                } catch {
                    return;
                }
                if (cancelled) return;
                setJob(current);
                if (current.status !== "queued" && current.status !== "running") break;
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
            }
            if (cancelled) return;
            if (current.status === "succeeded" && current.result?.confirmation?.action === "write_next") {
                setProposal(current.result.confirmation);
            }
            message.info(current.status === "succeeded" ? "已恢复并完成任务，请查看结果" : `任务已结束：${current.error || ""}`);
            void queryClient.invalidateQueries({ queryKey: ["story", projectId] });
            void queryClient.invalidateQueries({ queryKey: ["project", projectId] });
        })();
        return () => { cancelled = true; };
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [recoveredJobId]);
    const cancelWriterJob = useMutation({
        mutationFn: (jobId: string) => cancelNovelAgentJob(jobId),
        onSuccess: () => message.info("已发送取消请求，等待 InkOS 安全停止"),
        onError: (error) => message.error(error instanceof Error ? error.message : "取消任务失败"),
    });
    const draftOnly = useMutation({
        mutationFn: async () => {
            if (!foundation?.agentSessionId || !foundation.inkosBookId) throw new Error("当前小说还没有绑定 InkOS Agent 会话，请从 Agent 建书完成后重新打开项目");
            const { job: initialJob } = await startNovelAgentJob({
                sessionId: foundation.agentSessionId,
                bookId: foundation.inkosBookId,
                mode: "inkos-long-writing",
                message: "仅写下一章草稿，写完即停，不要审计",
                requestedSkills: ["inkos-long-writing"],
                confirmedIntent: "draft_next",
                confirmedActionPayload: {},
                model: requestModel(),
            });
            setJob(initialJob);
            let current = initialJob;
            while (current.status === "queued" || current.status === "running") {
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
                current = (await getNovelAgentJob(current.id)).job;
                setJob(current);
            }
            if (current.status === "failed") throw new Error(current.error || current.stage || "草稿任务失败");
            if (!current.result) throw new Error("草稿任务完成但没有返回结果");
            return current.result;
        },
        onSuccess: (result) => {
            setJob(null);
            void queryClient.invalidateQueries({ queryKey: ["project", projectId] });
            void queryClient.invalidateQueries({ queryKey: ["story", projectId] });
            message.success(result.text || "草稿已完成");
        },
        onError: (error) => {
            setJob(null);
            message.error(error instanceof Error ? error.message : "草稿任务失败");
        },
    });

    const execute = useMutation({
        mutationFn: async (confirmation: NovelAgentConfirmation) => {
            if (!foundation?.agentSessionId || !foundation.inkosBookId) throw new Error("当前小说缺少 InkOS 绑定");
            const { job: initialJob } = await startNovelAgentJob({
                sessionId: foundation.agentSessionId,
                bookId: foundation.inkosBookId,
                mode: "inkos-long-writing",
                message: confirmation.instruction,
                requestedSkills: ["inkos-long-writing"],
                confirmedIntent: "write_next",
                confirmedActionPayload: confirmation.actionPayload,
                model: requestModel(),
            });
            setJob(initialJob);
            let current = initialJob;
            while (current.status === "queued" || current.status === "running") {
                await new Promise((resolve) => window.setTimeout(resolve, 1500));
                current = (await getNovelAgentJob(current.id)).job;
                setJob(current);
            }
            if (current.status === "failed") throw new Error(current.error || current.stage || "写作任务失败");
            return current;
        },
        onSuccess: (completedJob) => {
            setProposal(null);
            setJob(null);
            void queryClient.invalidateQueries({ queryKey: ["project", projectId] });
            void queryClient.invalidateQueries({ queryKey: ["story", projectId] });
            let completed = 0;
            let reviewable = 0;
            try {
                const index = completedJob.result?.artifacts?.chapterIndex;
                if (index) {
                    const parsed = JSON.parse(index) as Array<{ status?: string }>;
                    completed = parsed.length;
                    reviewable = parsed.filter((item) => item.status === "ready-for-review").length;
                }
            } catch {
                completed = 0;
            }
            const requested = Number(proposal?.actionPayload?.writeNext?.chapterCount ?? 1);
            message.success(completed > 1
                ? `已连续同步 ${completed} 章，其中 ${reviewable} 章待复核`
                : completed === 1 ? `第 ${completed} 章已完成生产并同步` : "写作任务已完成并同步");
            if (requested > 1 && completed < requested) {
                message.warning(`批量任务提前停止：请求 ${requested} 章，实际同步 ${completed} 章`);
            }
        },
        onError: (error) => {
            setJob(null);
            message.error(error instanceof Error ? error.message : "写作任务失败");
        },
    });
    return (
        <section className="mt-5 rounded-md border border-primary/25 bg-primary/5 p-3" aria-label="InkOS Writer 生产管线">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <p className="text-sm font-medium">InkOS Writer 生产管线</p>
                    <p className="mt-1 text-xs leading-5 text-foreground/55">读取当前故事记忆，由 Writer 写作并经过 Auditor/Reviser 质量门禁；确认后才会修改正文。</p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                    <Button type="primary" loading={plan.isPending} disabled={Boolean(execute.isPending) || !foundation?.agentSessionId} onClick={() => plan.mutate()}>
                        让 Agent 写下一章
                    </Button>
                    <Button loading={draftOnly.isPending} disabled={Boolean(execute.isPending) || !foundation?.agentSessionId} onClick={() => draftOnly.mutate()}>
                        仅草稿·写完即停
                    </Button>
                </div>
            </div>
            {!foundation?.agentSessionId ? <p className="mt-2 text-xs text-warning">当前项目没有绑定 Agent 会话，无法安全定位 InkOS 书籍。</p> : null}
            {proposal ? (
                <div className="mt-3 rounded-md border border-primary/30 bg-background p-3">
                    <p className="text-sm font-semibold">{proposal.title}</p>
                    <p className="mt-1 whitespace-pre-wrap text-xs leading-5 text-foreground/65">{proposal.summary}</p>
                    <div className="mt-3 flex flex-wrap items-center gap-2">
                        <span className="text-xs text-foreground/55">连续写作章数</span>
                        <Select
                            size="small"
                            className="w-24"
                            value={proposal.actionPayload?.writeNext?.chapterCount ?? 1}
                            disabled={execute.isPending}
                            onChange={(value) => setProposal({ ...proposal, actionPayload: { ...proposal.actionPayload, writeNext: { ...proposal.actionPayload?.writeNext, chapterCount: value } } })}
                            options={[1, 2, 3, 5, 10].map((value) => ({ value, label: `${value} 章` }))}
                        />
                    </div>
                    <Button className="mt-3" type="primary" loading={execute.isPending} onClick={() => execute.mutate(proposal)}>
                        确认执行 Writer
                    </Button>
                </div>
            ) : null}
            {job ? <AgentTaskCard job={job} title="写作任务" targetChapters={Number(proposal?.actionPayload?.writeNext?.chapterCount ?? 1)} onCancel={(jobId) => cancelWriterJob.mutate(jobId)} /> : null}
        </section>
    );
}

function StoryToolsPanel({
    versions,
    reviews,
    memories,
    branches,
    scenes,
    reviewSummary,
    memoryText,
    branchTitle,
    branchPlan,
    sceneTitle,
    sceneAction,
    characterName,
    onReviewSummary,
    onMemoryText,
    onBranchTitle,
    onBranchPlan,
    onSceneTitle,
    onSceneAction,
    onCharacterName,
    onSaveReview,
    onSaveMemory,
    onCreateBranch,
    onCreateScene,
    onCreateCharacter,
    onCreateSceneShot,
    onRestore,
}: {
    versions: Array<{ id: string; number: number; createdAt: string }>;
    reviews: Array<{ id: string; summary: string; createdAt: string }>;
    memories: Array<{ id: string; kind: string; content: string }>;
    branches: Array<{ id: string; title: string; status: string }>;
    scenes: Array<{ id: string; title: string; action: string; status: string }>;
    reviewSummary: string;
    memoryText: string;
    branchTitle: string;
    branchPlan: string;
    sceneTitle: string;
    sceneAction: string;
    characterName: string;
    onReviewSummary: (value: string) => void;
    onMemoryText: (value: string) => void;
    onBranchTitle: (value: string) => void;
    onBranchPlan: (value: string) => void;
    onSceneTitle: (value: string) => void;
    onSceneAction: (value: string) => void;
    onCharacterName: (value: string) => void;
    onSaveReview: () => void;
    onSaveMemory: () => void;
    onCreateBranch: () => void;
    onCreateScene: () => void;
    onCreateCharacter: () => void;
    onCreateSceneShot: (sceneId: string) => void;
    onRestore: (versionId: string) => void;
}) {
    return (
        <div className="mt-5 grid gap-3 xl:grid-cols-2">
            <section className="rounded-md border border-border p-3">
                <h3 className="flex items-center gap-2 text-sm font-medium">
                    <SearchCheck className="size-4" />
                    审稿与修订
                </h3>
                <Input.TextArea className="mt-3" rows={3} value={reviewSummary} onChange={(event) => onReviewSummary(event.target.value)} placeholder="记录本章的节奏、人物、逻辑或语言问题。" />
                <Button className="mt-2" onClick={onSaveReview}>
                    保存审稿记录
                </Button>
                <div className="mt-3 grid gap-2">
                    {reviews.slice(0, 3).map((item) => (
                        <p key={item.id} className="rounded bg-surface-active p-2 text-xs leading-5 text-foreground/65">
                            {item.summary || "无摘要"}
                        </p>
                    ))}
                    {!reviews.length ? <p className="text-xs text-foreground/45">还没有审稿记录。</p> : null}
                </div>
            </section>
            <section className="rounded-md border border-border p-3">
                <h3 className="flex items-center gap-2 text-sm font-medium">
                    <History className="size-4" />
                    版本回溯
                </h3>
                <div className="mt-3 grid gap-2">
                    {versions.map((item) => (
                        <div key={item.id} className="flex items-center justify-between gap-2 rounded bg-surface-active p-2">
                            <span className="text-xs">版本 {item.number}</span>
                            <Button size="small" onClick={() => onRestore(item.id)}>
                                恢复
                            </Button>
                        </div>
                    ))}
                    {!versions.length ? <p className="text-xs text-foreground/45">保存正文后创建版本快照。</p> : null}
                </div>
            </section>
            <section className="rounded-md border border-border p-3">
                <h3 className="flex items-center gap-2 text-sm font-medium">
                    <Lightbulb className="size-4" />
                    故事记忆
                </h3>
                <Input.TextArea className="mt-3" rows={3} value={memoryText} onChange={(event) => onMemoryText(event.target.value)} placeholder="记录不能丢失的人物事实、世界规则或伏笔。" />
                <Button className="mt-2" onClick={onSaveMemory}>
                    添加记忆
                </Button>
                <div className="mt-3 grid gap-2">
                    {memories.slice(0, 3).map((item) => (
                        <p key={item.id} className="rounded bg-surface-active p-2 text-xs leading-5 text-foreground/65">
                            {item.content}
                        </p>
                    ))}
                    {!memories.length ? <p className="text-xs text-foreground/45">还没有故事记忆。</p> : null}
                </div>
            </section>
            <section className="rounded-md border border-border p-3">
                <h3 className="flex items-center gap-2 text-sm font-medium">
                    <GitBranch className="size-4" />
                    未来分支
                </h3>
                <Input className="mt-3" value={branchTitle} onChange={(event) => onBranchTitle(event.target.value)} placeholder="分支名称" />
                <Input.TextArea className="mt-2" rows={2} value={branchPlan} onChange={(event) => onBranchPlan(event.target.value)} placeholder="记录这个可能走向，不会修改当前正文。" />
                <Button className="mt-2" onClick={onCreateBranch}>
                    创建分支
                </Button>
                <div className="mt-3 grid gap-2">
                    {branches.slice(0, 3).map((item) => (
                        <p key={item.id} className="rounded bg-surface-active p-2 text-xs text-foreground/65">
                            {item.title} · {item.status}
                        </p>
                    ))}
                    {!branches.length ? <p className="text-xs text-foreground/45">还没有未来分支。</p> : null}
                </div>
            </section>
            <section className="rounded-md border border-border p-3">
                <h3 className="flex items-center gap-2 text-sm font-medium">
                    <BookOpenText className="size-4" />
                    角色资产
                </h3>
                <Input className="mt-3" value={characterName} onChange={(event) => onCharacterName(event.target.value)} placeholder="角色名称" />
                <Button className="mt-2" onClick={onCreateCharacter}>
                    创建角色资产
                </Button>
                <p className="mt-3 text-xs leading-5 text-foreground/45">角色会进入现有项目资产库，后续可生成角色图和绑定到场次。</p>
            </section>
            <section className="rounded-md border border-border p-3">
                <h3 className="flex items-center gap-2 text-sm font-medium">
                    <FileText className="size-4" />
                    剧本场次
                </h3>
                <Input className="mt-3" value={sceneTitle} onChange={(event) => onSceneTitle(event.target.value)} placeholder="场次标题，例如：雨夜车站" />
                <Input.TextArea className="mt-2" rows={2} value={sceneAction} onChange={(event) => onSceneAction(event.target.value)} placeholder="写下动作、对白和视觉事件。" />
                <Button className="mt-2" onClick={onCreateScene}>
                    创建场次
                </Button>
                <div className="mt-3 grid gap-2">
                    {scenes.map((item) => (
                        <div key={item.id} className="rounded bg-surface-active p-2">
                            <p className="text-xs leading-5 text-foreground/65">
                                {item.title}
                                {item.action ? `：${item.action}` : ""}
                            </p>
                            <Button size="small" className="mt-2" onClick={() => onCreateSceneShot(item.id)}>
                                生成镜头
                            </Button>
                        </div>
                    ))}
                    {!scenes.length ? <p className="text-xs text-foreground/45">还没有剧本场次。</p> : null}
                </div>
            </section>
        </div>
    );
}

function NovelScriptCharacterCandidatesPanel({
    candidates,
    confirmingId,
    onConfirm,
}: {
    candidates: ProjectAssetCandidate[];
    confirmingId: string;
    onConfirm: (candidateId: string) => void;
}) {
    const pendingCharacters = candidates.filter((candidate) => candidate.category === "character" && candidate.status === "pending_confirmation" && candidate.source === "inkos_script_character");
    if (!pendingCharacters.length) return null;
    return (
        <section className="mt-5 rounded-md border border-primary/25 bg-primary/5 p-3" aria-label="InkOS 剧本待确认角色">
            <div className="flex flex-wrap items-start justify-between gap-2">
                <div>
                    <h3 className="flex items-center gap-2 text-sm font-medium">
                        <BookOpenText className="size-4" />
                        剧本待确认角色
                    </h3>
                    <p className="mt-1 text-xs leading-5 text-foreground/55">这些人物来自 InkOS 剧本，但尚未有项目角色卡；确认后会自动进入当前场次和镜头引用。</p>
                </div>
                <Tag color="blue">{pendingCharacters.length} 位待确认</Tag>
            </div>
            <div className="mt-3 grid gap-2">
                {pendingCharacters.map((candidate) => {
                    let sceneTitle = "当前剧本场次";
                    try {
                        const details = JSON.parse(candidate.detailsJson) as { sceneTitle?: unknown };
                        if (typeof details.sceneTitle === "string" && details.sceneTitle.trim()) sceneTitle = details.sceneTitle;
                    } catch {
                        sceneTitle = "当前剧本场次";
                    }
                    return (
                        <div key={candidate.id} className="flex flex-wrap items-center justify-between gap-3 rounded bg-background p-3">
                            <div>
                                <p className="text-sm font-medium">{candidate.name}</p>
                                <p className="mt-1 text-xs text-foreground/55">来源：{sceneTitle}</p>
                            </div>
                            <Button size="small" type="primary" loading={confirmingId === candidate.id} onClick={() => onConfirm(candidate.id)}>
                                确认创建角色卡
                            </Button>
                        </div>
                    );
                })}
            </div>
        </section>
    );
}

function NovelShotGenerationPanel({
    projectId,
    unitId,
    detail,
    scenes,
    onRefresh,
}: {
    projectId: string;
    unitId: string;
    detail: ProjectDetail;
    scenes: Array<{ id: string; title: string; action: string; dialogue: string; emotion: string; visualIntent: string; characterAssetIdsJson?: string; shotId?: string; shotIds?: string[] }>;
    onRefresh: () => void;
}) {
    const { message } = App.useApp();
    const config = useEffectiveConfig();
    const isAiConfigReady = useConfigStore((state) => state.isAiConfigReady);
    const [mode, setMode] = useState<"image" | "video">("image");
    const [selectedSceneId, setSelectedSceneId] = useState("");
    const [selectedModel, setSelectedModel] = useState("");
    const [selectedReferenceIds, setSelectedReferenceIds] = useState<string[]>([]);
    const modelOptions = useMemo(() => selectableModelsByCapability(config, mode), [config, mode]);
    const defaultModel = mode === "image" ? config.imageModel : config.videoModel;
    const activeModel = modelOptions.includes(selectedModel) ? selectedModel : defaultModel || modelOptions[0] || "";
    const linkedScenes = useMemo(() => scenes.flatMap((scene) => {
        const shotIds = scene.shotIds?.length ? scene.shotIds : scene.shotId ? [scene.shotId] : [];
        return shotIds.map((shotId, index) => ({
            ...scene,
            shotId,
            selectionId: `${scene.id}:${shotId}`,
            label: shotIds.length > 1 ? `${scene.title} · 镜头 ${index + 1}` : scene.title,
        }));
    }), [scenes]);
    const selectedScene = linkedScenes.find((scene) => scene.selectionId === selectedSceneId) || linkedScenes[0];
    const selectedShot = selectedScene?.shotId ? detail.shots.find((shot) => shot.id === selectedScene.shotId) : undefined;
    const artifactType = mode === "image" ? "storyboard" : "video";
    const referenceAssets = useMemo(
        () => detail.assets.filter((asset) => asset.primaryVersionId && ["character", "environment", "prop", "material"].includes(asset.category)),
        [detail.assets],
    );
    const selectedReferenceAssets = useMemo(
        () => selectedReferenceIds
            .map((assetId) => referenceAssets.find((asset) => asset.id === assetId))
            .filter((asset): asset is NonNullable<typeof asset> => Boolean(asset)),
        [referenceAssets, selectedReferenceIds],
    );
    const selectedSceneCharacterAssetIds = useMemo(() => {
        if (!selectedScene?.characterAssetIdsJson) return [];
        try {
            const parsed = JSON.parse(selectedScene.characterAssetIdsJson) as unknown;
            if (!Array.isArray(parsed)) return [];
            const available = new Set(referenceAssets.map((asset) => asset.id));
            return parsed.filter((value): value is string => typeof value === "string" && available.has(value));
        } catch {
            return [];
        }
    }, [referenceAssets, selectedScene?.characterAssetIdsJson]);
    useEffect(() => {
        setSelectedReferenceIds(selectedSceneCharacterAssetIds);
    }, [selectedScene?.selectionId]);
    const activeTask = useMemo(() => {
        if (!selectedShot) return undefined;
        return (detail.tasks || [])
            .filter((task) => task.clientContext?.shotId === selectedShot.id && task.clientContext?.artifactType === artifactType)
            .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0];
    }, [artifactType, detail.tasks, selectedShot?.id]);
    const activeArtifact = useMemo(() => {
        if (!selectedShot) return undefined;
        return (detail.shotArtifacts || [])
            .filter((item) => item.shotId === selectedShot.id && item.type === artifactType)
            .sort((left, right) => right.version - left.version)
            .find((item) => item.selected) || (detail.shotArtifacts || []).find((item) => item.shotId === selectedShot.id && item.type === artifactType);
    }, [artifactType, detail.shotArtifacts, selectedShot?.id]);

    const submitGeneration = useMutation({
        mutationFn: async () => {
            if (!selectedScene || !selectedScene.shotId || !selectedShot) throw new Error("请选择一个已经关联镜头的场次");
            if (!activeModel) throw new Error(mode === "image" ? "请先配置图片模型" : "请先配置视频模型");
            let workflowStep = (detail.workflows || [])
                .flatMap((workflow) => workflow.steps || [])
                .find((step) => step.stepKey === artifactType);
            if (!workflowStep) {
                const initialized = await createUnitWorkflow(projectId, unitId);
                workflowStep = (initialized.workflow.steps || []).find((step) => step.stepKey === artifactType);
            }
            if (!workflowStep) throw new Error("生产工作流初始化失败，请刷新后重试");
            const durationSeconds = Math.max(1, Math.round((selectedShot.durationMs || 3000) / 1000));
            const generationConfig = {
                ...config,
                model: activeModel,
                imageModel: mode === "image" ? activeModel : config.imageModel,
                videoModel: mode === "video" ? activeModel : config.videoModel,
                size: detail.project.aspectRatio || config.size || "16:9",
                videoSeconds: String(durationSeconds),
            };
            if (!isAiConfigReady(generationConfig, activeModel)) throw new Error("当前模型渠道配置不完整，请先到设置中补齐");
            const basePrompt = [selectedScene.visualIntent || selectedScene.action, selectedScene.action, selectedScene.dialogue && `台词：${selectedScene.dialogue}`, selectedScene.emotion && `情绪：${selectedScene.emotion}`]
                .filter(Boolean)
                .join("\n");
            const prompt = mode === "image"
                ? `${basePrompt}\n\n生成电影感分镜图：清晰构图、景别与人物动作，避免文字水印。`
                : `${basePrompt}\n\n生成连续镜头视频：动作流畅，镜头语言明确，保持角色一致性。`;
            const saved = await saveProjectShot(projectId, {
                id: selectedShot.id,
                unitId,
                title: selectedShot.title,
                description: selectedScene.action,
                position: selectedShot.position,
                durationMs: selectedShot.durationMs,
                status: selectedShot.status,
                revision: {
                    plotDescription: selectedScene.action || selectedScene.visualIntent,
                    action: selectedScene.action,
                    dialogue: selectedScene.dialogue,
                    imagePrompt: selectedScene.visualIntent || selectedScene.action,
                    videoPrompt: selectedScene.visualIntent || selectedScene.action,
                    continuityNotes: selectedScene.emotion,
                    durationMs: selectedShot.durationMs,
                },
            });
            const existingReferenceVersionIds = new Set(
                (detail.shotReferences || [])
                    .filter((reference) => reference.shotId === saved.shot.id)
                    .map((reference) => reference.assetVersionId),
            );
            for (const asset of selectedReferenceAssets) {
                if (!asset.primaryVersionId || existingReferenceVersionIds.has(asset.primaryVersionId)) continue;
                await linkShotAsset(projectId, saved.shot.id, { assetVersionId: asset.primaryVersionId, role: "reference" });
            }
            await submitBackendGenerationTask({
                projectId,
                mode,
                prompt,
                config: generationConfig,
                referenceImages: selectedReferenceAssets
                    .filter((asset) => asset.storageKey)
                    .map((asset) => ({
                        id: asset.primaryVersionId || asset.id,
                        name: asset.title,
                        type: "image",
                        dataUrl: "",
                        storageKey: asset.storageKey,
                    })),
                metadata: {
                    workflowStepId: workflowStep.id,
                    domainProjectId: projectId,
                    unitId,
                    shotId: saved.shot.id,
                    shotRevisionId: saved.shot.currentRevisionId,
                    artifactType,
                    role: "output",
                    source: "novel-shot-generation",
                    artifactMetadata: {
                        model: activeModel,
                        aspectRatio: generationConfig.size,
                        durationSeconds,
                        sceneId: selectedScene.id,
                        sceneTitle: selectedScene.title,
                        referenceAssetIds: selectedReferenceAssets.map((asset) => asset.id),
                    },
                },
            });
            return saved.shot.id;
        },
        onSuccess: () => {
            onRefresh();
            message.success(`${mode === "image" ? "分镜图" : "视频"}生成任务已提交`);
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "生成任务提交失败"),
    });

    const taskStatus = activeTask?.status === "succeeded"
        ? "已完成"
        : activeTask?.status === "failed"
            ? "失败"
            : activeTask?.status === "cancelled"
                ? "已取消"
                : activeTask
                    ? "生成中"
                    : "未生成";

    return (
        <section className="mt-4 rounded-md border border-border p-3">
            <h3 className="flex items-center gap-2 text-sm font-medium">
                <Film className="size-4" />
                镜头生成
            </h3>
            <p className="mt-2 text-xs leading-5 text-foreground/55">选择已生成镜头的场次，确认后把该镜头提交到现有图片或视频任务链路。</p>
            <div className="mt-3 grid gap-2 md:grid-cols-[180px_150px_minmax(0,1fr)_auto]">
                <Select
                    size="small"
                    value={selectedScene?.selectionId || ""}
                    placeholder="选择场次"
                    onChange={setSelectedSceneId}
                    options={linkedScenes.map((scene) => ({ value: scene.selectionId, label: scene.label }))}
                />
                <Select
                    size="small"
                    value={mode}
                    onChange={setMode}
                    options={[{ value: "image", label: "分镜图" }, { value: "video", label: "视频" }]}
                />
                <ModelPicker
                    config={config}
                    value={activeModel}
                    capability={mode}
                    variant="default"
                    showSelectedPrice={false}
                    placeholder={mode === "image" ? "选择图片模型" : "选择视频模型"}
                    onChange={setSelectedModel}
                />
                <Select
                    mode="multiple"
                    size="small"
                    className="md:col-span-2"
                    value={selectedReferenceIds}
                    onChange={setSelectedReferenceIds}
                    placeholder="引用角色/场景/道具资产（可选）"
                    options={referenceAssets.map((asset) => ({
                        value: asset.id,
                        label: `${asset.title}${asset.character ? "（角色卡）" : ""}`,
                    }))}
                    maxCount={6}
                />
                <Button type="primary" loading={submitGeneration.isPending} disabled={!selectedShot} onClick={() => submitGeneration.mutate()}>
                    确认生成
                </Button>
            </div>
            <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-foreground/60">
                <Tag color={activeTask?.status === "failed" ? "red" : activeTask ? "blue" : "default"}>{taskStatus}</Tag>
                <span>产物状态：{activeArtifact?.status || "无"}</span>
                {selectedSceneCharacterAssetIds.length ? <span>已自动带入 {selectedSceneCharacterAssetIds.length} 个角色参考</span> : null}
                {activeTask?.error ? <span className="text-foreground/45">错误：{activeTask.error}</span> : null}
            </div>
            {activeTask?.previewUrl && activeTask.previewKind === "image" ? (
                <img src={activeTask.previewUrl} alt="镜头分镜图预览" className="mt-3 max-h-64 rounded-md border border-border object-contain" />
            ) : null}
            {activeTask?.previewUrl && activeTask.previewKind === "video" ? (
                <video src={activeTask.previewUrl} controls className="mt-3 max-h-64 w-full rounded-md border border-border" />
            ) : null}
        </section>
    );
}

function StoryStage({ title, detail, ready = false }: { title: string; detail: string; ready?: boolean }) {
    return (
        <div className="rounded-md bg-surface-active p-3">
            <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium">{title}</span>
                <span className={ready ? "text-xs text-success" : "text-xs text-foreground/45"}>{ready ? "已就绪" : "待开始"}</span>
            </div>
            <p className="mt-1 text-xs text-foreground/55">{detail}</p>
        </div>
    );
}

const foundationFileLabels: Record<string, string> = {
    "story/author_intent.md": "作者意图",
    "story/brief.md": "创作简报",
    "story/story_bible.md": "故事基石",
    "story/book_rules.md": "书籍规则",
    "story/character_matrix.md": "角色矩阵",
    "story/current_state.md": "当前状态",
    "story/pending_hooks.md": "伏笔池",
    "story/emotional_arcs.md": "情感弧线",
    "story/style_guide.md": "风格指南",
    "story/outline/story_frame.md": "故事结构",
    "story/outline/volume_map.md": "卷纲规划",
};

function StoryFoundationPanel({ projectId, foundation, isLoading, queryClient, message, config }: { projectId: string; foundation?: StoryFoundationView; isLoading: boolean; queryClient: ReturnType<typeof useQueryClient>; message: ReturnType<typeof App.useApp>["message"]; config: ReturnType<typeof useEffectiveConfig> }) {
    const [selectedPath, setSelectedPath] = useState("story/story_bible.md");
    const [editing, setEditing] = useState(false);
    const [editDraft, setEditDraft] = useState("");
    const exportsQuery = useQuery({ queryKey: ["story", projectId, "exports"], queryFn: () => listNovelExports(projectId), enabled: Boolean(projectId) });
    const downloadExport = useMutation({
        mutationFn: async (exportPath: string) => {
            const result = await readNovelExport(projectId, exportPath);
            const binary = atob(result.content);
            const bytes = new Uint8Array(binary.length);
            for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
            const blob = new Blob([bytes], { type: exportPath.endsWith(".epub") ? "application/epub+zip" : exportPath.endsWith(".md") ? "text/markdown" : "text/plain" });
            const url = URL.createObjectURL(blob);
            const link = document.createElement("a");
            link.href = url;
            link.download = exportPath.split("/").at(-1) || "export.txt";
            link.click();
            URL.revokeObjectURL(url);
        },
        onSuccess: () => message.success("导出文件已下载"),
        onError: (error) => message.error(error instanceof Error ? error.message : "导出下载失败"),
    });
    const evaluate = useMutation({
        mutationFn: () => evaluateNovelBook(projectId),
        onSuccess: (result) => {
            const summary = JSON.stringify(result.eval).slice(0, 400);
            message.info({ content: `书籍评估完成：${summary}`, duration: 8 });
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "书籍评估失败"),
    });
    const radar = useMutation({
        mutationFn: async () => {
            const modelValue = config.textModel || config.model;
            if (!modelValue) throw new Error("请先在设置中选择文本模型");
            const requestConfig = resolveModelRequestConfig(config, modelValue);
            return runMarketRadar(projectId, { baseUrl: requestConfig.baseUrl, apiKey: requestConfig.apiKey, model: requestConfig.model, apiFormat: requestConfig.interfaceType === "openai-responses" ? "responses" : "chat" });
        },
        onSuccess: (result) => {
            const summary = JSON.stringify(result.radar).slice(0, 500);
            message.info({ content: `市场雷达完成：${summary}`, duration: 10 });
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "市场雷达失败"),
    });
    const saveEdit = useMutation({
        mutationFn: async () => {
            await writeNovelTruth(projectId, selectedPath, editDraft);
        },
        onSuccess: () => {
            setEditing(false);
            void queryClient.invalidateQueries({ queryKey: ["story", projectId] });
            message.success("真相文件已写回 InkOS 工作区");
        },
        onError: (error) => message.error(error instanceof Error ? error.message : "真相文件保存失败"),
    });
    const availableFiles = useMemo(() => {
        if (!foundation) return [];
        const labelOrder = Object.keys(foundationFileLabels);
        return Object.keys(foundation.files).filter((filePath) => foundation.files[filePath]?.trim()).sort((left, right) => {
            const leftIndex = labelOrder.indexOf(left);
            const rightIndex = labelOrder.indexOf(right);
            return (leftIndex < 0 ? 999 : leftIndex) - (rightIndex < 0 ? 999 : rightIndex) || left.localeCompare(right);
        });
    }, [foundation]);
    useEffect(() => {
        if (availableFiles.length && !availableFiles.includes(selectedPath)) setSelectedPath(availableFiles[0]);
    }, [availableFiles, selectedPath]);
    if (isLoading) return <div className="mt-5 rounded-md border border-border bg-surface-active px-4 py-3 text-xs text-foreground/55">正在加载 InkOS 核心文件…</div>;
    if (!foundation || !availableFiles.length) return null;
    const truthEditable = selectedPath.startsWith("story/");
    return (
        <section className="mt-5 rounded-md border border-border bg-surface-active p-4" aria-label="故事核心文件">
            <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                    <h3 className="text-sm font-semibold">故事核心文件</h3>
                    <p className="mt-1 text-xs text-foreground/55">来自 InkOS 建书 Pipeline · 来源哈希 {foundation.sourceHash.slice(0, 20)}…</p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                    <Button size="small" loading={evaluate.isPending} onClick={() => evaluate.mutate()}>书籍评估</Button>
                    <Button size="small" loading={radar.isPending} onClick={() => radar.mutate()}>市场雷达</Button>
                    <Tag color="blue">{availableFiles.length} 个文件</Tag>
                </div>
            </div>
            <div className="mt-4 grid gap-4 lg:grid-cols-[190px_minmax(0,1fr)]">
                <nav className="grid content-start gap-1" aria-label="核心文件列表">
                    {availableFiles.map((filePath) => (
                        <button
                            key={filePath}
                            type="button"
                            aria-pressed={selectedPath === filePath}
                            className={`rounded px-3 py-2 text-left text-xs ${selectedPath === filePath ? "bg-primary/12 text-foreground" : "text-foreground/65 hover:bg-background"}`}
                            onClick={() => { setSelectedPath(filePath); setEditing(false); }}
                        >
                            {foundationFileLabels[filePath] || filePath}
                        </button>
                    ))}
                </nav>
                <article className="max-h-[440px] overflow-auto rounded-md border border-border bg-background p-4">
                    <div className="flex items-center justify-between gap-2">
                        <h4 className="text-sm font-medium">{foundationFileLabels[selectedPath] || selectedPath}</h4>
                        {truthEditable ? (
                            editing ? (
                                <div className="flex gap-1.5">
                                    <Button size="small" type="primary" loading={saveEdit.isPending} onClick={() => saveEdit.mutate()}>保存</Button>
                                    <Button size="small" onClick={() => setEditing(false)}>取消</Button>
                                </div>
                            ) : (
                                <Button size="small" onClick={() => { setEditDraft(foundation.files[selectedPath] || ""); setEditing(true); }}>编辑</Button>
                            )
                        ) : null}
                    </div>
                    {editing ? (
                        <Input.TextArea className="mt-3" rows={18} value={editDraft} onChange={(event) => setEditDraft(event.target.value)} aria-label="真相文件编辑" />
                    ) : (
                        <pre className="mt-3 whitespace-pre-wrap break-words text-xs leading-6 text-foreground/75">{foundation.files[selectedPath]}</pre>
                    )}
                </article>
            </div>
            {exportsQuery.data?.files?.length ? (
                <div className="mt-4 border-t border-border pt-3" aria-label="导出文件">
                    <h4 className="text-sm font-semibold">导出文件</h4>
                    <div className="mt-2 grid gap-1">
                        {exportsQuery.data.files.map((file) => (
                            <div key={file.path} className="flex items-center justify-between rounded px-2 py-1.5 text-xs hover:bg-background">
                                <span className="truncate text-foreground/70">{file.path.split("/").slice(-2).join("/")}</span>
                                <div className="flex items-center gap-2 text-muted-foreground">
                                    <span className="tabular-nums">{(file.size / 1024).toFixed(1)} KB</span>
                                    <Button size="small" type="text" loading={downloadExport.isPending && downloadExport.variables === file.path} onClick={() => downloadExport.mutate(file.path)}>下载</Button>
                                </div>
                            </div>
                        ))}
                    </div>
                </div>
            ) : null}
        </section>
    );
}


