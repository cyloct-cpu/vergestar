# Novel Agent

Vergestar 的 Novel Agent Bridge 复用 InkOS Core 与 Skills，并将其 Agent/Skill 能力对齐到 Vergestar 的用户、权限、模型渠道、任务、资产、画布和媒体生产体系。

当前目录不是 InkOS Studio，也不读取本机 InkOS 服务、项目文件或密钥。上游来源、许可证和修改策略见 `vendor/INKOS_UPSTREAM.md`。

当前最小 Bridge 仅监听 `127.0.0.1:17421`，并要求 `x-vergestar-novel-agent-token`。已实现 `/health`、`/skills`、`/skills/resolve` 和内部 `/turn`。`/turn` 使用 InkOS Core 的 `runWorkerAgent` 与已选择的 Skill 指导；其模型凭据只允许由 Vergestar 后端传入，浏览器不得直接调用它。
