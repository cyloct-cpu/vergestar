import assert from "node:assert/strict";
import test from "node:test";

// Bun 直接执行 TypeScript 测试时需要保留扩展名；生产 tsconfig 不包含 test/。
import { defaultModelCapabilityConfig, normalizeVideoDurationConfig, normalizeVideoValue } from "../src/lib/model-capabilities.ts";

test("switching to MiniMax H3 replaces an unsupported 720p value with 768P", () => {
    const profile = defaultModelCapabilityConfig("minimax-video", "MiniMax-H3").video!;

    assert.deepEqual(normalizeVideoValue(profile, { seconds: "11", ratio: "16:9", resolution: "720" }), {
        seconds: "11",
        ratio: "16:9",
        resolution: "768P",
    });
});

test("infers continuous duration sliders from known video model families", () => {
    expect(defaultModelCapabilityConfig("newapi", "seedance-2.5").video!.duration).toEqual({
        selection: "range",
        min: 1,
        max: 30,
        step: 1,
        default: 5,
    });
    expect(defaultModelCapabilityConfig("newapi", "wan3.0").video!.duration).toEqual({
        selection: "range",
        min: 1,
        max: 30,
        step: 1,
        default: 5,
    });
});

test("migrates a saved consecutive duration enum into a slider range", () => {
    expect(normalizeVideoDurationConfig({ selection: "enum", values: [4, 5, 6, 7], default: 6 })).toEqual({
        selection: "range",
        min: 4,
        max: 7,
        step: 1,
        default: 6,
    });
});
