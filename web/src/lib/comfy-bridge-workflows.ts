import type { AiConfig, RunningHubCapability } from "@/stores/use-config-store";
import { normalizeRunningHubCapability } from "@/stores/use-config-store";

export type ComfyBridgeWorkflowLike = {
    workflowId: string;
    title?: string;
    capability?: RunningHubCapability;
    capabilities?: RunningHubCapability[];
};

export function normalizeComfyBridgeWorkflowCapabilities(value: unknown, fallback: RunningHubCapability = "image"): RunningHubCapability[] {
    const values = Array.isArray(value) ? value : value ? [value] : [];
    const normalized = values
        .map((item) => normalizeRunningHubCapability(item))
        .filter((item, index, all) => Boolean(item) && all.indexOf(item) === index);
    return normalized.length ? normalized : [fallback];
}

export function comfyBridgeWorkflowSupports(workflow: ComfyBridgeWorkflowLike, capability: RunningHubCapability): boolean {
    return normalizeComfyBridgeWorkflowCapabilities(
        workflow.capabilities?.length ? workflow.capabilities : workflow.capability,
        capability,
    ).includes(capability);
}

export function selectComfyBridgeWorkflowForCapability(
    config: Pick<AiConfig, "comfyBridge">,
    capability: RunningHubCapability,
): ComfyBridgeWorkflowLike | undefined {
    const workflows = config.comfyBridge.workflows.filter((item) => comfyBridgeWorkflowSupports(item, capability));
    if (!workflows.length) return undefined;
    const currentWorkflowId = config.comfyBridge.workflowId.trim();
    const lastUsedWorkflowId = config.comfyBridge.lastUsedWorkflows?.[capability]?.trim() || "";
    return workflows.find((item) => item.workflowId.trim() === currentWorkflowId)
        || workflows.find((item) => item.workflowId.trim() === lastUsedWorkflowId)
        || [...workflows].sort((left, right) => (left.title || left.workflowId).localeCompare(right.title || right.workflowId, "zh-Hans-CN"))[0];
}

export function trackComfyBridgeWorkflowUse(
    config: Pick<AiConfig, "comfyBridge">,
    capability: RunningHubCapability,
    workflowId: string,
): Partial<Record<RunningHubCapability, string>> {
    return { ...config.comfyBridge.lastUsedWorkflows, [capability]: workflowId.trim() };
}
