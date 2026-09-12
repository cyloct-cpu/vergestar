import { apiClient, request } from "@/services/api/request";

const api = apiClient;

export type NovelAgentSkill = { id: string; name: string; description: string; source: string };
export type NovelAgentSession = {
    id: string;
    userId: string;
    projectId?: string;
    title: string;
    mode: string;
    status: string;
    createdAt: string;
    updatedAt: string;
};
export type NovelAgentConfirmationActionPayload = {
    createBook?: Record<string, unknown>;
    writeNext?: { chapterCount?: number };
    repairState?: { chapterNumber?: number };
    reviseChapter?: { chapterNumber?: number; mode?: "spot-fix" | "polish" | "rewrite" | "rework" | "anti-detect"; instruction?: string };
    scriptCreate?: { title: string; chapterNumber?: number; episodeCount?: number; instruction?: string };
    storyboardCreate?: { title: string; chapterNumber?: number; maxShots?: number; visualStyle?: string; aspectRatio?: string; instruction?: string };
    shortRun?: { title?: string; direction?: string; storyId?: string; chapters?: number; charsPerChapter?: number; language?: "zh" | "en"; cover?: boolean };    fanficCreate?: { title?: string; sourceText?: string; sourcePath?: string; sourceName?: string; mode?: "canon" | "au" | "ooc" | "cp"; genre?: string; platform?: "tomato" | "qidian" | "feilu" | "other"; language?: "zh" | "en"; targetChapters?: number; chapterWordCount?: number };
    continuationImport?: { bookId?: string; title?: string; sourcePath?: string; splitPattern?: string; resumeFrom?: number; genre?: string; platform?: "tomato" | "qidian" | "feilu" | "other"; language?: "zh" | "en"; targetChapters?: number; chapterWordCount?: number };
    spinoffCreate?: { title?: string; parentBookId?: string; direction?: string; genre?: string; platform?: "tomato" | "qidian" | "feilu" | "other"; language?: "zh" | "en"; targetChapters?: number; chapterWordCount?: number };
    imitationCreate?: { title?: string; referenceText?: string; referencePath?: string; storyIdea?: string; sourceName?: string; genre?: string; platform?: "tomato" | "qidian" | "feilu" | "other"; language?: "zh" | "en"; targetChapters?: number; chapterWordCount?: number };
    translationCreate?: { filePath?: string; sourceLanguage?: string; targetLanguage?: string; title?: string; segmentMaxChars?: number };
    interactiveFilmCreate?: { title?: string; requirements?: string; targetAudience?: string; episodeCount?: number; episodeDuration?: string; budget?: string; referenceMode?: string; projectId?: string; outDir?: string };
    playStart?: { title?: string; premise?: string; worldContract?: string; visualContract?: string; mode?: "open" | "guided"; initialScene?: string; suggestedActions?: string[] };
    playStep?: Record<string, never>;
    [key: string]: unknown;
};
export type NovelAgentConfirmation = { action: "create_book" | "write_next" | "repair_state" | "revise_chapter" | "create_script" | "create_storyboard" | "short_run" | "fanfic_init" | "continuation_import" | "spinoff_create" | "style_imitation" | "translation_create" | "interactive_film_create" | "play_start" | "play_step"; title: string; summary: string; instruction: string; requestedSkills: string[]; actionPayload: NovelAgentConfirmationActionPayload };
export type NovelAgentSessionMessage = {
    id: string;
    role: "user" | "assistant";
    content: string;
    skills: NovelAgentSkill[];
    confirmation?: NovelAgentConfirmation;
    createdAt: string;
};
export type NovelAgentTurnResult = {
    session: NovelAgentSession;
    text: string;
    skills: NovelAgentSkill[];
    missingSkillIds: string[];
    usage: { promptTokens: number; completionTokens: number; totalTokens: number };
    storyProjectId?: string;
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
    confirmation?: NovelAgentConfirmation;
};

export type NovelAgentJob = {
    id: string;
    status: "queued" | "running" | "succeeded" | "failed";
    stage: string;
    logs: string[];
    result?: NovelAgentTurnResult;
    storyProjectId?: string;
    error?: string;
    createdAt: string;
    updatedAt: string;
};

export function listNovelAgentSkills() {
    return request<{ skills: NovelAgentSkill[] }>(api.get("/novel-agent/skills"));
}

export function listNovelAgentSessions() {
    return request<{ sessions: NovelAgentSession[] }>(api.get("/novel-agent/sessions"));
}

export function getNovelAgentSessionMessages(sessionId: string) {
    return request<{ session: NovelAgentSession; messages: NovelAgentSessionMessage[] }>(api.get(`/novel-agent/sessions/${encodeURIComponent(sessionId)}/messages`));
}

export function runNovelAgentTurn(input: {
    sessionId?: string;
    bookId?: string;
    mode: string;
    message: string;
    requestedSkills: string[];
    confirmedIntent?: "create_book" | "write_next" | "repair_state" | "revise_chapter" | "create_script" | "create_storyboard" | "short_run" | "draft_next" | "plan_chapter" | "compose_chapter" | "fanfic_init" | "continuation_import" | "spinoff_create" | "style_imitation" | "translation_create" | "interactive_film_create" | "play_start" | "play_step";
    confirmedActionPayload?: Record<string, unknown>;
    model: { provider: "openai" | "anthropic" | "custom"; service?: string; baseUrl: string; apiKey: string; model: string; apiFormat: "chat" | "responses"; temperature: number };
}) {
    return request<NovelAgentTurnResult>(api.post("/novel-agent/turn", input));
}

export function startNovelAgentJob(input: Parameters<typeof runNovelAgentTurn>[0]) {
    return request<{ job: NovelAgentJob }>(api.post("/novel-agent/jobs", input));
}

export function getNovelAgentJob(jobId: string) {
    return request<{ job: NovelAgentJob }>(api.get(`/novel-agent/jobs/${encodeURIComponent(jobId)}`));
}

export function cancelNovelAgentJob(jobId: string) {
    return request<{ ok: boolean }>(api.post(`/novel-agent/jobs/${encodeURIComponent(jobId)}/cancel`));
}

export type NovelAgentJobHistoryItem = {
    id: string;
    status: "queued" | "running" | "succeeded" | "failed";
    stage: string;
    error?: string;
    mode?: string;
    confirmedIntent?: string;
    sessionId?: string;
    bookId?: string;
    resultSummary?: string;
    createdAt: string;
    updatedAt: string;
};

export function listNovelAgentJobHistory() {
    return request<{ jobs: NovelAgentJobHistoryItem[] }>(api.get("/novel-agent/jobs/history"));
}

export function listNovelAgentActiveJobs() {
    return request<{ jobs: NovelAgentJobHistoryItem[] }>(api.get("/novel-agent/jobs/active"));
}

export function renameNovelAgentSession(sessionId: string, title: string) {
    return request<{ ok: boolean }>(api.put(`/novel-agent/sessions/${encodeURIComponent(sessionId)}`, { title }));
}

export function deleteNovelAgentSession(sessionId: string) {
    return request<{ ok: boolean }>(api.delete(`/novel-agent/sessions/${encodeURIComponent(sessionId)}`));
}

export function searchNovelAgentSessions(query: string, page = 1, pageSize = 20) {
    return request<{ sessions: NovelAgentSession[]; total: number; page: number; pageSize: number }>(api.get("/novel-agent/sessions/search", { params: { q: query || undefined, page, page_size: pageSize } }));
}

export type NovelPlayState = { exists: boolean; stateMd?: string; sceneMd?: string; suggestions: string[] };

export function getNovelPlayState(sessionId: string) {
    return request<NovelPlayState>(api.post(`/novel-agent/sessions/${encodeURIComponent(sessionId)}/play-state`));
}
