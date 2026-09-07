import { useEffect } from "react";
import type { Dispatch, SetStateAction } from "react";

import { selectComfyBridgeWorkflowForCapability, trackComfyBridgeWorkflowUse } from "@/lib/comfy-bridge-workflows";
import { resolveCanvasWorkflowProvider } from "@/lib/canvas/canvas-workflow";
import { workflowProviderPluginEnabled } from "@/lib/plugins/builtin/workflows";
import { usePluginStore } from "@/stores/use-plugin-store";
import { useEffectiveConfig, useConfigStore, type RunningHubCapability } from "@/stores/use-config-store";
import { CanvasNodeType, type CanvasNodeData, type CanvasNodeMetadata, type CanvasNodeTypeId } from "@/types/canvas";

type UseComfyBridgeLegacyNodeMigrationOptions = {
    enabled: boolean;
    nodes: CanvasNodeData[];
    nodesRef: { current: CanvasNodeData[] };
    setNodes: Dispatch<SetStateAction<CanvasNodeData[]>>;
};

const MEDIA_NODE_CAPABILITIES = new Map<CanvasNodeTypeId, RunningHubCapability>([
    [CanvasNodeType.Image, "image"],
    [CanvasNodeType.Video, "video"],
    [CanvasNodeType.Audio, "audio"],
]);

/**
 * Bridge 配置完成前创建的媒体节点没有工作流标记；加载画布时补一次，
 * 避免用户必须删掉旧节点重建才能走 Bridge。
 */
export function useComfyBridgeLegacyNodeMigration({ enabled, nodes, nodesRef, setNodes }: UseComfyBridgeLegacyNodeMigrationOptions) {
    const effectiveConfig = useEffectiveConfig();
    const runtimeStatuses = usePluginStore((state) => state.runtimeStatuses);
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const configComfyBridge = useConfigStore((state) => state.config.comfyBridge);

    useEffect(() => {
        if (!enabled) return;
        const bridgeReady = workflowProviderPluginEnabled(runtimeStatuses, "comfyui")
            && effectiveConfig.comfyBridge.enabled
            && Boolean(effectiveConfig.comfyBridge.bridgeId.trim());
        if (!bridgeReady) return;

        let changed = false;
        const lastUsedWorkflows = { ...effectiveConfig.comfyBridge.lastUsedWorkflows };
        const nextNodes = nodes.map((node) => {
            const capability = MEDIA_NODE_CAPABILITIES.get(node.type);
            if (!capability || resolveCanvasWorkflowProvider(node.metadata) !== "model") return node;

            const workflow = selectComfyBridgeWorkflowForCapability(effectiveConfig, capability);
            if (!workflow) return node;
            const workflowId = workflow.workflowId.trim();
            if (!workflowId) return node;

            changed = true;
            lastUsedWorkflows[capability] = workflowId;
            const metadata: CanvasNodeMetadata = {
                ...node.metadata,
                generationMode: capability,
                workflowProvider: "comfyui",
                comfyBridgeWorkflowId: workflowId,
                workflowTitle: workflow.title?.trim() || workflowId,
            };
            return { ...node, metadata };
        });

        if (!changed) return;
        nodesRef.current = nextNodes;
        setNodes(nextNodes);
        updateConfig("comfyBridge", {
            ...configComfyBridge,
            lastUsedWorkflows: trackComfyBridgeWorkflowUse(
                { comfyBridge: configComfyBridge },
                "image",
                lastUsedWorkflows.image || configComfyBridge.workflowId.trim(),
            ),
        });
    }, [configComfyBridge, effectiveConfig, enabled, nodes, nodesRef, runtimeStatuses, setNodes, updateConfig]);
}
