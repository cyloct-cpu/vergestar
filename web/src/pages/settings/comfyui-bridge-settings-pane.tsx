import { App, Button, Form, Input, Popconfirm, Segmented, Select, Space, Switch } from "antd";
import { Copy, Download, FolderOpen, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { WorkflowGraphEditor } from "@/components/workflow-graph-editor";
import { WorkflowTestWorkbench } from "@/components/workflow-test-workbench";
import { controlComfyBridge, createComfyBridge, listComfyBridges, revokeComfyBridge, selectComfyBridgeDirectory, type ComfyBridgeControlResult, type ComfyBridgeSummary } from "@/services/api/comfy-bridge";
import { mergeWorkflowFieldMappings, normalizeWorkflowFieldMappings, useConfigStore, type ComfyBridgeConfig, type ComfyBridgeWorkflow, type RunningHubCapability, type WorkflowFieldMapping, type WorkflowGraphPreview } from "@/stores/use-config-store";
import { useUserStore } from "@/stores/use-user-store";

type DiscoveredWorkflow = { workflowId: string; title?: string; capabilities?: RunningHubCapability[]; fields?: WorkflowFieldMapping[]; workflowJson?: Record<string, unknown>; workflowGraph?: WorkflowGraphPreview; format?: "api" | "ui" };
type BridgePlatform = "windows" | "linux";
type LinuxArchitecture = "amd64" | "arm64";
type ComfyInstallType = NonNullable<ComfyBridgeConfig["comfyInstallType"]>;

export function ComfyUIBridgeSettingsPane() {
    const { message } = App.useApp();
    const userId = useUserStore((state) => state.user?.id);
    const config = useConfigStore((state) => state.config.comfyBridge);
    const updateConfig = useConfigStore((state) => state.updateConfig);
    const [bridges, setBridges] = useState<ComfyBridgeSummary[]>([]);
    const [bridgeName, setBridgeName] = useState("我的 ComfyUI");
    const [bridgeToken, setBridgeToken] = useState("");
    const [loading, setLoading] = useState(false);
    const [registering, setRegistering] = useState(false);
    const [revoking, setRevoking] = useState(false);
    const [bridgePlatform, setBridgePlatform] = useState<BridgePlatform>(detectBridgePlatform);
    const [linuxArchitecture, setLinuxArchitecture] = useState<LinuxArchitecture>(detectLinuxArchitecture);
    const [workflowJsonText, setWorkflowJsonText] = useState("");
    const [workspaceMode, setWorkspaceMode] = useState<"fields" | "test">("fields");
    const [comfyControlAction, setComfyControlAction] = useState<"start" | "stop" | "detect" | null>(null);
    const [directoryPickerAction, setDirectoryPickerAction] = useState<"install" | "workflow" | null>(null);
    const [comfyControlResult, setComfyControlResult] = useState<ComfyBridgeControlResult | undefined>();

    const selectedBridge = bridges.find((item) => item.id === config.bridgeId);
    const selectedWorkflow = config.workflows.find((item) => item.workflowId === config.workflowId);
    const selectedCapabilities = selectedWorkflow?.capabilities?.length ? selectedWorkflow.capabilities : [config.capability];
    const selectedCapability = selectedCapabilities[0];
    const discovered = useMemo(() => discoveredWorkflows(selectedBridge, config.capability), [config.capability, selectedBridge]);
    const draftWorkflowJson = useMemo(() => parseWorkflowJson(workflowJsonText) || selectedWorkflow?.workflowJson, [selectedWorkflow?.workflowJson, workflowJsonText]);

    const update = (patch: Partial<ComfyBridgeConfig>) => updateConfig("comfyBridge", { ...config, ...patch });
    const bridgeServer = typeof window === "undefined" ? "" : window.location.origin;
    const hasFreshBridgeToken = Boolean(bridgeToken);
    const commandToken = bridgeToken || "注册 Bridge 后生成一次性 Token";
    const bridgeDownloadURL = `${bridgeServer}/OpenAICanvas-ComfyBridge.exe`;
    const linuxBridgeDownloadURL = `${bridgeServer}/OpenAICanvas-ComfyBridge-linux-${linuxArchitecture}`;
    const agentStartCommand =
        bridgePlatform === "linux"
            ? linuxStartCommand(linuxBridgeDownloadURL, bridgeServer, commandToken, config.comfyUrl, config.workflowDir, linuxArchitecture)
            : windowsStartCommand(bridgeDownloadURL, bridgeServer, commandToken, config.comfyUrl, config.workflowDir);
    const comfyUrlError = comfyURLValidationError(config.comfyUrl);

    const reload = async () => {
        if (!userId) return;
        setLoading(true);
        try {
            const items = await listComfyBridges();
            setBridges(items);
            const currentBridgeExists = Boolean(config.bridgeId && items.some((item) => item.id === config.bridgeId));
            // Bridge 重连或页面刷新后，只有一个可用设备时自动恢复选择；工作流配置继续保留。
            if (!currentBridgeExists && !config.bridgeId && items.length === 1) update({ bridgeId: items[0].id });
        } catch (error) {
            message.error(error instanceof Error ? `读取 Bridge 失败：${error.message}` : "读取 Bridge 失败");
        } finally {
            setLoading(false);
        }
    };

    useEffect(() => {
        if (!userId) {
            setBridges([]);
            return;
        }
        void reload();
        // 账号切换时重新读取设备；配置变更由控件直接维护。
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [userId]);

    useEffect(() => {
        setWorkflowJsonText(selectedWorkflow?.workflowJson ? JSON.stringify(selectedWorkflow.workflowJson, null, 2) : "");
    }, [selectedWorkflow?.workflowId, selectedWorkflow?.workflowJson]);

    useEffect(() => {
        // 旧快照可能已经保存了一份缺少语义标签的 fields，不能只在其为空时补拉；
        // 这里用 Bridge 返回的最新 schema 合并用户策略，避免 H3 的时长标签被旧数据卡住。
        const discoveredById = new Map(discovered.map((item) => [item.workflowId, item]));
        const nextWorkflows = config.workflows.map((item) => {
            const found = discoveredById.get(item.workflowId);
            if (!found) return item;
            const capability = item.capabilities?.length === 1 ? item.capabilities[0] : item.capability;
            const mergedFields = found.fields?.length ? mergeWorkflowFieldMappings(item.fields, found.fields, capability) : item.fields;
            const needsFields = Boolean(found.fields?.length) && JSON.stringify(mergedFields) !== JSON.stringify(item.fields);
            const needsJson = !item.workflowJson && Boolean(found.workflowJson);
            const needsGraph = !item.workflowGraph && Boolean(found.workflowGraph);
            if (!needsFields && !needsJson && !needsGraph) return item;
            return {
                ...item,
                ...(needsFields ? { fields: mergedFields } : {}),
                ...(needsJson ? { workflowJson: found.workflowJson } : {}),
                ...(needsGraph ? { workflowGraph: found.workflowGraph } : {}),
            };
        });
        if (JSON.stringify(nextWorkflows) === JSON.stringify(config.workflows)) return;
        updateConfig("comfyBridge", { ...config, workflows: nextWorkflows });
    }, [config, discovered, selectedWorkflow, updateConfig]);

    const register = async () => {
        if (!bridgeName.trim()) return message.warning("请填写 Bridge 名称");
        setRegistering(true);
        try {
            const result = await createComfyBridge(bridgeName.trim());
            setBridges((items) => [result.bridge, ...items]);
            setBridgeToken(result.token);
            update({ bridgeId: result.bridge.id });
            message.success("Bridge 已注册，请立即复制启动命令并运行 Bridge");
        } catch (error) {
            message.error(error instanceof Error ? `注册 Bridge 失败：${error.message}` : "注册 Bridge 失败");
        } finally {
            setRegistering(false);
        }
    };

    const selectWorkflow = (workflowId: string) => {
        const normalizedWorkflowId = workflowId.trim();
        if (!normalizedWorkflowId) {
            update({ workflowId: "" });
            return;
        }
        const found = discovered.find((item) => item.workflowId === normalizedWorkflowId);
        const saved = config.workflows.find((item) => item.workflowId === normalizedWorkflowId);
        const capabilities = found?.capabilities?.length ? found.capabilities : saved?.capabilities?.length ? saved.capabilities : [config.capability];
        const trackedCapability = capabilities.includes(config.capability) ? config.capability : capabilities[0];
        const workflows = saved
            ? config.workflows.map((item) =>
                  item.workflowId === normalizedWorkflowId
                      ? {
                            ...item,
                            ...(found?.fields?.length && !item.fields?.length ? { fields: found.fields } : {}),
                            ...(found?.workflowJson && !item.workflowJson ? { workflowJson: found.workflowJson } : {}),
                            ...(found?.workflowGraph && !item.workflowGraph ? { workflowGraph: found.workflowGraph } : {}),
                        }
                      : item,
              )
            : [
                  ...config.workflows,
                  {
                      workflowId: normalizedWorkflowId,
                      title: found?.title || normalizedWorkflowId,
                      capability: capabilities[0],
                      capabilities,
                      fields: found?.fields || [],
                      ...(found?.workflowJson ? { workflowJson: found.workflowJson } : {}),
                      ...(found?.workflowGraph ? { workflowGraph: found.workflowGraph } : {}),
                  },
              ];
        update({ workflowId: normalizedWorkflowId, capability: trackedCapability, lastUsedWorkflows: { ...config.lastUsedWorkflows, [trackedCapability]: normalizedWorkflowId }, workflows });
    };

    const revokeSelectedBridge = async () => {
        if (!selectedBridge) return;
        setRevoking(true);
        try {
            await revokeComfyBridge(selectedBridge.id);
            setBridges((items) => items.filter((item) => item.id !== selectedBridge.id));
            update({ bridgeId: "", workflowId: "" });
            setBridgeToken("");
            message.success("Bridge 已撤销，该进程将无法继续领取任务");
        } catch (error) {
            message.error(error instanceof Error ? `撤销 Bridge 失败：${error.message}` : "撤销 Bridge 失败");
        } finally {
            setRevoking(false);
        }
    };

    const updateWorkflowCapabilities = (capabilities: RunningHubCapability[]) => {
        const workflowId = config.workflowId.trim();
        const normalizedCapabilities = [...new Set(capabilities)];
        if (!normalizedCapabilities.length) normalizedCapabilities.push(config.capability);
        const primaryCapability = normalizedCapabilities[0];
        const fieldCapability = normalizedCapabilities.length === 1 ? primaryCapability : undefined;
        const workflows = workflowId
            ? config.workflows.map((item) => (item.workflowId === workflowId ? { ...item, capability: primaryCapability, capabilities: normalizedCapabilities, fields: normalizeWorkflowFieldMappings(item.fields, fieldCapability) } : item))
            : config.workflows;
        update({ capability: primaryCapability, workflows });
    };

    const saveWorkflow = () => {
        const workflowId = config.workflowId.trim();
        if (!workflowId) return message.warning("请先选择或填写工作流文件名");
        try {
            const workflowJson = workflowJsonText.trim() ? (JSON.parse(workflowJsonText) as Record<string, unknown>) : undefined;
            if (workflowJson && (typeof workflowJson !== "object" || Array.isArray(workflowJson))) throw new Error("工作流 JSON 必须是对象");
            const current = config.workflows.find((item) => item.workflowId === workflowId);
            const found = discovered.find((item) => item.workflowId === workflowId);
            const next: ComfyBridgeWorkflow = {
                workflowId,
                title: current?.title || found?.title || workflowId,
                capability: selectedCapabilities[0],
                capabilities: selectedCapabilities,
                fields: normalizeWorkflowFieldMappings(current?.fields || found?.fields || [], selectedCapabilities.length === 1 ? selectedCapabilities[0] : undefined),
                ...(workflowJson ? { workflowJson } : {}),
                ...(current?.workflowGraph || found?.workflowGraph ? { workflowGraph: current?.workflowGraph || found?.workflowGraph } : {}),
            };
            update({ workflows: [...config.workflows.filter((item) => item.workflowId !== workflowId), next] });
            message.success("ComfyUI 工作流配置已保存");
        } catch (error) {
            message.error(error instanceof Error ? error.message : "工作流配置格式无效");
        }
    };

    const updateWorkflowFields = (fields: WorkflowFieldMapping[]) => {
        if (!selectedWorkflow) return;
        update({ workflows: config.workflows.map((item) => (item.workflowId === selectedWorkflow.workflowId ? { ...item, fields } : item)) });
    };

    const runComfyControl = async (action: "start" | "stop" | "detect") => {
        if (!selectedBridge) return message.warning("请先选择 Bridge 设备");
        if (!selectedBridge.online) return message.warning("Bridge 离线，无法控制本机 ComfyUI");
        setComfyControlAction(action);
        try {
            const result = await controlComfyBridge(selectedBridge.id, action, {
                comfyUrl: config.comfyUrl,
                installDir: config.comfyInstallDir || "",
                installType: config.comfyInstallType || "auto",
                port: config.comfyPort || 8188,
                workflowDir: config.workflowDir,
            });
            setComfyControlResult(result);
            update({
                comfyUrl: result.comfyUrl ? result.comfyUrl : config.comfyUrl,
                comfyPort: result.comfyPort || config.comfyPort || 8188,
            });
            await reload();
            message.success(action === "start" ? "ComfyUI 已启动" : action === "stop" ? "ComfyUI 已停止" : "ComfyUI 探测完成");
        } catch (error) {
            message.error(error instanceof Error ? `ComfyUI ${action === "start" ? "启动" : action === "stop" ? "停止" : "探测"}失败：${error.message}` : "ComfyUI 控制失败");
        } finally {
            setComfyControlAction(null);
        }
    };

    const chooseComfyInstallDir = async () => {
        if (!selectedBridge) return message.warning("请先选择 Bridge 设备");
        if (!selectedBridge.online) return message.warning("Bridge 离线，无法打开该电脑的系统目录选择器");
        setDirectoryPickerAction("install");
        try {
            const result = await selectComfyBridgeDirectory(selectedBridge.id, "选择 ComfyUI 安装目录");
            if (!result.cancelled && result.path) {
                update({ comfyInstallDir: result.path });
                await controlComfyBridge(selectedBridge.id, "detect", {
                    comfyUrl: config.comfyUrl,
                    installDir: result.path,
                    installType: config.comfyInstallType || "auto",
                    port: config.comfyPort || 8188,
                    workflowDir: config.workflowDir,
                });
                await reload();
                message.success("安装目录已生效");
            }
        } catch (error) {
            message.error(error instanceof Error ? `打开目录选择器失败：${error.message}` : "打开目录选择器失败");
        } finally {
            setDirectoryPickerAction(null);
        }
    };

    const chooseComfyWorkflowDir = async () => {
        if (!selectedBridge) return message.warning("请先选择 Bridge 设备");
        if (!selectedBridge.online) return message.warning("Bridge 离线，无法打开该电脑的系统目录选择器");
        setDirectoryPickerAction("workflow");
        try {
            const result = await selectComfyBridgeDirectory(selectedBridge.id, "选择 ComfyUI 工作流目录");
            if (!result.cancelled && result.path) {
                update({ workflowDir: result.path });
                await controlComfyBridge(selectedBridge.id, "detect", {
                    comfyUrl: config.comfyUrl,
                    installDir: config.comfyInstallDir || "",
                    installType: config.comfyInstallType || "auto",
                    port: config.comfyPort || 8188,
                    workflowDir: result.path,
                });
                await reload();
                message.success("工作流目录已生效，Bridge 已重新扫描");
            }
        } catch (error) {
            message.error(error instanceof Error ? `打开目录选择器失败：${error.message}` : "打开目录选择器失败");
        } finally {
            setDirectoryPickerAction(null);
        }
    };

    return (
        <Form layout="vertical" requiredMark={false}>
            <div className="settings-pane-header">
                <div className="min-w-0">
                    <h2>ComfyUI Bridge</h2>
                    <p>Bridge 主动连接云端，并访问运行 Bridge 的机器能够连接到的任意 ComfyUI 服务；每个账号注册并运行自己的 Bridge，其他账号不能调用你的设备。</p>
                </div>
                <Switch checked={config.enabled} checkedChildren="启用" unCheckedChildren="停用" onChange={(enabled) => update({ enabled })} />
            </div>
            {!userId ? (
                <p className="text-sm text-foreground/60">登录后可注册和管理 Bridge。</p>
            ) : (
                <>
                    <section className="settings-section">
                        <div className="mb-3 flex items-center justify-between gap-3">
                            <h3 className="text-sm font-semibold">Bridge 设备</h3>
                            <Button size="small" icon={<RefreshCw className="size-3.5" />} loading={loading} onClick={() => void reload()}>
                                刷新
                            </Button>
                        </div>
                        <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
                            <Input value={bridgeName} maxLength={80} placeholder="例如：办公室 ComfyUI" onChange={(event) => setBridgeName(event.target.value)} />
                            <Button type="primary" icon={<Plus className="size-4" />} loading={registering} onClick={() => void register()}>
                                注册 Bridge
                            </Button>
                        </div>
                        <div className="mt-3 grid gap-3 lg:grid-cols-2">
                            <Form.Item label="ComfyUI 服务地址" className="mb-0" validateStatus={comfyUrlError ? "error" : undefined} help={comfyUrlError || "可填写本机、局域网或公网地址；必须能从运行 Bridge 的电脑访问。"}>
                                <Input value={config.comfyUrl} placeholder="http://127.0.0.1:8188" onChange={(event) => update({ comfyUrl: event.target.value })} onBlur={(event) => update({ comfyUrl: normalizeComfyURL(event.target.value) })} />
                            </Form.Item>
                            <Form.Item label="工作流目录" className="mb-0" extra="Bridge 从此目录发现 ComfyUI API JSON；也可在下方直接粘贴 JSON。">
                                <Space.Compact className="w-full">
                                    <Input
                                        value={config.workflowDir}
                                        placeholder={bridgePlatform === "linux" ? "/opt/ComfyUI/user/default/workflows" : "D:\ComfyUI\workflows"}
                                        onChange={(event) => update({ workflowDir: event.target.value })}
                                        onBlur={(event) => update({ workflowDir: event.target.value.trim() })}
                                    />
                                    <Button icon={<FolderOpen className="size-3.5" />} loading={directoryPickerAction === "workflow"} disabled={!selectedBridge?.online || directoryPickerAction !== null} title={selectedBridge?.online ? "通过 Bridge 打开系统目录选择器" : "选择 Bridge 设备并确认在线后可使用"} onClick={() => void chooseComfyWorkflowDir()}>
                                        选择
                                    </Button>
                                </Space.Compact>
                            </Form.Item>
                        </div>
                        <div className="mt-4 rounded-lg border border-border/60 bg-foreground/[0.03] p-3">
                            <div className="grid gap-3 lg:grid-cols-3">
                                <Form.Item label="ComfyUI 安装类型" className="mb-0">
                                    <Segmented
                                        block
                                        value={config.comfyInstallType || "auto"}
                                        options={[
                                            { label: "自动", value: "auto" },
                                            { label: "整合包", value: "portable" },
                                            { label: "桌面版", value: "desktop" },
                                        ]}
                                        onChange={(value) => update({ comfyInstallType: value as ComfyInstallType })}
                                    />
                                </Form.Item>
                                <Form.Item label="安装目录" className="mb-0" extra="通过 Bridge 打开系统目录选择器，返回本机完整路径。">
                                    <Space.Compact className="w-full">
                                        <Input
                                            value={config.comfyInstallDir}
                                            placeholder={bridgePlatform === "linux" ? "/opt/ComfyUI" : "D:\\ComfyUI"}
                                            onChange={(event) => update({ comfyInstallDir: event.target.value })}
                                            onBlur={(event) => update({ comfyInstallDir: event.target.value.trim() })}
                                        />
                                        <Button icon={<FolderOpen className="size-3.5" />} loading={directoryPickerAction === "install"} disabled={!selectedBridge?.online || directoryPickerAction !== null} title={selectedBridge?.online ? "通过 Bridge 打开系统目录选择器" : "选择 Bridge 设备并确认在线后可使用"} onClick={() => void chooseComfyInstallDir()}>
                                            选择
                                        </Button>
                                    </Space.Compact>
                                </Form.Item>
                                <Form.Item label="服务端口" className="mb-0">
                                    <Input
                                        inputMode="numeric"
                                        value={String(config.comfyPort || 8188)}
                                        onChange={(event) => update({ comfyPort: Number(event.target.value.replace(/\D/g, "")) || 8188 })}
                                    />
                                </Form.Item>
                            </div>
                            <div className="mt-3 flex flex-wrap items-center gap-2">
                                <Button size="small" icon={<RefreshCw className="size-3.5" />} loading={comfyControlAction === "detect"} disabled={!selectedBridge || !selectedBridge.online} onClick={() => void runComfyControl("detect")}>
                                    探测
                                </Button>
                                <Button size="small" type="primary" loading={comfyControlAction === "start"} disabled={!selectedBridge || !selectedBridge.online || comfyControlAction !== null} onClick={() => void runComfyControl("start")}>
                                    启动 ComfyUI
                                </Button>
                                <Button size="small" danger loading={comfyControlAction === "stop"} disabled={!selectedBridge || !selectedBridge.online || comfyControlAction !== null} onClick={() => void runComfyControl("stop")}>
                                    停止 ComfyUI
                                </Button>
                            </div>
                            <p className="mt-2 text-xs text-foreground/60">
                                {comfyControlResult
                                    ? `状态：${comfyControlResult.comfyOnline ? "在线" : "离线"} · 地址：${comfyControlResult.comfyUrl || config.comfyUrl} · 安装类型：${comfyControlResult.installType || config.comfyInstallType || "auto"}${comfyControlResult.managed ? " · 由 Bridge 管理" : ""}`
                                    : "填写安装信息后，Bridge 可自动探测端口；整合包可由 Bridge 启动，桌面版只提供启动与状态检测。"}
                            </p>
                        </div>
                        <div className="mt-4 rounded-lg border border-border/60 bg-foreground/[0.03] p-3">
                            <div className="flex flex-wrap items-center justify-between gap-2">
                                <div>
                                    <h4 className="text-sm font-semibold">安装 Bridge</h4>
                                    <p className="mt-1 text-xs text-foreground/60">在能访问 ComfyUI 的机器上运行；Windows 使用独立程序，Linux 使用对应架构的原生程序，不需要安装 Node.js、npm 或项目源码。</p>
                                </div>
                                <Segmented
                                    size="small"
                                    value={bridgePlatform}
                                    options={[
                                        { label: "Windows", value: "windows" },
                                        { label: "Linux", value: "linux" },
                                    ]}
                                    onChange={(value) => setBridgePlatform(value as BridgePlatform)}
                                />
                            </div>
                            {bridgePlatform === "linux" ? (
                                <div className="mt-3 flex flex-wrap items-center gap-2 text-xs text-foreground/65">
                                    <span>Linux 架构：</span>
                                    <Segmented
                                        size="small"
                                        value={linuxArchitecture}
                                        options={[
                                            { label: "x64", value: "amd64" },
                                            { label: "ARM64", value: "arm64" },
                                        ]}
                                        onChange={(value) => setLinuxArchitecture(value as LinuxArchitecture)}
                                    />
                                    <span>云服务器通常选择 x64。</span>
                                </div>
                            ) : null}
                            <div className="mt-3 flex flex-wrap gap-2">
                                <Button
                                    size="small"
                                    icon={<Download className="size-3.5" />}
                                    href={bridgePlatform === "linux" ? linuxBridgeDownloadURL : bridgeDownloadURL}
                                    download={bridgePlatform === "linux" ? `OpenAICanvas-ComfyBridge-linux-${linuxArchitecture}` : "OpenAICanvas-ComfyBridge.exe"}
                                >
                                    下载 {bridgePlatform === "linux" ? "Linux Bridge" : "Windows Bridge"}
                                </Button>
                            </div>
                            <div className="mt-3 grid gap-2 text-xs">
                                <div className="rounded-md bg-black/20 px-3 py-2">
                                    <span className="mr-2 text-foreground/55">安装并启动（{bridgePlatform === "linux" ? "Linux Shell" : "PowerShell"}）：</span>
                                    <pre className="mt-2 whitespace-pre-wrap break-all font-mono text-xs leading-5">{hasFreshBridgeToken ? agentStartCommand : "请先点击“注册 Bridge”。一次性 Token 只在注册成功后显示；旧 Token 无法从页面找回。"}</pre>
                                </div>
                                <div className="flex flex-wrap gap-2">
                                    <Button
                                        size="small"
                                        disabled={Boolean(comfyUrlError) || !hasFreshBridgeToken}
                                        icon={<Copy className="size-3.5" />}
                                        onClick={() =>
                                            void navigator.clipboard
                                                .writeText(agentStartCommand)
                                                .then(() => message.success("安装并启动命令已复制"))
                                                .catch(() => message.warning("复制失败，请手动复制"))
                                        }
                                    >
                                        复制安装并启动命令
                                    </Button>
                                </div>
                            </div>
                            <p className="mt-2 text-xs text-foreground/55">每位用户都要登录自己的账号、注册自己的 Bridge，再在能访问 ComfyUI 的机器执行命令。不要把 Bridge Token 发给其他人；目录选择器修改后会立即下发到在线 Bridge，重启 Bridge 时请复制最新启动命令。</p>
                        </div>
                        {bridgeToken ? (
                            <div className="mt-3 space-y-2">
                                <p className="text-xs text-foreground/60">令牌只显示一次，建议复制启动命令后关闭页面。令牌不会写入下载链接。</p>
                                <Input.TextArea value={bridgeToken} readOnly autoSize={{ minRows: 2, maxRows: 4 }} />
                                <Button
                                    size="small"
                                    icon={<Copy className="size-3.5" />}
                                    onClick={() =>
                                        void navigator.clipboard
                                            .writeText(bridgeToken)
                                            .then(() => message.success("令牌已复制"))
                                            .catch(() => message.warning("复制失败，请手动复制"))
                                    }
                                >
                                    复制令牌
                                </Button>
                            </div>
                        ) : null}
                        <Form.Item label="当前 Bridge" className="mb-0 mt-3">
                            <Select
                                className="w-full"
                                value={config.bridgeId || undefined}
                                placeholder="选择已注册 Bridge"
                                options={bridges.map((item) => ({ label: `${item.name} · ${item.online ? "在线" : "离线"}`, value: item.id }))}
                                onChange={(bridgeId) => update({ bridgeId })}
                            />
                        </Form.Item>
                        {selectedBridge ? (
                            <div className="mt-2 flex items-center justify-between gap-3 text-xs text-foreground/60">
                                <span>
                                    Bridge ID：{selectedBridge.id} · {selectedBridge.online ? "在线" : "离线"} · 已发现 {discovered.length} 个工作流
                                </span>
                                <Popconfirm
                                    title="撤销这个 Bridge？"
                                    description="撤销后令牌立即失效，本机 Bridge 将无法继续领取任务。"
                                    okText="撤销"
                                    cancelText="取消"
                                    okButtonProps={{ danger: true, loading: revoking }}
                                    onConfirm={() => revokeSelectedBridge()}
                                >
                                    <Button size="small" type="text" danger loading={revoking} icon={<Trash2 className="size-3.5" />}>
                                        撤销
                                    </Button>
                                </Popconfirm>
                            </div>
                        ) : null}
                    </section>
                    <section className="settings-section mt-4">
                        <h3 className="mb-3 text-sm font-semibold">工作流配置</h3>
                        <div className="grid gap-3 lg:grid-cols-12">
                            <Form.Item label="Bridge 发现的工作流" className="mb-0 lg:col-span-6">
                                <Select
                                    showSearch
                                    className="w-full"
                                    value={config.workflowId || undefined}
                                    placeholder={selectedBridge?.online ? "选择 workflows 目录中的 JSON" : "Bridge 在线后自动显示"}
                                    options={discovered.map((item) => ({ label: item.title || item.workflowId, value: item.workflowId }))}
                                    onChange={selectWorkflow}
                                />
                            </Form.Item>
                            <Form.Item label="工作流用途" className="mb-0 lg:col-span-3">
                                <Select
                                    value={selectedCapabilities}
                                    options={[
                                        { label: "图片", value: "image" },
                                        { label: "视频", value: "video" },
                                        { label: "音频", value: "audio" },
                                    ]}
                                    mode="multiple"
                                    maxTagCount={2}
                                    onChange={(capabilities) => updateWorkflowCapabilities(capabilities as RunningHubCapability[])}
                                />
                            </Form.Item>
                            <Form.Item label="工作流文件名" className="mb-0 lg:col-span-3">
                                <Input value={config.workflowId} placeholder="workflow.json" onChange={(event) => update({ workflowId: event.target.value })} onBlur={(event) => selectWorkflow(event.target.value.trim())} />
                            </Form.Item>
                            <div className="workflow-workspace-tabs lg:col-span-12">
                                <Segmented
                                    block
                                    value={workspaceMode}
                                    options={[
                                        { label: "字段配置", value: "fields" },
                                        { label: "测试画布", value: "test" },
                                    ]}
                                    onChange={(value) => setWorkspaceMode(value as "fields" | "test")}
                                />
                            </div>
                            {workspaceMode === "fields" ? (
                                <>
                                    <div className="lg:col-span-12">
                                        <WorkflowGraphEditor
                                            workflowJson={draftWorkflowJson}
                                            workflowGraph={selectedWorkflow?.workflowGraph}
                                            fields={selectedWorkflow?.fields || []}
                                            disabled={!selectedWorkflow}
                                            onChange={updateWorkflowFields}
                                            emptyDescription="尚未读取到工作流拓扑；请刷新 Bridge 后重新选择工作流"
                                        />
                                    </div>
                                    <details className="workflow-json-details lg:col-span-12" open={!draftWorkflowJson}>
                                        <summary>查看或粘贴 ComfyUI API JSON</summary>
                                        <Form.Item className="mb-0 mt-3" extra="留空时 Bridge 从其 workflows 目录按文件名读取；粘贴后使用同一套工作流拓扑和字段编辑器。">
                                            <Input.TextArea rows={10} spellCheck={false} value={workflowJsonText} onChange={(event) => setWorkflowJsonText(event.target.value)} />
                                        </Form.Item>
                                    </details>
                                    <div className="flex justify-end gap-2 lg:col-span-12">
                                        <Popconfirm title="删除当前工作流配置？" okText="删除" cancelText="取消" onConfirm={() => update({ workflowId: "", workflows: config.workflows.filter((item) => item.workflowId !== config.workflowId) })}>
                                            <Button danger disabled={!selectedWorkflow} icon={<Trash2 className="size-4" />}>
                                                删除
                                            </Button>
                                        </Popconfirm>
                                        <Button type="primary" onClick={saveWorkflow}>
                                            保存工作流
                                        </Button>
                                    </div>
                                </>
                            ) : (
                                <div className="lg:col-span-12">
                                    <WorkflowTestWorkbench
                                        key={`comfyui:${selectedWorkflow?.workflowId || "empty"}`}
                                        provider="comfyui"
                                        workflowId={selectedWorkflow?.workflowId || ""}
                                        title={selectedWorkflow?.title || config.workflowId}
                                        capability={selectedCapability}
                                        fields={selectedWorkflow?.fields || []}
                                        disabled={!selectedWorkflow || !config.bridgeId || !selectedBridge?.online}
                                        disabledReason={!selectedWorkflow ? "请先选择并保存一个 ComfyUI 工作流" : !config.bridgeId ? "请先选择 Bridge 设备" : "当前 Bridge 离线，请在可访问 ComfyUI 的电脑上启动 Bridge"}
                                    />
                                </div>
                            )}
                        </div>
                    </section>
                </>
            )}
        </Form>
    );
}

function discoveredWorkflows(bridge: ComfyBridgeSummary | undefined, fallbackCapability: RunningHubCapability): DiscoveredWorkflow[] {
    const value = bridge?.capabilities ? bridge.capabilities["workflows"] : undefined;
    if (!Array.isArray(value)) return [];
    return value.flatMap((item) => {
        if (typeof item === "string" && item.trim()) return [{ workflowId: item.trim(), title: item.trim() }];
        if (!item || typeof item !== "object") return [];
        const raw = item as Record<string, unknown>;
        const workflowId = String(raw.workflowId || raw.id || raw.fileName || "").trim();
        const rawCapabilities = Array.isArray(raw.capabilities) ? raw.capabilities : raw.capability ? [raw.capability] : [];
        const capabilities = rawCapabilities
            .map((value) => (value === "image" || value === "video" || value === "audio" ? value : fallbackCapability))
            .filter((value, index, all) => all.indexOf(value) === index);
        const fields = normalizeWorkflowFieldMappings(raw.fields, capabilities.length === 1 ? capabilities[0] : undefined);
        const workflowJson = raw.workflowJson && typeof raw.workflowJson === "object" && !Array.isArray(raw.workflowJson) ? (raw.workflowJson as Record<string, unknown>) : undefined;
        const workflowGraph = raw.workflowGraph && typeof raw.workflowGraph === "object" && !Array.isArray(raw.workflowGraph) ? (raw.workflowGraph as WorkflowGraphPreview) : undefined;
        return workflowId ? [{ workflowId, title: String(raw.title || raw.name || workflowId), capabilities, fields, workflowJson, workflowGraph, format: raw.format === "api" ? "api" : "ui" }] : [];
    });
}

function normalizeComfyURL(value: string) {
    const normalized = value.trim().replace(/\/+$/, "");
    return normalized || "http://127.0.0.1:8188";
}

function comfyURLValidationError(value: string) {
    try {
        const parsed = new URL(value.trim());
        if (parsed.protocol !== "http:" && parsed.protocol !== "https:") return "ComfyUI 地址只支持 http:// 或 https://";
        if (!parsed.hostname || parsed.username || parsed.password) return "ComfyUI 地址格式无效，不能在 URL 中携带账号密码";
        return "";
    } catch {
        return "请输入完整的 ComfyUI 地址，例如 http://192.168.1.20:8188";
    }
}

function powerShellArgument(value: string) {
    return `'${value.replace(/'/g, "''")}'`;
}

function shellArgument(value: string) {
    return `'${value.replace(/'/g, "'\\''")}'`;
}

function windowsStartCommand(downloadURL: string, server: string, token: string, comfyURL: string, workflowDir: string) {
    return `$bridgeDir = Join-Path $env:LOCALAPPDATA 'OpenAICanvas'\nNew-Item -ItemType Directory -Force -Path $bridgeDir | Out-Null\n$bridgeFile = Join-Path $bridgeDir 'OpenAICanvas-ComfyBridge.exe'\nInvoke-WebRequest ${powerShellArgument(downloadURL)} -OutFile $bridgeFile\n$bridgeStream = [System.IO.File]::OpenRead($bridgeFile)\ntry { $bridgeHeader0 = $bridgeStream.ReadByte(); $bridgeHeader1 = $bridgeStream.ReadByte() } finally { $bridgeStream.Dispose() }\nif ($bridgeHeader0 -ne 0x4D -or $bridgeHeader1 -ne 0x5A) { throw 'Bridge 下载失败：服务器未返回 Windows 可执行程序，请联系管理员重新部署 Bridge' }\n& $bridgeFile --server ${powerShellArgument(server)} --token ${powerShellArgument(token)} --comfy ${powerShellArgument(comfyURL)} --workflow-dir ${powerShellArgument(workflowDir)}`;
}

function linuxStartCommand(downloadURL: string, server: string, token: string, comfyURL: string, workflowDir: string, architecture: LinuxArchitecture) {
    const bridgeDir = "./openai-canvas-bridge";
    const bridgeFile = `${bridgeDir}/OpenAICanvas-ComfyBridge-linux-${architecture}`;
    return `bridge_dir=${shellArgument(bridgeDir)}\nmkdir -p "$bridge_dir"\ncurl --fail --location ${shellArgument(downloadURL)} --output ${shellArgument(bridgeFile)}\nchmod +x ${shellArgument(bridgeFile)}\n${shellArgument(bridgeFile)} --server ${shellArgument(server)} --token ${shellArgument(token)} --comfy ${shellArgument(comfyURL)} --workflow-dir ${shellArgument(workflowDir)}`;
}

function detectBridgePlatform(): BridgePlatform {
    if (typeof navigator !== "undefined" && /linux/i.test(`${navigator.platform} ${navigator.userAgent}`)) return "linux";
    return "windows";
}

function detectLinuxArchitecture(): LinuxArchitecture {
    if (typeof navigator !== "undefined" && /arm64|aarch64/i.test(`${navigator.platform} ${navigator.userAgent}`)) return "arm64";
    return "amd64";
}

function parseWorkflowJson(value: string) {
    if (!value.trim()) return undefined;
    try {
        const parsed = JSON.parse(value);
        return parsed && typeof parsed === "object" && !Array.isArray(parsed) ? (parsed as Record<string, unknown>) : undefined;
    } catch {
        return undefined;
    }
}
