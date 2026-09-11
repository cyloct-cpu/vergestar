import assert from "node:assert/strict";
import { mkdir, mkdtemp, rm, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

import { readProposedAction } from "../dist/full-agent.js";

test("reads proposed actions when the InkOS role is nested in message", async () => {
    const root = await mkdtemp(path.join(os.tmpdir(), "vergestar-proposal-"));
    const sessionId = "session-1";
    try {
        const directory = path.join(root, ".inkos", "sessions");
        await mkdir(directory, { recursive: true });
        await writeFile(
            path.join(directory, `${sessionId}.jsonl`),
            JSON.stringify({
                type: "message",
                message: {
                    role: "toolResult",
                    details: {
                        kind: "proposed_action",
                        action: "create_book",
                        title: "创建长篇小说",
                        summary: "确认后建书",
                        instruction: "创建这本书",
                        requestedSkills: ["inkos-long-writing"],
                        actionPayload: { createBook: { title: "测试书" } },
                    },
                },
            }) + "\n",
            "utf8",
        );
        assert.deepEqual(await readProposedAction(root, sessionId), {
            action: "create_book",
            title: "创建长篇小说",
            summary: "确认后建书",
            instruction: "创建这本书",
            requestedSkills: ["inkos-long-writing"],
            actionPayload: { createBook: { title: "测试书" } },
        });
    } finally {
        await rm(root, { recursive: true, force: true });
    }
});

test("reads a write-next proposal from the persisted InkOS transcript", async () => {
    const root = await mkdtemp(path.join(os.tmpdir(), "vergestar-write-proposal-"));
    const sessionId = "session-write-next";
    try {
        const directory = path.join(root, ".inkos", "sessions");
        await mkdir(directory, { recursive: true });
        await writeFile(
            path.join(directory, `${sessionId}.jsonl`),
            JSON.stringify({
                type: "message",
                role: "toolResult",
                message: {
                    details: {
                        kind: "proposed_action",
                        action: "write_next",
                        title: "写下一章",
                        summary: "确认后运行 Writer 生产管线",
                        instruction: "写作下一章",
                        requestedSkills: ["inkos-long-writing"],
                        actionPayload: { writeNext: { chapterCount: 1 } },
                    },
                },
            }) + "\n",
            "utf8",
        );
        assert.deepEqual(await readProposedAction(root, sessionId), {
            action: "write_next",
            title: "写下一章",
            summary: "确认后运行 Writer 生产管线",
            instruction: "写作下一章",
            requestedSkills: ["inkos-long-writing"],
            actionPayload: { writeNext: { chapterCount: 1 } },
        });
    } finally {
        await rm(root, { recursive: true, force: true });
    }
});

test("does not return a stale proposal from an earlier request", async () => {
    const root = await mkdtemp(path.join(os.tmpdir(), "vergestar-stale-proposal-"));
    const sessionId = "session-stale-proposal";
    try {
        const directory = path.join(root, ".inkos", "sessions");
        await mkdir(directory, { recursive: true });
        await writeFile(
            path.join(directory, `${sessionId}.jsonl`),
            [
                { type: "request_started", seq: 1 },
                { type: "message", seq: 2, role: "toolResult", message: { details: { kind: "proposed_action", action: "create_book", title: "旧建书卡", summary: "旧卡", instruction: "旧指令" } } },
                { type: "request_started", seq: 3 },
                { type: "message", seq: 4, role: "assistant", message: { role: "assistant", content: [{ type: "text", text: "本轮没有提出生产动作。" }] } },
            ].map((event) => JSON.stringify(event)).join("\n") + "\n",
            "utf8",
        );
        assert.equal(await readProposedAction(root, sessionId), undefined);
    } finally {
        await rm(root, { recursive: true, force: true });
    }
});
