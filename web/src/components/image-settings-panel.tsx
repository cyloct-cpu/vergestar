import { type ReactNode, useState } from "react";
import { ConfigProvider, Switch } from "antd";

import { type CanvasTheme } from "@/lib/canvas-theme";
import { buildImageResolutionOptions, formatImageResolutionSize, imageRatioForSize, imageResolutionChoices, imageResolutionOption, imageSizeForResolution, supportsImageResolutionPresets, type ImageResolutionChoice } from "@/lib/image-resolution-tiers";
import { modelCapabilityConfigFor, normalizeImageValue, shortEdgeImageDimensions, workflowImageCapabilityConfig, type ImageCapabilityConfig, type ImageResolutionTier, type WorkflowVideoFieldLike } from "@/lib/model-capabilities";
import { mergedImageCapabilityConfig } from "@/lib/model-selection";
import { modelOptionName, resolveModelChannel, type AiConfig } from "@/stores/use-config-store";

const qualityOptions = [
    { value: "auto", label: "自动" },
    { value: "high", label: "高" },
    { value: "medium", label: "中" },
    { value: "low", label: "低" },
    { value: "1k", label: "1K" },
    { value: "2k", label: "2K" },
];
const DIMENSION_STEP = 16;

type AspectOption = { value: string; label: string; width: number; height: number; icon: string; size?: string };

const aspectOptions: AspectOption[] = [
    { value: "1:1", label: "1:1", width: 1024, height: 1024, icon: "square" },
    { value: "3:2", label: "3:2", width: 1536, height: 1024, icon: "landscape" },
    { value: "2:3", label: "2:3", width: 1024, height: 1536, icon: "portrait" },
    { value: "4:3", label: "4:3", width: 1360, height: 1024, icon: "landscape" },
    { value: "3:4", label: "3:4", width: 1024, height: 1360, icon: "portrait" },
    { value: "16:9", label: "16:9", width: 1824, height: 1024, icon: "landscape" },
    { value: "2:1", label: "2:1", size: "2048x1024", width: 2048, height: 1024, icon: "landscape" },
    { value: "1:2", label: "1:2", size: "1024x2048", width: 1024, height: 2048, icon: "portrait" },
    { value: "21:9", label: "21:9", size: "2352x1008", width: 2352, height: 1008, icon: "landscape" },
    { value: "9:16", label: "9:16", width: 1024, height: 1824, icon: "portrait" },
    { value: "1:1-2k", label: "1:1(2k)", size: "2048x2048", width: 2048, height: 2048, icon: "square" },
    { value: "16:9-2k", label: "16:9(2k)", size: "2048x1152", width: 2048, height: 1152, icon: "landscape" },
    { value: "9:16-2k", label: "9:16(2k)", size: "1152x2048", width: 1152, height: 2048, icon: "portrait" },
    { value: "16:9-4k", label: "16:9(4k)", size: "3840x2160", width: 3840, height: 2160, icon: "landscape" },
    { value: "9:16-4k", label: "9:16(4k)", size: "2160x3840", width: 2160, height: 3840, icon: "portrait" },
    { value: "auto", label: "auto", width: 0, height: 0, icon: "auto" },
];

type ImageSettingsPanelProps = {
    config: AiConfig;
    workflowFields?: WorkflowVideoFieldLike[];
    onConfigChange: (key: "quality" | "size" | "transparentBackground" | "count" | "vquality", value: string) => void;
    theme: CanvasTheme;
    showTitle?: boolean;
    showCount?: boolean;
    className?: string;
    maxCount?: number;
    quickCount?: number;
};

export function ImageSettingsPanel({ config, workflowFields = [], onConfigChange, theme, showTitle = true, showCount = true, className = "w-[304px] space-y-3 rounded-2xl px-1 py-0.5", maxCount = 15, quickCount = 3 }: ImageSettingsPanelProps) {
    const [snapDimensionToStep, setSnapDimensionToStep] = useState(true);
    const profile = workflowFields.length
        ? workflowImageCapabilityConfig(workflowFields)
        : mergedImageCapabilityConfig(config, config.model || config.imageModel);
    const normalized = normalizeImageValue(profile, config);
    const quality = normalized.quality;
    const transparentBackground = normalized.transparentBackground === "true";
    const effectiveMaxCount = Math.min(maxCount, profile.maxOutputs);
    const count = Math.max(1, Math.min(effectiveMaxCount, Number(normalized.count)));
    const activeSize = normalized.size;
    const pixelSizeValues = profile.size.values.filter((value) => value.trim().toLowerCase() !== "auto");
    const hasResolutionPresets = supportsImageResolutionPresets(profile.size);
    const resolutionOptions = !profile.resolutionTier && hasResolutionPresets ? buildImageResolutionOptions(pixelSizeValues) : [];
    const activeResolution = activeSize === "auto" ? undefined : imageResolutionOption(resolutionOptions, activeSize);
    const activeRatio = activeResolution?.ratio || imageRatioForSize(activeSize);
    const resolutionTier = profile.resolutionTier;
    const resolutionChoices = !resolutionTier && hasResolutionPresets ? imageResolutionChoices(profile.size.values) : [];
    // 只有一个分辨率层级时，分辨率切换器没有实际选择意义；更重要的是不能因此把比例列表裁剪成当前层级的 3 个像素尺寸。
    // 例如历史 `*` 配置恢复为标准值后，虽然包含 1024x1024/1536x1024/1024x1536，实际仍应展示完整的比例和尺寸选项。
    const usesResolutionPicker = Boolean(resolutionTier) || resolutionChoices.length > 1;
    const availableAspects: AspectOption[] = usesResolutionPicker && activeSize === "auto"
        ? []
        : resolutionTier
        ? shortEdgeAspectOptions(profile)
        : usesResolutionPicker && activeResolution
        ? resolutionOptions.filter((item) => item.tier === activeResolution.tier).map((item) => ({ value: item.ratio, label: item.ratio, size: item.size, width: item.width, height: item.height, icon: item.width === item.height ? "square" : item.width > item.height ? "landscape" : "portrait" }))
        : imageAspectOptions(profile);
    const selectedAspect = availableAspects.find((item) => imageOptionValue(profile, item) === activeSize || item.value === activeSize) || availableAspects.find((item) => item.label === activeRatio);
    const activeTier: ImageResolutionTier | undefined = resolutionTier
        ? (resolutionTier.values.includes(config.vquality as ImageResolutionTier) ? config.vquality as ImageResolutionTier : resolutionTier.default)
        : undefined;
    const shortEdgeDimensions = resolutionTier && activeTier && (selectedAspect || availableAspects[0])
        ? shortEdgeImageDimensions(imageOptionValue(profile, selectedAspect || availableAspects[0]), activeTier)
        : undefined;
    const dimensions = shortEdgeDimensions || readSizeDimensions(activeSize, selectedAspect || aspectOptions[0]);
	const activeQualityOptions = profile.quality.values.map((value) => qualityOptions.find((item) => item.value === value) || { value, label: value });
	const priceTiers = imageModelPriceTiers(config);
    const selectAspect = (value: string) => {
        const option = availableAspects.find((item) => item.value === value);
        if (resolutionTier) {
            onConfigChange("size", imageOptionValue(profile, option || availableAspects[0]));
            return;
        }
        onConfigChange("size", option ? imageOptionValue(profile, option) : "auto");
    };
    const selectResolution = (choice: ImageResolutionChoice | ImageResolutionTier) => {
        if (choice === "auto") {
            onConfigChange("size", "auto");
            return;
        }
        if (resolutionTier && resolutionTier.values.includes(choice as ImageResolutionTier)) {
            onConfigChange("vquality", choice);
            const ratio = selectedAspect || availableAspects[0];
            if (ratio && profile.size.parameter === "size") {
                const dimensions = shortEdgeImageDimensions(imageOptionValue(profile, ratio), choice);
                if (dimensions) onConfigChange("size", `${dimensions.width}x${dimensions.height}`);
            }
            return;
        }
        const presetChoice = choice.toLowerCase() as "1k" | "2k" | "4k";
        const ratio = activeRatio || availableAspects[0]?.label;
        const size = imageSizeForResolution(resolutionOptions, presetChoice, ratio) || resolutionOptions.find((item) => item.tier === presetChoice)?.size;
        if (size) onConfigChange("size", size);
    };
    const updateDimension = (key: "width" | "height", value: number | null) => {
        const next = Math.max(1, Math.floor(value || dimensions[key] || 1024));
        const width = key === "width" ? next : dimensions.width;
        const height = key === "height" ? next : dimensions.height;
        onConfigChange("size", `${alignDimension(width, snapDimensionToStep)}x${alignDimension(height, snapDimensionToStep)}`);
    };

    return (
        <ImageSettingsTheme theme={theme}>
            <div
                className={className}
                style={{ color: theme.node.text }}
                onMouseDown={(event) => {
                    event.stopPropagation();
                    if (event.target instanceof HTMLInputElement) return;
                    if (document.activeElement instanceof HTMLInputElement && event.currentTarget.contains(document.activeElement)) document.activeElement.blur();
                }}
            >
                {showTitle ? <div className="text-base font-semibold">图像设置</div> : null}
                {profile.quality.supported ? <div className="space-y-2">
                    <SettingTitle color={theme.node.muted}>{isGrokResolutionQuality(profile) ? "分辨率" : "质量"}</SettingTitle>
                    <div className={`grid gap-1.5 ${activeQualityOptions.length <= 2 ? "grid-cols-[repeat(2,minmax(0,1fr))]" : "grid-cols-[repeat(4,minmax(0,1fr))]}"}`}>
						{activeQualityOptions.map((item) => (
							<OptionPill key={item.value} selected={quality === item.value} disabled={!hasPriceTierForImageSelection(priceTiers, item.value, activeSize)} theme={theme} onClick={() => onConfigChange("quality", item.value)}>
                                {item.label}
                            </OptionPill>
                        ))}
                    </div>
                </div> : null}
                {profile.transparentBackground.supported ? <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                        <SettingTitle color={theme.node.muted}>透明背景</SettingTitle>
                        <div className="mt-1 text-[var(--fs-label)]" style={{ color: theme.node.muted }}>
                            请求模型输出保留 Alpha 通道的 PNG
                        </div>
                    </div>
                    <span title="是否支持透明背景由当前模型接口决定" onMouseDown={(event) => event.stopPropagation()}>
                        <Switch
                            size="small"
                            checked={transparentBackground}
                            onChange={(checked) => onConfigChange("transparentBackground", checked ? "true" : "false")}
                        />
                    </span>
                </div> : null}
                {resolutionTier ? <div className="space-y-2">
                    <SettingTitle color={theme.node.muted}>分辨率</SettingTitle>
                    <div className="grid grid-cols-[repeat(3,minmax(0,1fr))] gap-1.5">
                        {resolutionTier.values.map((choice) => (
                            <OptionPill key={choice} selected={activeTier === choice} theme={theme} onClick={() => selectResolution(choice)}>
                                {choice}
                            </OptionPill>
                        ))}
                    </div>
                </div> : resolutionChoices.length ? <div className="space-y-2">
                    <SettingTitle color={theme.node.muted}>分辨率</SettingTitle>
                    <div className={`grid gap-1.5 ${resolutionChoices.length <= 2 ? "grid-cols-[repeat(2,minmax(0,1fr))]" : resolutionChoices.length === 4 ? "grid-cols-[repeat(4,minmax(0,1fr))]" : "grid-cols-[repeat(3,minmax(0,1fr))]"}`}>
                        {resolutionChoices.map((choice) => (
                            <OptionPill key={choice} selected={choice === "auto" ? activeSize === "auto" : activeResolution?.tier === choice} theme={theme} onClick={() => selectResolution(choice)}>
                                {choice === "auto" ? "自动" : choice.toUpperCase()}
                            </OptionPill>
                        ))}
                    </div>
                </div> : null}
                {profile.size.allowCustom ? <div className="space-y-2">
                    <div className="flex items-center justify-between gap-3">
                        <SettingTitle color={theme.node.muted}>尺寸</SettingTitle>
                        <div className="flex items-center gap-2">
                            <span className="text-xs font-medium" style={{ color: theme.node.muted }}>
                                16倍数对齐
                            </span>
                            <span title="输入完成后自动向上补成 16 的倍数" onMouseDown={(event) => event.stopPropagation()}>
                                <Switch size="small" checked={snapDimensionToStep} onChange={setSnapDimensionToStep} />
                            </span>
                        </div>
                    </div>
                    <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-1.5">
                        <DimensionInput prefix="W" value={dimensions.width} disabled={activeSize === "auto"} theme={theme} alignToStep={snapDimensionToStep} onChange={(value) => updateDimension("width", value)} />
                        <span className="text-sm opacity-45">↔</span>
                        <DimensionInput prefix="H" value={dimensions.height} disabled={activeSize === "auto"} theme={theme} alignToStep={snapDimensionToStep} onChange={(value) => updateDimension("height", value)} />
                    </div>
                </div> : null}
                {availableAspects.length ? <div className="space-y-2">
                    <SettingTitle color={theme.node.muted}>尺寸或比例</SettingTitle>
                    <div className="grid grid-cols-[repeat(4,minmax(0,1fr))] gap-1.5 min-[380px]:grid-cols-[repeat(5,minmax(0,1fr))]">
                        {availableAspects.map((item) => (
                            <button
                                key={item.value}
                                type="button"
                                className="flex h-[52px] cursor-pointer flex-col items-center justify-center gap-0.5 rounded-lg bg-transparent text-[var(--fs-label)] transition-colors hover:brightness-110 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-1"
                                style={{ background: selectedAspect?.value === item.value ? theme.toolbar.activeBg : "transparent", color: theme.node.text, outlineColor: theme.node.muted }}
                                onMouseDown={(event) => event.stopPropagation()}
                                onClick={() => selectAspect(item.value)}
                            >
                                <AspectIcon type={item.icon} width={item.width} height={item.height} color={theme.node.text} />
                                <span className="max-w-full min-w-0 truncate" title={item.label}>{item.label}</span>
                            </button>
                        ))}
                    </div>
                </div> : null}
                {showCount && effectiveMaxCount > 1 ? (
                    <div className="space-y-2">
                        <SettingTitle color={theme.node.muted}>生成张数</SettingTitle>
                        <div className="grid grid-cols-[repeat(4,minmax(0,1fr))] gap-1.5">
                            {Array.from({ length: Math.min(quickCount, effectiveMaxCount) }, (_, index) => index + 1).map((value) => (
                                <OptionPill key={value} selected={count === value} theme={theme} onClick={() => onConfigChange("count", String(value))}>
                                    {value}
                                </OptionPill>
                            ))}
                            <CountInput value={count} quickCount={quickCount} max={effectiveMaxCount} theme={theme} onChange={(value) => onConfigChange("count", String(value || 1))} />
                        </div>
                    </div>
                ) : null}
            </div>
        </ImageSettingsTheme>
    );
}

function imageOptionAllowed(profile: ImageCapabilityConfig, option: AspectOption) {
    if (profile.size.parameter === "none") return false;
    if (profile.size.allowCustom && profile.size.values.length === 0) return true;
    return [option.value, option.size, option.width && option.height ? `${option.width}x${option.height}` : ""].filter(Boolean).some((value) => profile.size.values.includes(String(value)));
}

// 宽高比选项直接取模型配置 values（与创作页面一致），不在白名单里的比例（如 8:1）也能显示。
function imageAspectOptions(profile: ImageCapabilityConfig): AspectOption[] {
    if (profile.size.parameter === "none") return [];
    const values = profile.size.values.filter((value) => value.trim().toLowerCase() !== "auto");
    if (!values.length) return profile.size.allowCustom ? aspectOptions.filter((item) => item.value !== "auto") : [];
    return values.map((value) => {
        const known = aspectOptions.find((item) => (item.size || item.value) === value || item.value === value);
        if (known) return known;
        const parts = ratioParts(value);
        return { value, label: compactImageSizeLabel(value), size: value, width: parts?.width || 0, height: parts?.height || 0, icon: "custom" };
    });
}

function ratioParts(value: string) {
    const normalized = value.trim();
    const pixel = normalized.match(/^(\d+)\s*[x×]\s*(\d+)(?:\s*\([^)]*\))?$/i);
    if (pixel) {
        const divisor = gcd(Number(pixel[1]), Number(pixel[2]));
        return { width: Number(pixel[1]) / divisor, height: Number(pixel[2]) / divisor };
    }
    const ratio = normalized.match(/^(\d+(?:\.\d+)?)\s*[:：/]\s*(\d+(?:\.\d+)?)(?:\s*\([^)]*\))?$/);
    if (!ratio) return undefined;
    return {
        width: Number(ratio[1]),
        height: Math.max(1, Number(ratio[2])),
    };
}

function shortEdgeAspectOptions(profile: ImageCapabilityConfig): AspectOption[] {
    if (profile.size.parameter === "aspect_ratio") {
        return profile.size.values.filter((value) => value.trim().toLowerCase() !== "auto").map((value) => {
            const parts = ratioParts(value);
            return { value, label: compactImageSizeLabel(value), size: value, width: parts?.width || 0, height: parts?.height || 0, icon: iconTypeForRatio(parts) };
        });
    }
    const ratios = new Set<string>();
    const options: AspectOption[] = [];
    for (const value of profile.size.values) {
        const parsed = buildImageResolutionOptions([value])[0];
        if (!parsed || ratios.has(parsed.ratio)) continue;
        ratios.add(parsed.ratio);
        options.push({ value: parsed.ratio, label: parsed.ratio, size: parsed.ratio, width: parsed.width, height: parsed.height, icon: iconTypeForRatio({ width: parsed.width, height: parsed.height }) });
    }
    return options;
}

function iconTypeForRatio(parts: { width: number; height: number } | undefined) {
    if (!parts?.width || !parts.height) return "custom";
    return parts.width === parts.height ? "square" : parts.width > parts.height ? "landscape" : "portrait";
}

function gcd(a: number, b: number): number {
    return b ? gcd(b, a % b) : a;
}

function compactImageSizeLabel(value: string) {
    const normalized = value.trim();
    if (normalized.toLowerCase() === "auto") return "自动";
    return normalized.match(/^(\d+(?:\.\d+)?\s*[:：/]\s*\d+(?:\.\d+)?)(?:\s*\([^)]*\))?$/)?.[1]?.replace(/\s+/g, "") || normalized;
}

function imageOptionValue(profile: ImageCapabilityConfig, option: AspectOption) {
    const candidates = [option.size, option.value, option.width && option.height ? `${option.width}x${option.height}` : ""].filter(Boolean).map(String);
    return candidates.find((value) => profile.size.values.includes(value)) || option.size || option.value || "auto";
}

export function ImageSettingsTheme({ theme, children }: { theme: CanvasTheme; children: ReactNode }) {
    return (
        <ConfigProvider
            theme={{
                token: { colorBgContainer: theme.canvas.background, colorBgElevated: theme.canvas.background, colorBorder: theme.node.stroke, colorPrimary: theme.node.activeStroke, colorText: theme.node.text, colorTextLightSolid: theme.node.panel },
                components: { Button: { defaultBg: theme.canvas.background, defaultBorderColor: theme.node.stroke, defaultColor: theme.node.text } },
            }}
        >
            {children}
        </ConfigProvider>
    );
}

export function imageQualityLabel(value: string) {
    return ({ auto: "自动", high: "高", medium: "中", low: "低", "1k": "1K", "2k": "2K" } as Record<string, string>)[value] || value || "默认";
}

function isGrokResolutionQuality(profile: ImageCapabilityConfig) {
    const values = profile.quality.values.map((item) => item.toLowerCase());
    return values.includes("1k") || values.includes("2k");
}

export function imageSizeLabel(size: string) {
    const resolutionLabel = formatImageResolutionSize(size, buildImageResolutionOptions([size]));
    return resolutionLabel !== size ? resolutionLabel : aspectOptions.find((item) => (item.size || item.value) === size || item.value === size)?.label || size;
}

export function imageResolutionTierLabel(value: string | undefined, profile?: ImageCapabilityConfig) {
    if (!profile?.resolutionTier || !value || !profile.resolutionTier.values.includes(value as ImageResolutionTier)) return "";
    return value;
}

function imageModelPriceTiers(config: AiConfig) {
	const channel = resolveModelChannel(config, config.model || config.imageModel);
	const cost = channel.modelCosts?.find((item) => item.model === modelOptionName(config.model || config.imageModel));
	return cost?.logicalPriceTiers || [];
}

function hasPriceTierForImageSelection(tiers: ReturnType<typeof imageModelPriceTiers>, quality: string, size: string) {
	if (!tiers.length) return true;
	return tiers.some((tier) => {
		const selector = tier.selector || {};
		return (!selector.quality || selector.quality === "*" || selector.quality === quality.toLowerCase()) && (!selector.size || selector.size === "*" || selector.size === size.toLowerCase());
	});
}

function OptionPill({ selected, disabled = false, theme, onClick, children }: { selected: boolean; disabled?: boolean; theme: CanvasTheme; onClick: () => void; children: ReactNode }) {
    return (
        <button
            type="button"
			className="flex h-8 min-w-0 cursor-pointer items-center justify-center overflow-hidden rounded-full px-2 text-xs transition-colors hover:brightness-110 focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-1 disabled:cursor-not-allowed disabled:opacity-40"
			style={{ background: selected ? theme.toolbar.activeBg : "transparent", color: theme.node.text, outlineColor: theme.node.muted }}
			disabled={disabled}
            onMouseDown={(event) => event.stopPropagation()}
            onClick={onClick}
        >
            {children}
        </button>
    );
}

function DimensionInput({ prefix, value, disabled, theme, alignToStep, onChange }: { prefix: string; value: number; disabled: boolean; theme: CanvasTheme; alignToStep: boolean; onChange: (value: number | null) => void }) {
    const commit = (input: HTMLInputElement) => {
        const next = alignDimension(Math.max(1, Math.floor(Number(input.value) || value || 1024)), alignToStep);
        input.value = String(next);
        onChange(next);
    };

    return (
        <label className="flex h-8 overflow-hidden rounded-lg text-xs" style={{ background: theme.toolbar.itemHover, color: theme.node.text, opacity: disabled ? 0.55 : 1 }}>
            <span className="grid w-8 place-items-center" style={{ color: theme.node.muted }}>
                {prefix}
            </span>
            <input
                type="number"
                min={1}
                disabled={disabled}
                className="min-w-0 flex-1 bg-transparent px-2 outline-none [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                defaultValue={value || ""}
                key={`${prefix}-${value}`}
                onBlur={(event) => commit(event.currentTarget)}
                onKeyDown={(event) => {
                    if (event.key === "Enter") event.currentTarget.blur();
                }}
                onMouseDown={(event) => event.stopPropagation()}
            />
        </label>
    );
}

function CountInput({ value, quickCount, max, theme, onChange }: { value: number; quickCount: number; max: number; theme: CanvasTheme; onChange: (value: number | null) => void }) {
    const commit = (input: HTMLInputElement) => {
        const next = Math.max(1, Math.min(max, Math.floor(Number(input.value) || 1)));
        input.value = String(next);
        onChange(next);
    };
    return (
        <label className="flex h-8 overflow-hidden rounded-full text-xs" style={{ background: theme.toolbar.itemHover, color: theme.node.text }}>
            <input
                key={value > quickCount ? `custom-${value}` : "quick"}
                type="number"
                min={1}
                max={max}
                aria-label="自定义生成张数"
                placeholder="输入"
                className="min-w-0 flex-1 bg-transparent px-2 text-center outline-none placeholder:text-current placeholder:opacity-55 [appearance:textfield] [&::-webkit-inner-spin-button]:appearance-none [&::-webkit-outer-spin-button]:appearance-none"
                style={{ color: theme.node.text, WebkitTextFillColor: theme.node.text }}
                defaultValue={value > quickCount ? value : ""}
                onBlur={(event) => commit(event.currentTarget)}
                onKeyDown={(event) => {
                    if (event.key === "Enter") event.currentTarget.blur();
                }}
                onMouseDown={(event) => event.stopPropagation()}
            />
        </label>
    );
}

function AspectIcon({ type, width, height, color }: { type: string; width: number; height: number; color: string }) {
    if (type === "auto") return null;
    const ratio = width > 0 && height > 0 ? width / height : 1;
    const boxWidth = ratio >= 1 ? 22 : Math.max(9, 22 * ratio);
    const boxHeight = ratio >= 1 ? Math.max(9, 22 / ratio) : 22;
    return (
        <span className="grid h-6 w-8 place-items-center">
            <span className="border-2" style={{ width: boxWidth, height: boxHeight, borderColor: color }} />
        </span>
    );
}

function SettingTitle({ children, color }: { children: string; color: string }) {
    return (
        <div className="text-xs font-medium" style={{ color }}>
            {children}
        </div>
    );
}

function readSizeDimensions(size: string, fallback: { width: number; height: number }) {
    const match = size?.match(/^(\d+)x(\d+)$/);
    return {
        width: match ? Number(match[1]) : fallback.width,
        height: match ? Number(match[2]) : fallback.height,
    };
}

function alignDimension(value: number, enabled: boolean) {
    return enabled ? Math.ceil(value / DIMENSION_STEP) * DIMENSION_STEP : value;
}
