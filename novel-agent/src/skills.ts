import {
    createSkillRegistry,
    loadBuiltinAgentSkills,
    type AgentSkill,
} from "@actalk/inkos-core";

export type NovelSkillSummary = {
    id: string;
    name: string;
    description: string;
    source: string;
};

export async function listNovelSkills(): Promise<NovelSkillSummary[]> {
    const loaded = await loadBuiltinAgentSkills();
    const registry = createSkillRegistry({ skills: loaded.skills });
    return registry.listSkills().map(toSummary);
}

export async function resolveNovelSkills(requestedSkills: string[]): Promise<{ skills: NovelSkillSummary[]; missingSkillIds: readonly string[] }> {
    const loaded = await loadBuiltinAgentSkills();
    const resolution = createSkillRegistry({ skills: loaded.skills }).resolveSkills({ requestedSkills });
    return { skills: resolution.usedSkills.map(toSummary), missingSkillIds: resolution.missingSkillIds };
}

export async function loadNovelSkillGuidance(requestedSkills: string[]): Promise<{ summaries: NovelSkillSummary[]; guidance: string; missingSkillIds: readonly string[] }> {
    const loaded = await loadBuiltinAgentSkills();
    const resolution = createSkillRegistry({ skills: loaded.skills }).resolveSkills({ requestedSkills });
    const guidance = resolution.usedSkills.map((skill) => [
        `## Skill: ${skill.name}`,
        skill.description,
        skill.body,
    ].filter(Boolean).join("\n\n")).join("\n\n");
    return { summaries: resolution.usedSkills.map(toSummary), guidance, missingSkillIds: resolution.missingSkillIds };
}

function toSummary(skill: AgentSkill): NovelSkillSummary {
    return { id: skill.id, name: skill.name, description: skill.description, source: skill.source };
}
