import { describe, expect, test } from "bun:test";

import { shortEdgeImageDimensions, workflowParameterFields, workflowVideoFieldsFromJson } from "@/lib/model-capabilities";
import { workflowFieldValueError, workflowImageCapabilityConfig, workflowVideoCapabilityConfig } from "@/lib/model-capabilities";
import { mergeWorkflowFieldMappings, normalizeWorkflowFieldMappings } from "@/stores/use-config-store";

describe("工作流字段映射", () => {
    test("中文命名的图片输入会被识别为参考图槽位", () => {
        const fields = normalizeWorkflowFieldMappings([
            { nodeId: "10", fieldName: "图片1", fieldValue: "girl.png", fieldType: "TEXT", enabled: true },
            { nodeId: "11", fieldName: "图片2", fieldValue: "stage.png", fieldType: "TEXT", enabled: true },
        ], "video");

        expect(fields.map((field) => field.source)).toEqual(["referenceImage", "referenceImage"]);
        expect(fields.map((field) => field.sourceIndex)).toEqual([0, 1]);
        expect(fields.map((field) => field.enabled)).toEqual([true, true]);
    });

    test("从 ComfyUI API JSON 推导字段时保留中文媒体槽位", () => {
        const fields = workflowVideoFieldsFromJson({
            "20": { class_type: "LoadImageFromPath", inputs: { 图片1: "first.png" } },
            "21": { class_type: "LoadImageFromPath", inputs: { 图片2: "last.png" } },
        });

        expect(fields.map((field) => field.source)).toEqual(["referenceImage", "referenceImage"]);
        expect(fields.every((field) => field.role === "media")).toBe(true);
        expect(fields.every((field) => field.enabled)).toBe(true);
    });

    test("已保存的空来源在 schema 合并后仍会修复为中文图片槽位", () => {
        const savedFields = [
            { nodeId: "10", fieldName: "图片1", fieldValue: "first.png", fieldType: "TEXT", source: "", sourceAutomatic: false, enabled: true },
            { nodeId: "11", fieldName: "图片2", fieldValue: "last.png", fieldType: "TEXT", source: "", sourceAutomatic: false, enabled: true },
        ];
        const schemaFields = workflowVideoFieldsFromJson({
            "10": { class_type: "LoadImageFromPath", inputs: { 图片1: "first.png" } },
            "11": { class_type: "LoadImageFromPath", inputs: { 图片2: "last.png" } },
        });

        const fields = mergeWorkflowFieldMappings(savedFields, schemaFields, "video");

        expect(fields.map((field) => field.source)).toEqual(["referenceImage", "referenceImage"]);
        expect(fields.map((field) => field.sourceIndex)).toEqual([0, 1]);
    });

    test("旧媒体槽位的禁用状态不会传给 Bridge", () => {
        const savedFields = [
            { nodeId: "114", fieldName: "image", fieldType: "TEXT", source: "referenceImage", role: "internal", enabled: false, imageOrder: 1 },
            { nodeId: "121", fieldName: "image", fieldType: "TEXT", source: "referenceImage", role: "internal", enabled: false, imageOrder: 2 },
        ];
        const schemaFields = workflowVideoFieldsFromJson({
            "114": { class_type: "LoadImageFromPath", inputs: { image: "first.png" } },
            "121": { class_type: "LoadImageFromPath", inputs: { image: "second.png" } },
        });

        const fields = mergeWorkflowFieldMappings(savedFields, schemaFields, "video");

        expect(fields.map((field) => field.role)).toEqual(["media", "media"]);
        expect(fields.map((field) => field.enabled)).toEqual([true, true]);
        expect(fields.map((field) => field.sourceIndex)).toEqual([0, 1]);
    });

    test("Bridge 必填提示词槽位的 internal 状态不会导致画布提示词被过滤", () => {
        const savedFields = [
            {
                id: "105::104::prompt",
                nodeId: "105",
                fieldName: "104::prompt",
                classType: "MiniMaxH3ImageToVideo",
                fieldType: "TEXT",
                fieldValue: "默认提示词",
                source: "prompt",
                sourceAutomatic: false,
                role: "internal",
                required: true,
                safeToOverride: true,
                enabled: false,
            },
        ];

        expect(normalizeWorkflowFieldMappings(savedFields, "video").map((field) => field.enabled)).toEqual([true]);
        expect(mergeWorkflowFieldMappings(savedFields, savedFields, "video").map((field) => field.enabled)).toEqual([true]);
    });

    test("Bridge 图片宽高字段会合成为 1K/2K/4K 像素档", () => {
        const profile = workflowImageCapabilityConfig([
            { nodeId: "156", classType: "EmptySD3LatentImage", fieldName: "width", fieldValue: 1280, fieldType: "NUMBER", source: "width" },
            { nodeId: "156", classType: "EmptySD3LatentImage", fieldName: "height", fieldValue: 720, fieldType: "NUMBER", source: "height" },
        ]);

        expect(profile.size.parameter).toBe("size");
        expect(profile.size.values).toContain("1536x1024");
        expect(profile.size.values).toContain("2496x1664");
        expect(profile.size.values).toContain("3504x2336");
        expect(profile.size.default).toBe("1824x1024");
    });

    test("ResolutionSelector 的百万像素字段合成 MiniMax 分辨率档", () => {
        const profile = workflowVideoCapabilityConfig([
            { nodeId: "115", classType: "ResolutionSelector", fieldName: "aspect_ratio", fieldValue: "16:9 (Widescreen)" },
            { nodeId: "115", classType: "ResolutionSelector", fieldName: "megapixels", fieldValue: 0.4, fieldType: "NUMBER", source: "vquality" },
        ]);

        expect(profile.resolutions).toEqual(["480P", "768P", "1080P"]);
        expect(profile.defaultResolution).toBe("480P");
        expect(profile.ratios).toContain("3:2 (Photo)");
        expect(profile.defaultRatio).toBe("16:9 (Widescreen)");
    });

    test("视频时长字段映射到秒数选项", () => {
        const profile = workflowVideoCapabilityConfig([
            { nodeId: "120", classType: "PrimitiveInt", fieldName: "video_length", fieldValue: 6, fieldType: "NUMBER", source: "duration", min: 1, max: 10, step: 1 },
        ]);

        expect(profile.duration).toEqual({ selection: "range", min: 1, max: 10, step: 1, default: 6 });
    });

    test("H3 的 PrimitiveFloat value 字段通过节点标题映射秒数", () => {
        const fields = workflowVideoFieldsFromJson({
            "214": {
                class_type: "PrimitiveFloat",
                _meta: { title: "Float (Duration)" },
                inputs: { value: 5 },
            },
        });
        const profile = workflowVideoCapabilityConfig(fields, {
            ...workflowVideoCapabilityConfig([]),
            duration: { selection: "enum", values: [5, 10], default: 5 },
        });

        expect(fields[0].label).toBe("Float (Duration) · value");
        expect(profile.duration).toEqual({ selection: "range", min: 4, max: 15, step: 1, default: 5 });
    });

    test("旧工作流字段标签不会覆盖新识别的 H3 时长标题", () => {
        const savedFields = [
            {
                nodeId: "105:111",
                classType: "PrimitiveFloat",
                fieldName: "value",
                fieldValue: 5,
                fieldType: "NUMBER",
                label: "value",
                source: "",
                sourceAutomatic: false,
                enabled: true,
            },
        ];
        const schemaFields = workflowVideoFieldsFromJson({
            "105:111": {
                class_type: "PrimitiveFloat",
                _meta: { title: "Float (duration)" },
                inputs: { value: 5 },
            },
        });

        const fields = mergeWorkflowFieldMappings(savedFields, schemaFields, "video");

        expect(fields[0].label).toBe("Float (duration) · value");
        const duration = workflowVideoCapabilityConfig(fields).duration;
        expect(duration.selection).toBe("range");
        expect(duration.min).toBeLessThanOrEqual(5);
        expect(duration.max).toBeGreaterThanOrEqual(5);
    });

    test("视频时长范围过大时映射为秒数滑条", () => {
        const profile = workflowVideoCapabilityConfig([
            { nodeId: "121", classType: "PrimitiveInt", fieldName: "video_length", fieldValue: 6, fieldType: "NUMBER", source: "duration", min: 1, max: 100, step: 1 },
        ]);

        expect(profile.duration).toEqual({ selection: "range", min: 1, max: 100, step: 1, default: 6 });
    });

    test("生图分辨率选择器按短边档位计算尺寸", () => {
        const fields = [
            { nodeId: "115", classType: "ResolutionSelector", fieldName: "aspect_ratio", fieldValue: "3:2 (Photo)" },
            { nodeId: "115", classType: "ResolutionSelector", fieldName: "megapixels", fieldValue: 1.0, fieldType: "NUMBER", source: "vquality" },
        ];
        const profile = workflowImageCapabilityConfig(fields);

        expect(profile.size.parameter).toBe("aspect_ratio");
        expect(profile.size.allowCustom).toBe(false);
        expect(profile.size.values).toContain("3:2 (Photo)");
        expect(profile.resolutionTier?.values).toEqual(["1K", "2K", "4K"]);
        expect(profile.resolutionTier?.default).toBe("1K");
        expect(shortEdgeImageDimensions("3:2 (Photo)", "1K")).toEqual({ width: 1536, height: 1024 });
        expect(shortEdgeImageDimensions("1:1", "2K")).toEqual({ width: 2048, height: 2048 });
    });

    test("已由专用面板消费的分辨率和比例不会重复进入动态参数列表", () => {
        const fields = [
            { nodeId: "115", classType: "ResolutionSelector", fieldName: "aspect_ratio", fieldValue: "3:2 (Photo)" },
            { nodeId: "115", classType: "ResolutionSelector", fieldName: "megapixels", fieldValue: 1.0, fieldType: "NUMBER", source: "vquality" },
        ];

        expect(workflowParameterFields(fields).map((field) => field.fieldName)).toEqual([]);
    });

    test("百万像素原始值和工作流默认值不会误判为非法选项", () => {
        const field = { nodeId: "115", classType: "ResolutionSelector", fieldName: "megapixels", fieldValue: 0.4, fieldType: "NUMBER", source: "vquality" };

        expect(workflowFieldValueError(field, 0.4)).toBe("");
        expect(workflowFieldValueError(field, "0.9")).toBe("");
        expect(workflowFieldValueError(field, "2.0")).toBe("");
        expect(workflowFieldValueError(field, "720p")).toBe("");
        expect(workflowFieldValueError(field, 999)).toContain("当前值不在允许选项中");
    });
});
