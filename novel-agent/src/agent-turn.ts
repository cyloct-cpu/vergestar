import { createLLMClient, runWorkerAgent, type LLMMessage } from "@actalk/inkos-core";

import { loadNovelSkillGuidance, type NovelSkillSummary } from "./skills.js";

export type NovelAgentTurnInput = {
    message: string;
    history?: Array<{ role: "user" | "assistant"; content: string }>;
    requestedSkills?: string[];
    model: {
        provider: "openai" | "anthropic" | "custom";
        service?: string;
        baseUrl: string;
        apiKey: string;
        model: string;
        apiFormat?: "chat" | "responses";
        temperature?: number;
    };
};

export type NovelAgentTurnResult = {
    text: string;
    skills: NovelSkillSummary[];
    missingSkillIds: readonly string[];
    usage: { promptTokens: number; completionTokens: number; totalTokens: number };
};

const SYSTEM_PROMPT = `你是 Vergestar 的小说创作 Agent。你帮助用户从一个想法开始创作长篇小说、短篇、剧本与分镜。

你当前只处于“讨论与规划”阶段：可以分析创意、提出一个关键问题、给出建书建议和创作计划，但不能声称已经创建、写入、修改、审稿、生成媒体或完成任何产物。高影响操作会由 Vergestar 在后续确认后通过受控工具执行。

输出要求：使用中文；不要使用表情符号；不要虚构工具结果；信息不足时最多提出一个最关键的问题。`;

export async function runNovelAgentTurn(input: NovelAgentTurnInput): Promise<NovelAgentTurnResult> {
    const message = input.message.trim();
    if (!message) throw new Error("message is required");
    if (!input.model.apiKey.trim()) throw new Error("model api key is required");
    const skills = await loadNovelSkillGuidance(input.requestedSkills || []);
    const client = createLLMClient({
        provider: input.model.provider === "anthropic" ? "anthropic" : "custom",
        service: input.model.service || "custom",
        configSource: "env",
        baseUrl: input.model.baseUrl,
        apiKey: input.model.apiKey,
        model: input.model.model,
        temperature: input.model.temperature ?? 0.7,
        thinkingBudget: 0,
        apiFormat: input.model.apiFormat || "chat",
        stream: false,
    });
    const messages: LLMMessage[] = [
        { role: "system", content: skills.guidance ? `${SYSTEM_PROMPT}\n\n${skills.guidance}` : SYSTEM_PROMPT },
        ...(input.history || []).map((item): LLMMessage => ({ role: item.role, content: item.content })),
        { role: "user", content: message },
    ];
    const result = await runWorkerAgent(client, input.model.model, messages, { temperature: input.model.temperature ?? 0.7 });
    return { text: result.content, skills: skills.summaries, missingSkillIds: skills.missingSkillIds, usage: result.usage };
}
