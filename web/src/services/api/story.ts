import { apiClient, request } from "@/services/api/request";

const api = apiClient;

export type StoryFoundationView = {
    id: string;
    projectId: string;
    inkosBookId: string;
    agentSessionId?: string;
    title: string;
    sourceHash: string;
    files: Record<string, string>;
    createdAt: string;
    updatedAt: string;
};

export function getStoryFoundation(projectId: string) {
    return request<StoryFoundationView>(api.get(`/story/projects/${encodeURIComponent(projectId)}/foundation`));
}

export type StoryChapterVersion = { id: string; projectId: string; unitId: string; number: number; title: string; content: string; sourceHash: string; createdAt: string };
export type StoryReview = { id: string; projectId: string; unitId: string; chapterVersionId: string; status: string; summary: string; issuesJson: string; createdAt: string };
export type StoryMemory = { id: string; projectId: string; kind: string; content: string; sourceType: string; sourceId?: string; sourceHash: string; createdAt: string; updatedAt: string };
export type StoryBranch = { id: string; projectId: string; baseUnitId: string; baseSourceHash: string; title: string; planJson: string; status: string; createdAt: string; updatedAt: string };
export type StoryScene = {
    id: string;
    projectId: string;
    unitId: string;
    position: number;
    title: string;
    location: string;
    timeOfDay: string;
    action: string;
    dialogue: string;
    emotion: string;
    visualIntent: string;
    characterAssetIdsJson: string;
    sourceHash: string;
    status: string;
    shotId?: string;
    shotIds?: string[];
    createdAt: string;
    updatedAt: string;
};

export function snapshotStoryChapter(projectId: string, unitId: string) {
    return request<{ version: StoryChapterVersion }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/snapshot`));
}

export function listStoryChapterVersions(projectId: string, unitId: string) {
    return request<{ versions: StoryChapterVersion[] }>(api.get(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/versions`));
}

export function restoreStoryChapterVersion(projectId: string, unitId: string, versionId: string) {
    return request<{ unit: { id: string; title: string; sourceText: string } }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/versions/${encodeURIComponent(versionId)}/restore`));
}

export function listStoryReviews(projectId: string, unitId: string) {
    return request<{ reviews: StoryReview[] }>(api.get(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/reviews`));
}

export function createStoryReview(projectId: string, unitId: string, input: { summary: string; issues: unknown[] }) {
    return request<{ review: StoryReview }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/reviews`, input));
}

export function listStoryMemories(projectId: string) {
    return request<{ memories: StoryMemory[] }>(api.get(`/story/projects/${encodeURIComponent(projectId)}/memories`));
}

export function createStoryMemory(projectId: string, input: { kind: string; content: string }) {
    return request<{ memory: StoryMemory }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/memories`, input));
}

export function deleteStoryMemory(projectId: string, memoryId: string) {
    return request<{ id: string }>(api.delete(`/story/projects/${encodeURIComponent(projectId)}/memories/${encodeURIComponent(memoryId)}`));
}

export function listStoryBranches(projectId: string) {
    return request<{ branches: StoryBranch[] }>(api.get(`/story/projects/${encodeURIComponent(projectId)}/branches`));
}

export function createStoryBranch(projectId: string, unitId: string, input: { title: string; plan: unknown }) {
    return request<{ branch: StoryBranch }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/branches`, input));
}

export function listStoryScenes(projectId: string, unitId: string) {
    return request<{ scenes: StoryScene[] }>(api.get(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/scenes`));
}

export function createStoryScene(projectId: string, unitId: string, input: { title: string; location?: string; timeOfDay?: string; action?: string; dialogue?: string; emotion?: string; visualIntent?: string; characterAssetIds?: string[] }) {
    return request<{ scene: StoryScene }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/scenes`, input));
}

export function createStorySceneShot(projectId: string, unitId: string, sceneId: string, input: { title?: string; description?: string; durationMs?: number }) {
    return request<{ shot: { id: string; title: string; unitId: string } }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/chapters/${encodeURIComponent(unitId)}/scenes/${encodeURIComponent(sceneId)}/shots`, input));
}

export function approveInkosChapter(projectId: string, chapterNumber: number) {
    return request<{ ok: boolean }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/inkos/chapters/${chapterNumber}/approve`));
}

export function rejectInkosChapter(projectId: string, chapterNumber: number) {
    return request<{ ok: boolean }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/inkos/chapters/${chapterNumber}/reject`));
}

export type NovelBookSettings = { chapterWordCount?: number; targetChapters?: number; status?: string; chapterReviewMode?: "auto" | "manual" };

export function updateInkosBookSettings(projectId: string, input: NovelBookSettings) {
    return request<{ config: Record<string, unknown> }>(api.put(`/story/projects/${encodeURIComponent(projectId)}/book-settings`, input));
}

export function deleteInkosBook(projectId: string) {
    return request<{ ok: boolean }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/delete-inkos-book`));
}

export function readNovelTruth(projectId: string, path: string) {
    return request<{ content: string }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/truth/read`, { path }));
}

export function writeNovelTruth(projectId: string, path: string, content: string) {
    return request<{ ok: boolean }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/truth/write`, { path, content }));
}

export type NovelExportFile = { path: string; size: number; modifiedAt: string };

export function listNovelExports(projectId: string) {
    return request<{ files: NovelExportFile[] }>(api.get(`/story/projects/${encodeURIComponent(projectId)}/exports`));
}

export function readNovelExport(projectId: string, path: string) {
    return request<{ content: string; encoding: string }>(api.get(`/story/projects/${encodeURIComponent(projectId)}/exports/content`, { params: { path } }));
}

export function evaluateNovelBook(projectId: string) {
    return request<{ eval: Record<string, unknown> }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/eval`));
}

export function runMarketRadar(projectId: string, input: { baseUrl: string; apiKey: string; model: string; apiFormat?: string; temperature?: number }) {
    return request<{ radar: Record<string, unknown> }>(api.post(`/story/projects/${encodeURIComponent(projectId)}/radar`, input));
}
