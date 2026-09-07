import { apiClient, request } from "@/services/api/request";

export type ComfyBridgeSummary = {
    id: string;
    name: string;
    enabled: boolean;
    online: boolean;
    lastSeenAt?: string;
    lastTaskAt?: string;
    capabilities?: Record<string, unknown>;
    createdAt: string;
    updatedAt: string;
};

export type ComfyBridgeRegistration = {
    bridge: ComfyBridgeSummary;
    token: string;
};

export type ComfyBridgeControlSettings = {
    comfyUrl?: string;
    installDir?: string;
    installType?: "auto" | "portable" | "desktop";
    port?: number;
    workflowDir?: string;
};

export type ComfyBridgeControlResult = {
    installDir?: string;
    installType?: string;
    launcher?: string;
    comfyUrl?: string;
    comfyPort?: number;
    comfyOnline?: boolean;
    managed?: boolean;
};

export type ComfyBridgeDirectoryPickerResult = {
    cancelled: boolean;
    path: string;
};

export function listComfyBridges() {
    return request<ComfyBridgeSummary[]>(apiClient.get("/comfy-bridges"));
}

export function createComfyBridge(name: string, capabilities?: Record<string, unknown>) {
    return request<ComfyBridgeRegistration>(apiClient.post("/comfy-bridges", { name, capabilities }));
}

export function revokeComfyBridge(id: string) {
    // 部分生产代理默认拦截 DELETE；后端保留 DELETE 兼容，但管理端统一走显式撤销动作。
    return request<{ revoked: boolean }>(apiClient.post(`/comfy-bridges/${encodeURIComponent(id)}/revoke`));
}

export function controlComfyBridge(id: string, action: "start" | "stop" | "detect", settings: ComfyBridgeControlSettings) {
    return request<ComfyBridgeControlResult>(apiClient.post(`/comfy-bridges/${encodeURIComponent(id)}/comfy-control`, { action, settings }));
}

export function selectComfyBridgeDirectory(id: string, title: string) {
    return request<ComfyBridgeDirectoryPickerResult>(
        apiClient.post(`/comfy-bridges/${encodeURIComponent(id)}/directory-picker`, { title }, { timeout: 120_000 }),
    );
}
