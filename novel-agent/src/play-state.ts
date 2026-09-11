import path from "node:path";
import { readFile } from "node:fs/promises";

const workspaceRoot = process.env.VERGESTAR_NOVEL_WORKSPACE_ROOT || path.resolve("data");

function playSessionRoot(userId: string, sessionId: string): string {
    if (!/^[A-Za-z0-9_-]{1,64}$/.test(userId) || !/^[A-Za-z0-9_-]{1,120}$/.test(sessionId)) {
        throw new Error("会话标识无效");
    }
    return path.join(workspaceRoot, userId, sessionId);
}

/** 读取 Play 世界的状态/场景投影与建议行动，供游玩 HUD 展示。 */
export async function readPlayState(userId: string, sessionId: string): Promise<{ exists: boolean; stateMd?: string; sceneMd?: string; suggestions: string[] }> {
    const root = playSessionRoot(userId, sessionId);
    const runDir = path.join(root, "worlds", sessionId, "runs", "main");
    const read = async (file: string): Promise<string> => {
        try {
            return await readFile(path.join(runDir, file), "utf8");
        } catch {
            return "";
        }
    };
    const stateMd = await read(path.join("projections", "state.md"));
    const sceneMd = await read(path.join("projections", "scene.md"));
    const exists = Boolean(stateMd || sceneMd);
    // 建议行动从状态投影提取（【建议前往：X】等行）。
    const suggestions: string[] = [];
    for (const line of stateMd.split("\n")) {
        const match = line.match(/[【\[]\s*建议[^：:]*[：:]\s*([^\】\]]+)\s*[】\]]?/);
        if (match) suggestions.push(match[1].trim());
    }
    return { exists, stateMd, sceneMd, suggestions };
}
