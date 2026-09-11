# InkOS 上游来源与修改记录

## 上游

- 仓库：`https://github.com/Narcooo/inkos`
- 固定提交：`091048383f411eb99948a8764f42b6fd13006f9`
- 复制时间：`2026-09-08`
- 引入范围：`packages/core/src`、`packages/core/genres`、`packages/core/skills`、`packages/core/package.json`。
- 未引入：InkOS Studio UI、CLI/TUI、任何 InkOS 本机项目、模型密钥、`.env`、`.inkos` 运行数据或用户书籍。

## 许可证

上游 `packages/core` 为 `AGPL-3.0-only`。其完整许可证文本保留在 `vendor/INKOS_LICENSE`；任何发布、部署或分发前，必须依照 `tasks/INKOS_AGENT_INTEGRATION_MASTER_PLAN.md` 中的合规决策门完成审核。

## 修改策略

1. `vendor/inkos-core` 尽量保持与上游一致，不在其中混入 Vergestar 业务代码。
2. Vergestar 适配代码放在 `novel-agent/src/`，通过受控接口将 InkOS Agent 工具映射到 Vergestar。
3. 每次上游更新记录来源提交、差异和验证结果。
4. 任何对 vendor 源码的修改都必须在本文件追加日期、文件、原因和上游可回馈性。

## Vergestar 本地修改记录

- `2026-09-11`：`src/agents/short-fiction.ts` — 大纲审核（OutlineReviewer）、草稿审核（DraftReview）、包装（Packaging）三处显式 `maxTokens` 由 4096/8192 提升至 16_384。原因：agnes 系列模型服务端默认开启思考，思考 token 计入输出预算，导致审核/包装流式输出在 4096/8192 预算下被 `finish_reason=length` 截断（实测 2666 字符或 0 字符截断），短篇管线无法完成。与上游 `writer.settle()` 的 maxTokens 修复同一性质，可回馈上游。
- `2026-09-11`：`src/agents/short-fiction.ts` — 大纲生成（createOutline）、初稿写作（writeDraft）、缺章续写（continueDraft）、草稿修订（reviseDraft）四处长输出调用接入既有 `completeLongForm` 续写机制（与 `script-storyboard.ts` 同模式）。原因：自建渠道对单次输出有硬上限（实测 ~8k token，`finish_reason=length`），14 章短篇的大纲/整稿单次调用必然截断；续写机制已在 Script/Storyboard 管线验证有效，可回馈上游。
- `2026-09-11`：`src/agents/writer.ts` — writeChapter/settle 的 `maxTokens` 由 32768 降至 16_384。原因：自建渠道对 agnes-2.5-pro-beta 的单次输出预算上限低于 32768，请求即被拒（0 字符 + finish_reason=length，无 partial 可续写）；16_384 与短篇管线一致且实测可用。上游若服务端模型卡（maxOutput）可靠，可改为按模型卡 clamp，属更优解。
