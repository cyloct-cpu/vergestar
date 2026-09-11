# vergestar × 上游 v1.2.9 合并记录（2026-09-12）

> 本文档自包含，供其他会话/窗口直接调用。涵盖合并范围、冲突解决方案、迁移编号约定、验证结果与遗留事项。

## 1. 结果状态

- 上游 `ddcat-ai/open-ai-canvas` 从分叉点 **v1.2.3.1**（`c08ae9f6`，2026-08-31）前进 **160 个提交**到 **v1.2.9**（`01a2d171`，2026-09-12 发布），变更 1447 个文件（+158,459 / -35,699）。
- 合并成果在本地分支 **`merge/upstream-v1.2.9`**，共 4 个提交：
  1. `6c4bf9b0` — Merge remote-tracking branch 'upstream/main'（26 个冲突全部人工解决）
  2. `c3a41393` — 清理两个文件残留的冲突标记
  3. `af341518` — 编译适配与入口调整（Story 域迁包、迁移重编号、恢复 pages/home、隐藏入口等）
  4. `fd9a9ec1` — 修复合并引入的 3 个前端测试回归
- **`main` 分支未动**（仍在 `1de53a39`）。验收通过后由用户决定合回：
  ```bash
  git checkout main && git merge --ff-only merge/upstream-v1.2.9   # 或普通 merge
  ```

## 2. 数据库迁移编号约定（最重要的长期约定）

双方从 v4 起独立编号且校验和强校验（name + checksum 不符即拒绝启动）。已确定方案：

| 版本段 | 归属 | 说明 |
| --- | --- | --- |
| v1–v3 | 共同基线 | 两边一致 |
| **v4–v12** | vergestar Story/Novel 域 | story_domain_foundation … story_agent_job_history，**保持本地原编号与校验和**，现有库 `backend/data/open_ai_canvas.db`（已在 v12）直接通过校验 |
| **v13–v19** | 上游 v1.2.4–v1.2.9 顺延 | v13=resource_upload_key（上游原 v4）、v14=payment_topup(原v5)、v15=resource_playback_variant(原v6)、v16=asset_library_folders(原v7)、v17=logical_model_active_code(原v8)、v18=channel_presentation(原v9)、v19=creation_runtime(原v10)；**校验和字符串保持上游原值** |

- `CurrentSchemaVersion = 19`。上游的 v6/v7 换位兼容 shim（`migrationsForDatabase` 的 legacy 分支）已移除，简化为直接返回 `schemaMigrations`。
- **下次同步上游时**：上游每新增一个迁移（v11、v12…），都要在 `backend/internal/database/migrations.go` 手动顺延编号（下一个可用号是 v20），函数名用语义命名（如 `migrateXxx`），不得再叫 `migrateSchemaV<N>`（Story 域占用了 V4–V12）。
- 若有数据库是用**上游原生二进制**（编号 v4–v10 已应用）直连分支代码，启动会报“迁移名称不一致”。修复 SQL（先备份）：`UPDATE schema_migrations SET version = version + 9 WHERE version >= 4;`

## 3. 后端架构适配

上游把 `internal/service`、`internal/handler` 拆分为 `internal/app` 领域模块（约 200 个文件），`internal/service` 只剩上游的别名文件（`aliases_types.go` 等，类型/函数全部 `= app.X` 转发）。

本地适配：
- 13 个本地服务文件从 `internal/service/` 迁到 `internal/app/`（`git mv` + `package service`→`package app`）：novel_agent*.go、novel_book_*.go、novel_job_*.go、novel_projection*.go、novel_radar.go、story.go。repository/story.go、handler/story.go、handler/novel_agent*.go 原地不动。
- 新增 `backend/internal/service/aliases_vergestar.go`：handler 层经 `service.XxxRequest` 别名引用的 12 个本地类型转发到 app 包。
- Story/Novel 路由接入上游统一的 `handler.RegisterCanvasAPI`（`backend/internal/handler/api.go` 中 `RegisterPluginRoutes` 之后、projectAPI 分组之前注册 `RegisterNovelAgentRoutes/RegisterNovelAgentJobStream/RegisterStoryRoutes`）。
- `app.Service` 结构体保留本地 `novelAgent *novelAgentClient` 字段；**删除了本地 `mailSender` 字段**——上游已把邮件机制整体迁入 `internal/auth`（auth.Service 自带 mailSender），本地 service/email.go 已被上游删除。
- 本地对 `systemChannelIDFromBaseURL` 的第三方网关路径修复（`channelIDFromProxyTail`）移植到了 `app/provider.go`，对应测试（metaso 用例）已在 `app/provider_test.go`、`app/task_creation_test.go`。

## 4. 前端融合要点（本地功能 × 上游功能的共存方案）

| 冲突文件 | 处理 |
| --- | --- |
| `router.tsx`、`workspace-sidebar-nav.tsx` | 保留本地 /novel、/home 路由与小说侧边栏入口；根路由与 /create 用上游的 CreatePage |
| `user-data-sync.ts` | 上游的资产解析 + repairMissingCanvasAssets 流程 + 本地的 `mergeHydratedCanvasProjects`（登录合并保护带任务凭证的本地生成节点），snapshotAssets 变量两边都要 |
| `use-config-store.ts`（3 处） | 全取本地：字段启用逻辑（media/prompt slot 强制可用）、policyKeys 排除 label、ComfyUI 多能力工作流（capabilities 数组） |
| `canvas-connection-policy.ts` | 本地 Bridge 工作流旁路提前 return + 上游的 acceptedInputKinds/maxInputCount 校验，两段共存 |
| `canvas-node-prompt-panel.tsx`（5 处） | 上游相机控制弹窗 + 本地 workflowFields 版图片设置弹窗；GenerationSourcePicker 取本地 props（组件定义是本地签名）；ComfyUI 视频参数持久化取本地 |
| `image-settings-panel.tsx` | 取上游 `ImageSizePicker` 方案（全应用统一组件），但保留本地 `workflowFields`→`workflowImageCapabilityConfig` 能力配置入口；本地内联分辨率 UI 被上游组件取代 |
| `canvas-generation-consumer.ts` | **恢复上游版 `generationProjectDelta`**：本地旧的内存兜底合并与上游重写的 rebase 语义冲突，导致 5 个持久化测试失败；上游新 rebase 已覆盖本地当年要修的问题 |
| `channel-settings-pane.tsx` | 整体取上游（弹窗式渠道编辑器）；本地“卡片展开/收起”交互被上游取代（本地仅 11 行改动） |
| `model-capabilities.ts` | 双方各有一个 `ImageResolutionTier`（本地大写 "1K"\|“2K”\|“4K”，上游小写来自 image-resolution-tiers）。导入别名 `ImageResolutionTier as ResolutionTierPreset` 消解，本地大写类型保留为本文件的 canonical 类型 |
| `chapters.tsx`、`comfyui-bridge-settings-pane.tsx` | import 并集；chapters 的 Tooltip 统一用上游 `ui/base/tooltip`（AntD API 兼容）；comfy 面板 antd Segmented/Space + ui/base Switch/SegmentedControl 并存 |

## 5. 按用户要求隐藏的第 7 点入口（代码保留，可恢复）

1. `web/src/main.tsx`：`/welcome` 不再分支加载 welcome-application，统一走工作台 bootstrap。
2. `web/src/components/layout/workspace-sidebar-nav.tsx`：移除“品牌首页”`<a href="/welcome">` 链接。
3. `web/src/pages/admin/settings/appearance-settings-page.tsx`：“5. 皮肤主题” `SettingsSectionCard` 用 `{false && (…)}` 包裹。

三处均有 `vergestar:` 注释标记，搜索 `vergestar:` 可找到恢复点。Celadon 自研组件族（上游 ADR-0008）无法剔除——它是创作助手/剪辑/全景等新功能的依赖基底（134 个文件引用）。

## 6. 产品决策差异（与上游故意不一致的地方）

- **保留 `/home` 创作仪表盘**：上游已删除该路由并写了断言 `not.toContain('path: "/home"')` 的测试；分支改写了 `web/test/workspace-route-loading.test.ts` 该用例（改为断言 /home 存在），其余断言不变。
- 上述 welcome/皮肤入口隐藏。
- 迁移编号方案（见 §2）。

## 7. 验证结果（2026-09-12）

| 项 | 结果 |
| --- | --- |
| 后端 `go build ./...` | ✅ 通过 |
| `go test ./internal/database/ ./internal/service/ ./internal/handler/` | ✅ 全过（迁移测试已改写为分支编号场景） |
| `go test ./internal/app/` | 9 个失败，与上游原版**逐一相同**（TestAdminSystemPerformance + 8 个 TestCreation*），上游在 Windows 的固有失败，非回归 |
| 前端 `tsc --noEmit` / `vite build` / `bun run build:bridge` | ✅ 全过 |
| 前端 `bun test` 全套（1786 用例） | 失败 55 个，与上游原版基线**逐一相同**（账号切换/远端同步等环境性失败）；合并曾引入 8 个回归，已全部修复 |
| 数据库升级 | `backend/data/open_ai_canvas.db`（v12）下次启动自动补跑 v13–v19，无需手工操作 |

对比基线方法（供复用）：`git worktree add /tmp/upstream-check upstream/main`，在两边跑同样测试后 `comm` 对比失败清单；用完 `git worktree remove --force` + `git worktree prune`。

## 8. 遗留事项 / 后续建议

1. **运行时验收**：按 `.local/start-vergestar.ps1` 启动全套服务，重点过一遍：小说工作台（Play 闭环）、画布新功能（全景图、相机控制、打光、媒体转换节点）、创作助手 Agent、剪辑时间线、登录后的画布数据迁移（自动补 v13–v19）。
2. 验收通过后将 `merge/upstream-v1.2.9` 合回 `main` 并推送 origin。
3. `pending-test.mdx` 已合并双方清单，上游新功能的待测项在其中。
4. 下游合并上游新版本时：迁移编号顺延（§2）、`migrateSchemaV<N>` 命名避让、channel-settings-pane 等“整取上游”的文件需重新评估本地是否有新改动。

## 9. 环境备忘（本次踩坑）

- `python` 是 WindowsApps 存根，会**静默失败**，脚本一律用 `py`。
- MSYS 会转换 `git show rev:path` 的参数（`/`→`\`、`:`→`;`），须加 `MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL="*"`。
- Go 构建需 MinGW gcc（cgo）：`PATH` 加 `C:\Users\Administrator\AppData\Local\Microsoft\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64\bin`，`GOCACHE`/`GOMODCACHE` 指向 `.local/cache/`；`.gocache-build/`（build:bridge 产生）已加入 .gitignore，勿 `git add -A` 前不检查。
- 前端构建脚本：`bun run build` = build:bridge + tsc --noEmit + vite build；开发用 `bun run dev`（13000 端口）。
