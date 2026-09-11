import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import test from "node:test";

import { ensureInkosWorkspace } from "../dist/full-agent.js";

test("creates an isolated InkOS workspace without persisting the API key", async () => {
    const root = await mkdtemp(path.join(os.tmpdir(), "vergestar-novel-agent-"));
    try {
        await ensureInkosWorkspace(root, {
            provider: "custom",
            service: "custom",
            configSource: "env",
            baseUrl: "https://example.test/v1",
            apiKey: "secret-must-not-persist",
            model: "test-model",
            temperature: 0.7,
            thinkingBudget: 0,
            apiFormat: "chat",
            stream: true,
        });
        const config = await readFile(path.join(root, "inkos.json"), "utf8");
        assert.ok(config.includes('"apiKey": ""'));
        assert.ok(!config.includes("secret-must-not-persist"));
    } finally {
        await rm(root, { recursive: true, force: true });
    }
});
