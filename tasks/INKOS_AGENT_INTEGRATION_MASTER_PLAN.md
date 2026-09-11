# InkOS Agent 整合总纲

## 状态

2026-09-09：路线 A 已由项目负责人确认。Writer 第一章已完成真实端到端验收：`write_next` 确认卡、异步 Job、Writer、Auditor/Reviser、状态结算、章节落盘、Vergestar 投影和页面刷新均已验证。`chapters/index.json` 路径校验误判导致的 502 已修复。

2026-09-10 继续开发记录：投影层已从“首次创建”升级为持续同步。`repair_state` 成功后，Vergestar 章节状态会从 `draft` 更新为 `ready`，旧审稿记录会按同一章节版本更新为 InkOS 最新审稿结果，不再残留旧错误面板；`story/chapter_summaries.md` 与 `story/subplot_board.md` 已加入产物导出、后端路径白名单和故事记忆投影。当前仍需补齐多章连续写作、取消/恢复和 Vergestar Task/SSE 统一持久化。

本地运行修复：`vite preview` 曾只读取 `server.proxy`，导致 `/api/*` 回退到旧默认端口 `127.0.0.1:18080` 并在登录页显示 502。已在 `web/vite.config.ts` 增加 `preview.proxy`，且启动 Preview 时必须传入 `VITE_API_PROXY_TARGET=http://127.0.0.1:8080`；当前 `GET http://127.0.0.1:13000/api/auth/session` 已返回 HTTP 200。

本文件是 Vergestar 小说创作功能后续开发的**唯一指导总纲**。后续任务开始前只需读取本文件，并更新相应阶段的状态、验收和决策记录。

### 最新实机验收（2026-09-08～2026-09-09）

Vergestar 内已实际运行一次完整 InkOS `create_book -> Architect -> FoundationReviewer` 建书流程，验收书名为《确认卡修复验收》。真实日志依次出现：

```text
阶段：生成基础设定
阶段：审核基础设定（第1轮）
Foundation review: 78/100 REJECTED
基础设定未通过审核，正在重新生成
阶段：审核基础设定（第2轮）
Foundation review: 76/100 REJECTED
基础设定未通过审核，正在重新生成
Foundation final review: 77/100 ACCEPTED (max retries)
阶段：保存书籍配置
阶段：写入基础设定文件
阶段：初始化控制文档
阶段：创建初始快照
```

对应 InkOS transcript 中已出现真实 `sub_agent(agent="architect")` 与 `book_created`，并生成 `books/确认卡修复验收/book.json`、世界设定、人物矩阵、卷纲、书规、伏笔表和初始快照。该结果证明完整 Harness/Pipeline 已实际接通；**不代表 Phase 3 已完成**，因为产物尚未同步为 Vergestar `Project`、章节、角色资产、故事记忆和版本记录的单一真相。

本轮已完成以下工程增量并通过构建检查：

- 最新 InkOS/​Vergestar 实测报告已合并到本文件，本文件成为唯一指导入口；
- Bridge Job 元数据使用原子 JSON 快照落盘，重启时将未完成任务标记为可重试失败；
- 完成任务增加受用户归属校验的基础设定导出接口；
- 后端新增 `StoryFoundation`（Schema v8）及登录态读取接口；
- 小说工作台新增“故事核心文件”面板，可读取故事基石、卷纲、当前状态、伏笔池、情感弧线等已导入文件；
- Bridge/后端均不向浏览器暴露工作目录、内部令牌或模型密钥。
- Novel Agent 会话已支持数据库恢复：会话列表、消息、Skill 和待确认动作均可在刷新后读取；确认创建后会消费确认动作，避免重复执行。
- 真实浏览器验收发现并修复了历史会话 `skills=null` 导致的小说页黑屏，以及“新会话”被最近历史会话自动覆盖的问题；修复后页面正常渲染，控制台无错误，新会话会显示欢迎语且不显示旧消息。
- Writer 对接底座已增加 `StoryFoundation.agentSessionId`（Schema v10）、Bridge 产物中的 `agentSessionId/chapterIndex`，并支持已有 Vergestar 小说项目接收后续新增或变更章节的增量投影；项目页已出现 Writer 面板代码，等待重新登录后的真实 Agent 规划和执行验收。

已完成的服务端闭环证据：最新 Job 已返回 `succeeded`，数据库已出现对应 `novel Project`、`StoryFoundation`、8 个角色资产和 4 条 `StoryMemory`；`/novel?project=31a69b1fd7b004e6872280a298b64fef` 页面级浏览器验收已通过，实际显示 18 个核心文件、7 个角色文件和故事基础面板。旧的手工小说项目没有 Foundation 时现在按“无基础设定”处理，不再产生无意义 500。

本文件是 Vergestar 小说创作功能后续开发的**唯一指导总纲**。后续任务开始前只需读取本文件，并更新相应阶段的状态、验收和决策记录。

`INKOS_VS_VERGESTAR_REAL_OPERATION_COMPARISON_2026-09-08.md` 是本次调查生成的历史原始记录，不再作为后续开发的必读入口；其关键事实、差异矩阵、完成度和优先级已经合并到本文件。

## 合并后的真实操作对照结论

### InkOS 从 0 创作的实际路径

在运行中的 InkOS Studio 中，新建独立“长篇小说”会话后，只输入自然语言创意，不填写书名、章节和世界观表单。Agent 会先生成结构化创作方案，内容包括：

- 书名、题材、平台、语言、阶段章节数和单章字数；
- 世界观规则、主角压力、核心冲突；
- 前几章的目标、钩子和危机升级；
- 叙事视角、节奏、禁忌和创作约束。

随后生成 `propose_action` 确认卡，用户选择“继续执行”或“取消”。确认后进入独立生产任务，而不是让聊天请求长期阻塞。真实 UI 展示：

```text
建书 · 执行中
○ 生成基础设定
○ 保存书籍配置
○ 写入基础设定文件
○ 初始化控制文档
○ 创建初始快照
```

本次真实对照书为《对照实验：雨夜档案》，输入方向是“夜班档案管理员在注销档案中发现自己的签名”。实测 FoundationReviewer 返回：

```text
Foundation review: 63/100 REJECTED
基础设定未通过审核，正在重新生成...
```

审核维度包括核心冲突、开篇节奏、世界一致性、角色区分度和节奏可行性。此前其他 InkOS 建书还实测过 `68/100 REJECTED`、`78/100 REJECTED`，证明拒绝、重试和质量门禁是稳定机制，不是装饰文案。

### InkOS 建书后的真实工作台

已观察到的持续创作对象和入口：

- 章节列表、章节字数和章节任务状态；
- 主要/次要角色卡；
- 故事基石、卷纲规划、当前状态、伏笔池、情感弧线、支线进度；
- 世界观阅读面板；
- “写下一章”“审计”“导出”“市场雷达”；
- 任务耗时、Skill、参考依据、阶段结果、失败原因和重试入口；
- 会话记录和独立新会话。

已有书籍的真实写作/审稿结果还出现过：

```text
写作 · 都市无敌仙尊 · 失败
专业 Skill inkos-long-writing
本轮参考依据：第 1 章 · sqlite-fts5-bm25
已写出第 1 章，字数 2656，但审稿未通过，状态 audit-failed
```

审稿会识别空间关系、冲突释放、配角功能化、AI 句式、术语密度、伏笔推进和敏感词阻断；第二章还出现过 `state-degraded`。InkOS 的质量语义是“生成文本不等于合格正文”。

### Vergestar 同类输入的真实路径

Vergestar `/novel` 当前已能实际完成：

```text
自然语言想法
→ Novel Agent
→ propose_action 确认卡
→ /api/novel-agent/jobs
→ InkOS runAgentSession
→ sub_agent(agent="architect")
→ PipelineRunner
→ FoundationReviewer
→ 拒绝/重试
→ InkOS 书籍工作目录
```

已实测建书日志：

```text
78/100 REJECTED
76/100 REJECTED
77/100 ACCEPTED (max retries)
保存书籍配置
写入基础设定文件
初始化控制文档
创建初始快照
```

因此，Vergestar 的底层建书 Harness 已经真实接通，不再是手工编辑器，也不再是只调用 `runWorkerAgent` 的假接入。

### 当前差异矩阵（合并版）

| 能力 | InkOS 实测 | Vergestar 当前 | 差距 |
| --- | --- | --- | --- |
| 自然语言起稿 | 完整 | 已有 | 小 |
| 自动理解意图 | 有 | 已有基础能力，但仍受固定模式影响 | 中 |
| Agent 自动 Skill 选择 | `use_skill`，会话内激活 | 主要是左侧手动选择 | 大 |
| 结构化创作方案 | 完整展示世界观、冲突、章节方向 | 主要展示摘要和确认卡 | 大 |
| 生产确认 | 完整 | 已有并实测点击 | 小 |
| 生产任务 | 阶段、耗时、状态、结果 | Bridge Job + 前端轮询，阶段日志已接入 | 中 |
| Architect | 已有 | 已接入并实测 | 小 |
| FoundationReviewer | 多轮评分、拒绝、重试 | 已接入并实测 | 小 |
| 核心文件 | 原生可读面板 | 目前主要在工作目录，基础投影刚开始 | 大 |
| 章节规划 | 章节、状态、依据 | 生产章节已同步为 ProjectUnit；未执行的章节规划仍待完整投影 | 中 |
| 角色资产 | 角色卡、主要/次要、持续使用 | InkOS 角色文件已自动投影为项目角色资产且变化会生成不可变新版本；Script 已有角色自动贯穿场次/镜头，未匹配人物生成待确认候选，确认后立即回填 | 极小 |
| 故事记忆 | 自动整理会话记忆、状态、伏笔、情感弧线 | 第 2 章后已自动同步章节摘要、伏笔池、支线板和状态文件 | 中 |
| Writer | 写作、状态文件、质量门禁 | 已接通 InkOS PipelineRunner；1-20 章连续写作 UI 已实测完成 2 章 | 小到中 |
| Auditor | 审稿问题阻断正史 | Agent Auditor 已接入并产出独立 DB 审稿；仍需审稿结果驱动 Reviser 决策 | 中 |
| Reviser | 根据问题返工 | 已按最新审稿问题触发 `reviseDraft`；第 1 章成功 E2E 已通过：修复 10 项问题、重新审计、落盘、索引/快照更新和 DB 投影 | 小 |
| 版本回溯 | 快照和状态联动 | 人工修改创建版本、版本恢复、正文回读、InkOS 状态快照回退和 DB 状态同步已实机通过；剩余主要是修订版本对比展示 | 极小 |
| 多分支 | 预测、分支选择、分支互动 | StoryBranch 表有，Agent 工具未接入 | 大 |
| 失败恢复 | 降级、复核、重试、继续 | Job 状态已落盘；服务重启可标记可重试失败；前端可取消 running Job 并同步中断 InkOS 会话，实机验收通过 | 中 |
| 会话历史 | 书籍会话、会话记录、新会话 | 页面 React 内存状态为主 | 大 |
| 小说到影视 | Script/Storyboard/互动影游链路 | InkOS 原生 Script 已实机投影为 4 个场次，Storyboard 已实机投影为这 4 场下的 12 镜头；重复同步幂等、任务成功后的媒体回填、镜头修订/资产解绑/章节改稿 stale 传播已聚焦回归；角色/场景/道具参考已接入镜头生成面板；图片上游已真实到达供应商但本次以 524 超时结束 | 小到中 |

### 差距的根本原因

本次同类输入实测证明，差距的主要来源不是模型，而是运行时上下文和产品承接：

1. InkOS 有 Architect、Writer、Auditor、Reviser 等角色分工；
2. InkOS 有 PipelineRunner，将生成、审核、重试和状态结算组织成生产流程；
3. InkOS 有可读可写的故事真相文件；
4. InkOS 将任务状态、耗时、Skill、检索依据、错误和结果呈现在 UI；
5. InkOS 用质量门禁阻止不合格内容直接进入正史；
6. Vergestar 已接通底层建书能力，但尚未完整承接这些生产语义。

### 完成度基线

以下是本次真实操作后记录的工程完成度基线，不是模型质量评分：

- Agent-first 入口：60%～70%；
- 建书 Harness：70%～80%；
- InkOS 风格书籍工作台：25%～35%；
- 连续长篇写作：15%～25%；
- 审稿修订闭环：35%～45%；
- 记忆/版本/分支 Agent 化：20%～30%；
- 小说到剧本、分镜、视频的生产链：35%～50%（Script/Storyboard Pipeline 与场次/镜头投影已实机跑通，视频与互动影游未接）。

### 合并后的优先级

```text
P0 任务/会话持久化、SSE/断线恢复、单写入会话（任务取消已通过首阶段实机验收）
P1 InkOS book.json/核心文件/章节/角色/记忆/快照 → Vergestar 单一真相
P2 InkOS 风格书籍工作台和任务时间线
P3 Writer → Auditor → Reviser → Memory Settlement 连续闭环
P4 小说 → 剧本场次 → 镜头 → 画布 → 图片/视频/音频
```

后续开发不得继续以孤立手工表单替代以上能力；每一阶段必须以真实 Agent 任务、产物、失败语义和可恢复状态验收。

## 目标

将 Vergestar 的“小说创作”重构为 InkOS 风格的 Agent-first 创作工作台：用户只需提供一个想法、约束或已有材料，Novel Agent 负责选择专业 Skill、规划、建书、写作、审稿、修订、记忆结算和影视转化；用户仅在创建、重写、生成媒体等高影响动作前确认。

最终闭环：

```text
想法 / 材料 / 对话
  -> Novel Agent + Skills
  -> 世界观、角色、大纲、章节计划、正文、审稿、记忆和分支
  -> 剧本场次、镜头、画布
  -> 图片、视频、音频任务
```

## 非目标

- 不把 `D:\BYW\NARCOOO` 正在运行的 InkOS Studio 当作 Vergestar 的 HTTP Worker。
- 不从浏览器直接访问 InkOS 的本机端口、文件目录或密钥。
- 不继续以“用户必须先手工填写章节、场次和记忆”为小说创作的主要交互。
- 不将 Skill 当成可绕过权限、确认或任务审计的脚本执行入口。

## 已验证事实

- InkOS 源码位于 `D:\BYW\inkos`；本机部署副本位于 `D:\BYW\NARCOOO`。
- InkOS 当前提交：`091048383f411eb99948a8764f42b6fd13006f9`。
- InkOS 的核心能力位于 `packages/core`，包含 pi-agent Agent Harness、Skill Registry、工具循环、会话恢复、上下文压缩、生产确认和长篇创作管线。
- InkOS Studio 是独立 React/Hono 应用，不应直接复制进 Vergestar Web。
- InkOS 的许可证为 `AGPL-3.0-only`。
- Vergestar 已经具备登录/权限、Go 后端、模型渠道、任务、SSE、资产、镜头、画布和媒体生成能力。

## 运行差异实测（2026-09-08）

已直接检查运行中的 InkOS Studio 建书会话。它并非单轮聊天：Agent 先生成 `propose_action` 确认卡；确认后运行完整建书生产管线，展示“生成基础设定、保存书籍配置、写入基础设定文件、初始化控制文档、创建初始快照”等阶段。实测中基础设定审稿器先后给出 `68/100`、`78/100`，均拒绝并自动重生成。

早期 Vergestar Novel Agent 曾只使用 InkOS Core 的 `runWorkerAgent` 与 Skill 指导，那一版是无工具、无生产确认、无 PipelineRunner、无审稿重试的单轮 Worker。当前路线 A 已切换为 `runAgentSession + PipelineRunner`：自由讨论仍保持只读，确认建书后才打开 Architect 和生产工具，并真实执行审稿重试与基础设定写入。

因此，即使上游模型、温度和用户输入完全相同，输出也会明显不同。模型并不是决定性差异，缺失的运行时闭环才是。

另：截图中当前 InkOS Studio 显示 `BAI · qwen3.8-flash`，而 Vergestar 截图显示 `gpt-5.5`；当前可见选择并不一致。即使统一为同一模型，前述 Agent Harness 差异仍然存在。

## 架构修正：运行时同构优先

路线 A 不再以 `runWorkerAgent` 作为最终 Novel Agent。它只保留为 Bridge 连通性、Skill 注入和无副作用讨论的降级能力。

后续必须接入 vendored InkOS Core 的完整运行链：

```text
runAgentSession
  + PipelineRunner
  + createInteractionToolsFromDeps
  + propose_action
  + confirmed production task
  + Architect / FoundationReviewer / Writer / Auditor / Reviser
  + StateManager / MemoryDB / context transform
```

Vergestar Bridge 的职责是把该运行链绑定到一个受控的每用户、每小说工作目录，并将已提交的书籍、角色、章节、审稿、记忆、场次和产物同步/映射到 Vergestar 数据库、任务、资产和画布。浏览器仍然不直接访问工作目录、Novel Agent 地址或模型密钥。

## 运行时同构门槛

在声称“已复用 InkOS Agent 能力”之前，必须满足：

- [x] Novel Agent 使用 `runAgentSession + PipelineRunner`，`runWorkerAgent` 仅保留为早期只读讨论实现和兼容验证能力。
- [ ] 自由对话可以调用 `use_skill`，并保存本轮已使用 Skill。
- [ ] 建书先生成确认卡；确认后调用真实 `create_book` 工具。
- [ ] 建书实际运行 Architect 与 FoundationReviewer，展示阶段、评分、拒绝原因和重试。
- [x] 完成后有真实的基础设定、角色、大纲、控制文档和初始快照；Vergestar 基础设定映射代码已完成，最终浏览器验收待执行。
- [ ] 每本书只允许一个生产写入会话，取消和重启可恢复，不产生双写；Writer 的 `bookId/sessionId` 绑定已具备，单写入锁和可恢复任务仍待接入 Vergestar Task。

## 决策门：许可证与复用路线

在复制、导入、链接或修改任何 InkOS 源码前，项目负责人必须选择以下其中一条路线并记录选择。

### 路线 A：直接复用 InkOS 核心（推荐的技术路线）

将 InkOS `packages/core` 中 Agent Harness、Skill 系统和小说管线作为 Vergestar 的受控 Node 子系统引入；保留 Vergestar 作为用户、权限、模型、任务、资产和画布的系统所有者。

推荐目录：

```text
vergestar/
  novel-agent/
    vendor/inkos-core/       # 来自 InkOS 的已追踪来源与上游版本说明
    src/
      vergistar-bridge.ts    # 将 InkOS action/tool 映射到 Vergestar 内部 API
      runtime.ts             # Agent、会话和队列生命周期
      server.ts              # 仅本机/容器内的受控接口
```

优点：

- 最大程度复用成熟 Agent、Skill 调度、上下文压缩与生产流程。
- 最接近 InkOS 的用户体验与输出质量。
- 不需要重新猜测长篇写作的复杂行为。

约束：

- InkOS 使用 AGPL-3.0-only。复制、修改或整合该代码并向网络用户提供服务时，可能触发对应源码提供义务。
- 必须保留版权、许可证、NOTICE、上游来源和修改记录。
- 发布前必须由熟悉 AGPL 的法律/合规人员确认 Vergestar 整体和分发方式的义务。
- 不能把“单独子进程”或“放进插件”视为自动规避 AGPL 的方法。

### 路线 B：将 InkOS 作为独立服务

Vergestar 通过受控内部协议调用单独部署的 InkOS 服务。

优点：

- 升级 InkOS 较容易。
- 进程隔离较清楚。

限制：

- 不符合“Vergestar 原生整合”的体验目标。
- 仍需要处理 AGPL 网络交互和修改源码的合规义务。
- 需要两套会话、模型配置和任务恢复机制，用户体验会割裂。

结论：不作为本项目默认路线。

### 路线 C：插件形式

插件只能承担 Vergestar UI 扩展、Skill 清单和工具注册；不能单独承担 InkOS Agent 的 Node 运行时、长任务、会话恢复和写作管线。

结论：插件可以是路线 A 的 UI/Skill 注册层，但不能替代 Novel Agent 后端子系统；且不会自动解决 AGPL 问题。

### 路线 D：干净重写

只参考 InkOS 的公开产品行为与架构思想，不复制任何代码或 Skill 内容，使用 Vergestar 的原生实现重写 Agent Harness。

优点：

- 避免直接引入 AGPL 源码的衍生作品风险。
- 完全控制技术栈和许可证。

缺点：

- 工作量最大。
- 需要重新实现 Agent 会话、Skill 调度、工具循环、上下文压缩、长篇状态和恢复。
- 较难在短期内达到 InkOS 的成熟程度。

## 当前推荐

若项目可接受 AGPL 合规义务，选择 **路线 A**：复用 InkOS `packages/core` 与 `skills`，不复制 Studio UI；以 `novel-agent` 作为 Vergestar 内部 Node 子系统，使用 Vergestar 作为所有业务数据和产品能力的唯一入口。

若项目不能接受 AGPL 义务，选择 **路线 D**，并停止复制/导入 InkOS 代码或 Skill 内容。

在做出路线选择前，仅允许继续进行不涉及 InkOS 源码复制的 Vergestar 原生基础工作。

## 总体架构（路线 A）

```text
Vergestar Web
  -> Novel Agent Workspace
  -> Vergestar Backend API
  -> Novel Agent Bridge (Node)
  -> InkOS Core Agent Harness + Skills
  -> Vergestar Internal Tool API
  -> Project / Story / Asset / Shot / Canvas / Task
```

职责边界：

| 模块 | 责任 |
| --- | --- |
| Vergestar Web | 创作模式入口、会话界面、Skill 展示、确认卡、产物阅读与人工编辑。 |
| Vergestar Backend | 用户/项目权限、任务、数据库、模型渠道、SSE、资产、镜头、画布和媒体生产。 |
| Novel Agent Bridge | InkOS Agent 会话、工具调用适配、上下文构建、Skill 调度、取消与恢复。 |
| InkOS Core | Agent Harness、Skill Registry、长篇写作/审稿/修订方法、上下文压缩和生产动作编排。 |

## 必须保留的安全与产品约束

1. Agent 和 Skill 不获得绕过 Vergestar 权限的能力。
2. 每次工具调用必须绑定 `userId`、`projectId`、`storyProjectId` 和可验证的会话身份。
3. 浏览器不得得到 Novel Agent 的内部地址、模型密钥或任何工作目录路径。
4. 模型密钥仍由 Vergestar 渠道和现有安全边界管理；不可复制 InkOS `.env` 或 `.inkos/secrets.json`。
5. 创建小说、整章重写、采纳分支、批量变更、图片/视频/音频生成是高影响动作，必须通过确认卡。
6. 任务超时、取消和恢复使用 Vergestar Task/SSE 语义；不能让 Agent 口头声称完成。
7. 小说正文、角色、场景、场次、镜头、画布和视频产物必须保留来源 ID 与哈希。

## 对话与 Skill 体验规范

### 创作模式

```text
长篇小说 | 短篇小说 | 剧本创作 | 分镜创作 | 互动影游
同人创作 | 番外创作 | 仿写创作 | 续写创作 | 翻译译介
```

### 会话行为

```text
普通讨论：直接回答，不写入项目。
计划与比较：生成候选，不改变正史。
生产动作：生成确认卡，确认后执行。
高影响修改：先显示影响范围、版本和下游过期内容，再确认。
```

### Skill 行为

```text
@novel-long-writing
@novel-story-review
@novel-script-writing
@novel-storyboard
```

- 用户可显式指定 Skill。
- 未指定时，Agent 根据意图选择一个或少量相关 Skill。
- Skill 仅提供领域规则和静态参考资料；副作用必须通过 Vergestar 工具和确认闸门。
- 不允许按关键词机械加载 Skill。

## 阶段路线

### Phase 0：路线与合规确认

- [x] 选择路线 A：直接复用 InkOS Core 与 Skills。
- [x] 记录上游提交、来源和修改策略，见 `novel-agent/vendor/INKOS_UPSTREAM.md`。
- [ ] 发布前由项目负责人和法律/合规人员确认 AGPL 源码提供方式、整体分发和网络服务义务。

### Phase 1：Agent-first 小说首页

目标：替换当前以表单为中心的 `/novel` 首页。

- [x] 创作模式快捷入口。
- [x] Novel Agent 对话区、当前模型显示与“我的创作”二层入口。
- [x] 首版会话持久化：用户消息、Agent 回复和已使用 Skill 保存到 Vergestar；下一轮会带回历史。
- [x] 会话列表、消息、Skill 和确认动作已持久化并支持刷新恢复；真实浏览器已验证“新会话”与历史会话切换。
- [ ] 流式消息、工具状态、取消和完整会话列表的分页/搜索。
- [x] `propose_action` 结构化确认卡：已由真实 InkOS transcript 解析并在 `/novel` 浏览器中实际点击验证。
- [ ] 流式消息、完整会话列表，以及生产 Job 的 Vergestar Task/SSE 统一持久化/断线恢复；Bridge 已有本地 Job 元数据快照，但不是最终 Task 真相源。

验收：用户输入一句想法后，Agent 能识别为长篇/短篇/剧本等意图，并在不写入数据库的情况下给出计划或请求一个关键补充信息。

### Phase 2：InkOS Harness/Skill 接入（路线 A）或原生替代（路线 D）

- [x] InkOS Core 与内置 Skills 已 vendoring、构建并由 Novel Agent Bridge 实际加载验证；来源、许可证和修改策略见 `novel-agent/vendor/INKOS_UPSTREAM.md`。
- [x] 会话列表与消息的可恢复历史；Job 元数据已具备重启后的显式失败状态，但运行中的模型调用不会自动续跑。
- [ ] 会话队列、上下文压缩和断点续跑。
- [x] Skill Registry 基础：已由 `novel-agent` 从 vendored InkOS Core 实际加载 15 个内置 Skills，并验证显式 Skill 解析和缺失 Skill 报告。
- [ ] `use_skill` 与显式 `@skill` 接入 Vergestar Novel Agent 会话。
- [ ] Agent Tool Registry 与 Vergestar 内部工具适配。
- [ ] Agent 调用 Vergestar 内部工具的身份/权限传递。

> 当前实现已验证确认卡、`sub_agent(agent="architect")` 与完整 Architect/FoundationReviewer 建书 Pipeline；长任务已改为 Bridge 内存 Job + 前端轮询，并显示阶段日志。Bridge 重启仍会丢失内存 Job，且尚未迁移至 Vergestar Task/SSE 持久化语义；在此之前不得把任务恢复能力标记为完成。

验收：一个会话可以使用 `novel-long-writing` 并真实列出已使用 Skill；重启后会话记录可恢复，且不泄露模型密钥。

### Phase 3：从零建书闭环

- [x] `create_story_project` 基础映射：完成的 InkOS 建书产物会幂等创建 Vergestar `novel Project` 与 `StoryFoundation`，并已通过页面级浏览器验收。
- [ ] `build_story_foundation`：基础文件已保存；角色资产、故事记忆自动投影和核心文件浏览器验收已完成；章节索引可导出且后续章节可增量投影，章节计划结构化元数据仍待完整导入。
- [ ] 确认卡与幂等执行。
- [~] StoryProject、ProjectUnit、StoryMemory、角色资产的完整自动持久化；已完成 StoryFoundation、核心文件读取面板、角色/状态/伏笔/情感弧线的来源幂等投影、角色文件变化版本化更新、后续章节增量投影和 InkOS 快照联动，章节计划元数据仍待完整导入。

验收：从一句想法开始，经一次确认得到可打开的小说项目及其基础设定、角色、大纲和第一章计划。

### Phase 4：长篇写作闭环

- [x] `plan_story_chapter`；InkOS `write_next` 动作、`bookId/sessionId` Bridge 契约、异步规划 Job 和项目级 Writer 确认面板已完成真实浏览器规划验收。
- [x] `write_story_chapter`；底层调用 InkOS `PipelineRunner.writeNextChapter`，真实模型任务、正文回写和 Auditor/Reviser 结果已验收。
- [x] 多章连续写作确认卡：1/2/3/5/10 章映射 `PipelineRunner.writeChapters`，实机完成 2 章 E2E；仍缺逐章实时进度、取消/恢复和更完整的 Vergestar Task/SSE。
- [~] 自动版本快照、审稿与记忆结算：第 2 章已实测生成独立 ProjectUnit/ChapterVersion/Review，并同步章节摘要、伏笔池、支线板；人工修改创建版本、恢复版本与 InkOS 状态快照回退已完成实机 E2E；审稿警告驱动 Reviser 修订的首个成功 E2E 也已通过，剩余是修订队列、修订结果对比展示和自动修订决策。
- [x] 任务取消首阶段：可取消 running Job，前端提示正确，并同步中断 InkOS 会话；已通过真实浏览器 E2E。
- [ ] 失败恢复：Vergestar Task/SSE 统一持久化、断线恢复、历史任务列表、逐章进度时间线和取消后一致性检查。

验收：用户要求“继续写第二章”时，Agent 读取相关记忆和计划，生成真实正文、审稿结果和可恢复版本。

### Phase 5：影视生产闭环

- [x] `create_script_scenes`（InkOS Script Pipeline 已实机生成并投影 4 个结构化场次；人物、动作、对白及场景地点均可读）。
- [x] `create_storyboard`（InkOS 原生 Pipeline 已实机跑通并投影 12 个场次/镜头）。
- [~] 角色/场景资产引用：Script 场次会自动匹配已有角色卡，并把主版本绑定为 Shot Reference；未匹配人物会生成待确认候选，确认后即时回填对应场次/镜头；镜头生成面板支持手选角色/场景/道具/素材并传递可用图片；自动场景/道具匹配和角色视觉素材实机生成仍待做。
- [~] `StoryScene -> Shot -> Canvas -> Task` 来源链（4 个 Script 场次到 12 个 Storyboard Shot 的层级和重复同步已实机通过；Canvas/Task 确认与成功回填契约已聚焦回归；图片上游请求已真实抵达供应商，但本次以 524 超时结束）。
- [x] 下游 stale 传播（镜头修订、资产解绑、章节改稿三个触发点均已聚焦回归通过）。

验收：用户确认“将第三章转为分镜”后，系统创建可追溯场次、镜头和画布；视频生成必须由用户确认发起。

### Phase 6：扩展创作模式

- [ ] 同人、番外、续写、仿写。
- [ ] 互动影游和开放世界。
- [ ] 翻译译介。
- [ ] 市场研究、封面和去 AI 味 Skill。

## 当前已完成的原生基础

当前 Vergestar 已实现但尚未被 Novel Agent 调度的基础：

```text
小说项目与章节
章节版本快照/恢复
审稿记录
故事记忆
未来分支
剧本场次
场次到镜头的来源链接
既有资产、画布和媒体任务体系
```

这些是 Phase 3-5 的工具和产物层，不是最终主交互。

## 验证命令

```powershell
cd D:\BYW\vergestar\web
npm.cmd run typecheck
node.exe node_modules/vite/bin/vite.js build

cd D:\BYW\vergestar\backend
$env:CC = 'D:\BYW\msys64\ucrt64\bin\clang.exe'
$env:Path = 'D:\BYW\msys64\ucrt64\bin;' + $env:Path
$env:CGO_ENABLED = '1'
go test ./internal/database ./internal/handler ./internal/service
```

## 开发纪律

- 每一阶段先写/更新专属规格和任务，再修改代码。
- 每个生产工具先做权限、输入、幂等和失败语义，再接模型调用。
- 每个模型动作通过确认卡和 Vergestar Task 执行；不在页面加载或聊天讨论时隐式计费。
- 不复制 InkOS Studio UI；仅在路线 A 批准后按上游提交复制必要核心和 Skill，保留许可证和修改记录。
- 本文件与 `tasks/plan.md` 不一致时，以本文件为准，并在下一次编辑时修复 `tasks/plan.md`。


## 最新实机验收记录：Writer 第一章

- 项目：`31a69b1fd7b004e6872280a298b64fef`；InkOS book：`vergestar同步验收`；Agent session：`7f0f95a16782b4b5613147bb9f52e70c`。
- 规划：项目级 Writer 入口创建异步规划 Job；InkOS 读取章节索引、故事核心文件、角色卡和伏笔上下文，生成 `write_next` 确认卡；确认卡不是历史 `create_book` 或 `draft_structure`。
- 生产：确认后真实调用 `sub_agent(agent=writer)`；Writer 生成 821 字初稿；Auditor 评分 87，Reviser 修复后 91；状态结算发现一致性警告，自动重试后以 `state-degraded` 保守状态落盘。
- 产物：`chapters/0001_架上签名.md`，最终 812 字；`chapters/index.json` 有第 1 章；StoryFoundation 文件数由 18 增至 20。
- Vergestar 投影：1 个章节 ProjectUnit、1 个 StoryChapterVersion、1 条 StoryReview；浏览器刷新后显示“1 章已建立”、章节正文、版本 1 和审稿问题。
- 修复：`validNovelArtifactPath` 先匹配 `chapters/*.md`，误拒绝 `chapters/index.json`，造成成功任务同步时 502；现已先放行精确路径 `chapters/index.json`，并加入回归断言。
- 运行：`.local/start-vergestar.ps1` 默认端口统一为 `8080/13000`，自动注入 Novel Agent 工作目录、Job 目录、端口和令牌；`.local/stop-vergestar.ps1` 会一并停止 Novel Agent。

## 2026-09-09 repair_state 实机验收与静默失败修复

- 触发：第一章写作后 `chapters/index.json` 为 `state-degraded`，UI 显示“本章状态需要修复”。
- 修复前 Bug：确认回合内工具返回 `isError: true`，最终 assistant 消息为空，Bridge 只检查 `result.errorMessage`，导致 Job 被误标为 `succeeded`，前端无错误提示。
- 已修复：`full-agent.ts` 在确认回合扫描扁平与嵌套 toolResult 的 `isError/content`，任一错误即抛出 `InkOS 生产工具执行失败`；同时 `writer.settle()` 的 Observer/Settler 两阶段显式设置 `maxTokens: 32768`。
- 修复后重跑：先创建规划 Job，再点击“确认修复状态”；执行链依次到达“阶段 2b：把观察结果回写到真相文件”、“阶段：审计第1章”，最终 Job `succeeded`。
- 验收结果：
  - 正文 `chapters/0001_架上签名.md` SHA256 保持 `43F023A5FF3B6096617F57898E7AC7C7C4DCF44E336362D3E8C9A91043C6D45C` 不变；
  - `chapters/index.json` 从 `state-degraded` 变为 `ready-for-review`，审计问题被重建为 8 条新审稿提示；
  - `story/current_state.md`、`story/pending_hooks.md`、`story/chapter_summaries.md`、`story/emotional_arcs.md`、`story/subplot_board.md`、`story/character_matrix.md` 均已重建/更新；
  - 页面刷新后“本章状态需要修复”面板消失；
  - InkOS Job token usage：prompt 59,573 / completion 113,359 / total 172,932。
- Bridge 回归：`npm test` 5/5 通过。


## 2026-09-10 多章连续写作第一步

- 已确认 InkOS Core 原生具备 `PipelineRunner.writeChapters(bookId, chapterCount, options)`，支持 1-20 章连续写作；`agent-tools.ts` 会根据 `actionPayload.writeNext.chapterCount` 自动切换到批量链路，并在每章完成后产生进度输出；若最后一章进入 `state-degraded`，批量写作会保守停止。
- Vergestar 确认卡现在会展示 `writeNext.chapterCount`，并允许用户在确认执行前选择 1、2、3、5、10 章；确认请求会把用户选择写回 `confirmedActionPayload.writeNext.chapterCount`，沿用既有 `startNovelAgentJob` 契约，不需要改动后端或新增 Bridge 接口。
- 2026-09-10 实机进度：第 2 章草稿完成，正文 933 字；Job 进入“阶段 2a：提取第 2 章事实”，说明 Writer 主体和质量门禁链路正在推进。
- 2026-09-10 浏览器实机选择 2 章，确认执行成功，生产 Job `2ecb1680-9ef9-4d8b-ad96-cb2d96b5890d` 已进入 running，初始阶段为“启动 InkOS Agent Harness”。
- 2026-09-10 修复前端多章选择联调的 3 个 TypeScript 错误：为 `NovelAgentConfirmationActionPayload` 增加 `writeNext.chapterCount` 专用类型，`NovelAgentConfirmation.actionPayload` 使用该类型；`web npm run typecheck` 已通过。当前待办：浏览器选择 2 章并确认执行，等待生产 Job 完成后做 DB/页面/连续性验收。
- 执行成功后，前端会解析 `chapterIndex`，统计“N 章 / M 个待复核”并提示用户刷新章节；如果请求章数只完成一部分，提示批量任务提前停止。
- 本步先覆盖“同步查询 + 成功/失败状态投影”，暂不实现取消/断线恢复，也不把 Job 迁移到 Vergestar Task/SSE。
## 2026-09-10 多章连续写作 E2E 验收：2 章成功

- 修复前端 3 个 TypeScript 错误：`NovelAgentConfirmationActionPayload` 增加 `writeNext.chapterCount` 专用类型，`web npm run typecheck` 通过。
- 浏览器实机选择“2 章”，确认执行 Writer，Job `2ecb1680-9ef9-4d8b-ad96-cb2d96b5890d` 成功，耗时约 21 分钟（17:05:14-17:26:03）。
- InkOS Harness 阶段链完整：准备章节输入 → 撰写草稿 → 第 2 章创作 → 状态结算（933 字） → 提取事实 → 回写真相文件 → 审计 → 落盘 → 生成最终真相文件 → 校验变更 → 同步记忆索引 → 更新索引与快照。
- 第 2 章标题《三源偏差》，正文 933 字，状态 `ready-for-review`；Auditor 输出 3 条 minor 警告，未阻断。
- DB 实测：`project_units(kind=chapter)` 2 条；`story_chapter_versions` 2 条；`story_reviews` 2 条；第 2 章 word_count 973（含 Markdown 标题），版本内容长度 973。
- 页面实测：顶部状态从“1 章已建立”变为“2 章已建立”；核心文件从 20 个变为 23 个，新增第 2 章正文、`chapter_summaries.md`、`subplot_board.md`；章节列表出现第 2 章；第 2 章有独立版本 1 和独立审稿。
- 连续性实测：第 2 章直接承接第 1 章签名异常，落实 H002 三源时间冲突、H004 权限后果、H006/H008 外包与设备缝隙，并更新第 1-2 章摘要和伏笔池。
- 差距更新：Writer 从“很大”缩小为“小到中”；Auditor 生产链路已有实机结果，但仍需把质量警告转成 Reviser 修订决策；连续批量链路可用，但前端仍缺逐章进度、任务取消、断线恢复和历史任务列表。

## 2026-09-10 投影持续同步实机校验

- 基线确认：`repair_state` 成功后，InkOS `chapters/index.json` 为 `ready-for-review`，DB `ProjectUnit.status` 已为 `ready`，但 DB 仍保留首轮写入的旧审稿 Summary；`story_memories` 里的当前状态、伏笔池和情感弧线也仍是建书时内容。前端此时虽不再显示修复入口，却会把过期审稿继续展示给用户。
- 已修复：
  - `syncNovelChapterMetadata` 对同一章节版本执行审稿 upsert；InkOS 审稿结果变化时更新 status/summary/issues，正文未变不会新增版本或审稿记录。
  - `syncNovelAgentProjection` 对 `SourceType=inkos-file` 的既有故事记忆按 `SourceID` 更新内容和来源哈希；新文件创建、旧文件不重复。
  - Bridge 导出与后端白名单新增 `story/chapter_summaries.md`、`story/subplot_board.md`；投影为 `chapter_summary` 和 `subplot` 两类故事记忆。
- 回归：Novel Agent build + test 5/5；后端 database/handler 全通过，service 聚焦通过；完整 service 仍存在既有资源删除相关失败（`TestDeleteLocalResourceObjectRemovesOnlyResourceDirectoryFile`、`TestResourceDeletionWorkerRemovesObjectAndCompletesOutbox`）；前端 typecheck 通过。
- 服务重建后实机查询：`GET /api/novel-agent/jobs/ec798b90-7872-4a34-b8e4-469069775d26` 返回 `succeeded` 并触发投影；DB 审稿 Summary 已变为“[warning] 正文中顾宁……”开头的 8 条最新审计；故事记忆当前状态不再是占位符，并包含第 1 章摘要和支线板；页面刷新显示“1 章已建立”，正文/版本仍为版本 1。

- 实机重试：Job `04f7f638-7d6b-4e2b-a442-7d3c0c84e833` 再次推进到 `2a：提取第2章事实` 后被同一上游连接失败阻断。当前判定为外部模型服务瞬断，代码链路已能进入 Reviser；暂不继续重试以免重复消耗。
- 实机首跑：Job `6539401b-fd3c-4406-bfdd-1b16cc2ab072` 进入 Reviser 与状态结算阶段，但在状态校验阶段被上游 `https://api.b.ai/v1` 连接失败阻断，Job 状态 failed。该结果证明：修订链路真实落库/落盘前有状态校验，且上游模型不可用不会伪造修订成功。
## 2026-09-10 审稿修订：Reviser 直连生产链路接入

- 已接入 Vergestar 专属 `revise_chapter` 确认动作：前端章节面板新增 `InkOS Reviser 章节修订`，可选择 `spot-fix / polish`，把当前章节号和最新审稿问题作为结构化 `confirmedActionPayload.reviseChapter` 发起任务。
- Bridge 复用 InkOS 原生 `PipelineRunner.reviseDraft`，不绕过生产链路；执行顺序为：预审计 → Reviser 生成修订稿 → 状态结算 → 状态校验 → 后审计 → 修订门禁 → 落盘/快照。
- 若修订未改进审计指标，InkOS 严格门禁会保留原章节并返回原因；若通过，则更新章节文件、章节索引、状态快照和故事记忆索引。
- 回归：Novel Agent build 通过、test 5/5；后端 Novel Agent 聚焦测试通过；前端 typecheck 通过。服务已用新 Bridge 和新后端重建并重启。
- 差距更新：Reviser 从“尚未接入生产 Agent”缩小为“可由审稿问题触发，且带有质量门禁”；剩余差距是逐章修订队列、自动批量修订、修订结果对比视图和更细粒度编辑建议。

## 2026-09-10 版本回溯 E2E 验收：人工版本创建与恢复通过

- 基线确认：验收项目第 1 章已有 InkOS 生产写入产生的版本 1，正文长度约 838 字符；`StoryChapterVersion` 与页面版本面板均可读取。
- 幂等快照确认：对未修改正文再次创建版本时，系统能识别同一内容并去重，没有生成重复版本。
- 人工修改链路：将第 1 章替换为 12 字节验收文本后保存，`StoryChapterVersion` 新增版本 2；DB 两条版本内容长度分别为 838 和 12，页面出现“版本 2 / 版本 1”。
- 恢复链路：点击恢复版本 1 后，页面提示“章节已恢复到选定版本”，正文回读为完整原文 838 字符；DB `project_units.source_text` 恢复为原文，版本表继续保留版本 1 和版本 2，满足“恢复不删除历史”的要求。
- 已知联动缺口：恢复后 Vergestar 章节状态回到 `draft`，并出现“本章状态需要修复”；原因是当前恢复只回写 Vergestar 正文，不会自动同步 InkOS 章节索引/状态快照。这属于正常暴露的待补差距，不是恢复动作丢数据。
- 差距更新：版本回溯从“中”缩小为“小”；剩余差距是恢复后自动调用 InkOS 状态修复/快照联动，以及生产 Reviser 修订产生的新版本与人工版本统一展示。
## 2026-09-10 版本回溯增强：InkOS 状态快照联动

- 新增 Bridge 内部接口 `POST /agent/chapters/restore-snapshot`：复用 InkOS `StateManager.restoreState`，按 `bookId + agentSessionId + chapterNumber` 回退对应章节状态快照，然后读取章节产物。
- 后端 `RestoreStoryChapterVersion` 先恢复 Vergestar 选中版本，再检查项目是否绑定 InkOS 书籍；绑定存在时自动调用 Bridge 状态快照回退。Bridge 返回的章节正文必须与选中版本一致，否则拒绝投影，防止旧工作区覆盖 DB 正文。
- 状态快照恢复成功后，复用 `syncNovelAgentArtifacts` 投影 InkOS 的章节索引、故事记忆、伏笔池、支线板和章节状态。
- 回归：Novel Agent build + test 5/5；Web typecheck 通过；后端 database/handler/service 聚焦测试通过；Backend/Bridge/Web 已重建重启。
- 实机验收：在《Vergestar同步验收》第 1 章点击恢复版本 1。页面提示“章节已恢复到选定版本”；DB `ProjectUnit.status` 从 `draft` 变为 `ready`，正文长度 838；InkOS `current_state.md` 从“当前章节 2”回退为“当前章节 1”；页面故事记忆中的当前状态、伏笔池和章节摘要也回到第 1 章版本。
- 差距更新：版本回溯从“小”缩小为“极小”；剩余差距是生产 Reviser 产生的新版本与人工版本的对比视图、按版本关联审稿结果的展示，以及未来分支从指定版本展开。
## 2026-09-10 角色资产投影升级：来源变化版本化

- 已把 InkOS `story/roles/**` 的投影从“首次创建”升级为幂等 upsert：角色文件首次出现时创建 Vergestar 角色资产；来源 ID、角色分类或 Markdown 内容变化时通过既有 `UpdateProjectCharacter` 创建新的不可变角色版本；内容未变化时不重复创建。
- 投影定义保留 `sourceType=inkos-agent`、`sourceId`、`roleCategory`、`sourceMarkdown` 和 `sourceHash`，保证角色资产可以追溯到 InkOS 原始文件；已有手绘形象、历史版本和镜头引用不会被删除。
- 回归：后端 database/handler/service 聚焦测试通过；新增测试覆盖“角色变化生成新版本 + 重复投影不重复版本”。Backend 已重建重启。
- 实机验收：修改《Vergestar同步验收》InkOS 角色卡 `story/roles/主要角色/顾宁.md`，追加验收标记，再通过页面恢复第 1 章版本触发状态快照联动。`POST /api/story/projects/.../restore` 返回 200；DB 中顾宁资产仍只有一个，但角色版本从 1 个变为 2 个，当前版本 `DefinitionJSON` 包含验收标记，历史版本仍保留。
- 差距更新：角色资产从“大”缩小为“小到中”；剩余差距是角色卡结构化字段抽取、视觉资产自动生成/绑定、InkOS 次要角色和主要角色的资产分组展示。
## 2026-09-10 任务取消 P0 首阶段：真实浏览器验收通过

- 修复取消链路缺口：`cancelNovelAgentJob` 原本只触发本地 `AbortController`；对 InkOS 规划回转（缓存的 pi-agent 会话）不够用，可能取消后上游仍继续执行。现在 Job 会记录 `userId + sessionId`，取消时同时调用 `abortNovelAgentSession`，复用 InkOS `abortAgentSession` 终止并驱逐缓存会话。
- 回归：Novel Agent build + test 5/5；后端 handler 测试通过；完整 service 仍只剩项目原有的资源删除测试失败（`TestDeleteLocalResourceObjectRemovesOnlyResourceDirectoryFile`、`TestResourceDeletionWorkerRemovesObjectAndCompletesOutbox`），与本次改动无关。Backend 已重新构建并重启。
- 实机验收：页面发起“让 Agent 写下一章”，Job `238ff503-727b-4326-b029-8f9fef44da3b` 进入 `running`；点击取消后后端 `POST /api/novel-agent/jobs/:jobId/cancel` 返回 200；Job 文件状态为 `failed / 任务已取消`，页面显示“任务已取消”；InkOS 工作区没有产生第 3 章残留写入。
- 环境修复：发现 stale Novel Agent 进程占用 `127.0.0.1:17421`，导致取消请求长期命中旧服务。清理后确认当前服务路由正常；后续 `stop-vergestar.ps1` 需要按端口检查/清理孤儿进程，避免 PID 文件外的旧进程继续接管服务。
- 差距更新：失败恢复从“大”缩小为“中”；剩余差距是 Vergestar Task/SSE 统一持久化、断线后任务恢复、历史任务列表、逐章进度时间线，以及取消后已产生的部分 InkOS 文件的一致性检查。

## 2026-09-10 Reviser 成功案例：第 1 章修订 E2E 通过

- 从页面“修订本章”发起第 1 章定点修复，Job `74e5e909-9722-4959-9526-65edac7da70a` 成功。阶段链完整：加载 Reviser 生产链路 → 加载修订上下文 → 修订第 1 章 → 提取事实 → 回写真相文件 → 落盘修订结果 → 更新索引与快照。
- InkOS 结果：`chapters/index.json` 第 1 章状态为 `ready-for-review`，字数从 838 字扩展到 930 字；Reviser 返回“已按审稿意见修订第 1 章，状态 ready-for-review，修复 10 项问题”；新审稿从 8 条收缩为 4 条，原“触碰证据附件”“章尾钩子不具体”等 warning 已消除；无长度 warning。
- DB 投影：`project_units` 第 1 章正文长度 955、`word_count=955`、状态 `ready`；`story_chapter_versions` 保留版本 1（838 字）、版本 2（人工恢复测试文本）、版本 3（955 字修订稿）；第 3 版审稿记录与 InkOS 最新 `auditIssues` 一致。第 2 章未被误改。
- 页面投影：第 1 章版本面板出现“版本 3 / 2 / 1”，审稿面板显示最新 4 条审计问题。为避免旧审稿误导，前端已把工作台审稿展示改为最新记录（历史仍保留在 DB）；`web typecheck` 通过。
- 差距更新：Reviser 从“小到中”缩小为“小”；剩余差距是批量/逐章修订队列、修订前后 diff、按版本关联审稿历史、Agent 自动判断何时修订、修订取消与断线恢复、以及将第 2 章的 `needs-revision` 状态转成明确可执行任务。


## 2026-09-10 影视生产（Phase 5）：InkOS Script/Storyboard 首闭环实机通过

- Bridge 新增确认意图 `create_script` / `create_storyboard`，直接调用 InkOS Core 原生 `runScriptCreation` / `runStoryboardCreation`，不复刻提示词、不降级为普通聊天。产物路径为 `dramas/.../script.md` 和 `storyboards/.../{storyboard.md,image-prompts.md,assets.json,status.json}`。
- 产物链路：`FullNovelAgentTurnResult.artifacts.productionFiles` 携带影视产物；`productionUnitId` 记录前端选中章节。Backend 产物 schema 同步扩展，路径白名单允许 `dramas/*.md|json` 与 `storyboards/*.md|json`，产物先并入 `StoryFoundation.FilesJSON`，再投影到 `story_scenes` / `shots` / `story_scene_shot_links`。
- 后端新增 `syncNovelProductionArtifacts`：解析 InkOS `assets.json` 的 `prompt_ready` 镜头，按确定性 SourceID（`inkos-storyboard:<projectId>:<shotId>`）幂等投影；重复轮询不会重复建镜头。分镜 Markdown 只做轻量结构抽取，不作为唯一真相。
- Schema v11：`story_agent_messages.artifacts_json` 持久化已完成任务的产物投影元数据，避免会话历史只保存文字、丢失影视生产上下文。数据库迁移已实际执行到 v11。
- 前端新增章节级“InkOS 影视生产”面板，可选镜头上限，按钮分别触发“生成剧本”和“生成分镜”；沿用异步 Job、阶段日志和取消按钮。`web typecheck` 通过。
- 回归：Novel Agent build/test 5/5；Backend build/database/handler/service 聚焦测试通过；服务已重建重启。
- 实机验收：从 `/novel?project=31a69b1fd7b004e6872280a298b64fef` 第 1 章点击“生成分镜”，Job `3560804c-fca7-466f-a9f7-9de343a54540` 状态 `succeeded`；InkOS 工作区生成 `storyboards/book-vergestar-ch-0001/` 五个文件；DB 首次出现 12 条对应的 `story_scenes` / `shots` / `story_scene_shot_links`；页面“剧本场次”出现 12 个镜头。修复章节映射后，第 1 章也投影 12 条；“故事核心文件”文件数从 23 增至 26，显示分镜、图像提示词和 assets 清单。
- 首验暴露并修复的映射问题：首轮产物先落到最后一个章节，页面选中第 1 章看不到场次。已改为优先使用前端 `unitId`；缺省时从产物路径 `...-ch-0001` 推导章节号，并选择对应 ProjectUnit。修复后刷新页面确认第 1 章场次可见。
- 上游瞬断验收与重试成功：从页面发起“生成剧本”，Job `ac435a7a-63f7-44ef-9432-5cfa5a6596e0` 正确进入 `failed`，错误为“无法连接到 API 服务”；失败语义、取消按钮和阶段日志均正常，未产生半成品正史投影。上游恢复后重试 Job `68a5a550-032a-4a0e-89ff-822ef3f8cc1c` 成功，InkOS 工作区生成 `dramas/book-vergestar-ch-0001/{script.md,script-spec.md,status.json}`，剧本已并入 `StoryFoundation.FilesJSON`。
- 差距更新：小说到影视从“很大”缩小为“中”。剩余差距：1) `script.md` 的“人物/剧本正文”尚未完整结构化成场次；2) ShotRevision 的 Camera/Movement/Duration 等字段仍依赖后续解析；3) 生成分镜后只到 `prompt_ready`，尚未确认并触发图片/视频 Task；4) 互动影游 `runInteractiveFilmCreation` 未接；5) 长文多段分镜的章节/场次映射仍需做严格单元测试。
## 2026-09-10 Shot→确认→媒体生成链路验证与资产引用接入

- 已按既有 payload 提交验收任务 `afa92374a3bfec5f5514eccc176157ca`，任务创建、元数据落库和失败状态投影均正常；失败原因是本机后端启动环境拦截了对 `api.aixoras.com` 的出网连接（`dial tcp 198.18.0.9:443: connectex...forbidden`），不是任务契约或回填代码错误。上游域名经带权限的独立连通性检查可达。
- 为避免伪造生产结果，本轮不把外网图片生成标记为实机成功；先对真实后端服务层做聚焦回归，覆盖 `TestRegisterTaskOutputFromTaskPersistsMediaAssetAndArtifactIdempotently`、`TestRegisterTaskOutputAcceptsLinkedCanvasAndCreatesShotArtifact`、`TestCreateShotRevisionInvalidatesExistingArtifacts`、`TestUnlinkShotAssetDeletesReferenceAndInvalidatesProduction`、`TestUpdateChapterSourceInvalidatesAllUnitArtifacts`，全部通过。这证明任务成功后的媒体资产、`ProductionTaskLink`、`shot_artifacts` 幂等回填，以及镜头修订/资产解绑/章节改稿导致的 stale 传播都是可用契约。
- 小说镜头生成面板新增“引用角色/场景/道具资产”多选（最多 6 个）。用户确认生成时会：先把选中资产的主版本绑定为 Shot Reference（已绑定则跳过），再把有 `storageKey` 的参考图作为 `referenceImages` 传入后端生成任务，同时在 `artifactMetadata.referenceAssetIds` 保留来源。
- 回归：`web npm run typecheck` 通过。
- 差距更新：角色/场景资产引用从“未接”变为“已接入、待实机验收”；下游 stale 传播从“未验”变为“聚焦回归通过”；媒体生成链从“待接”变为“代码契约已验、外网生成待复验”。

## 2026-09-10 Script 场次→多镜头层级重构与实机迁移

- InkOS 真实 `script.md` 的结构已被解析：只读取 `## 剧本正文` 后的 `**场次N ...**`，分别投影人物、动作、对白和场景地点；第 1 章 Script Job `68a5a550-032a-4a0e-89ff-822ef3f8cc1c` 重读后，数据库实际得到 4 个稳定来源为 `inkos-script:<book>:scene-N` 的场次。
- Storyboard 不再把每一镜伪装成一个独立场次。镜头匹配改为按提示词语义和镜头顺序做全局单调分配，保留“一个场次可有多个镜头”的原始生产模型；旧的 `inkos-storyboard:*` 独立场次会把已有关联迁移到对应 Script 场次，而非删除镜头或重新生成。
- 后端 `StorySceneView` 向后兼容保留首个 `shotId`，同时新增有序 `shotIds`；小说镜头生成面板会把有多个镜头的场次展开为可选项，因而用户能为同一场次下的每个镜头单独确认生成图片或视频。
- 聚焦回归新增 `TestSyncNovelProductionArtifactsMapsStoryboardShotsToScriptScenesIdempotently`，覆盖 Script 场次、Storyboard 镜头归属、顺序和重复同步不增量；连同脚本解析、任务回填和 outbound 校验测试均通过。前端 `npm run typecheck` 通过。
- 实机迁移：重读 Storyboard Job `3560804c-fca7-466f-a9f7-9de343a54540` 后，项目 `31a69b1fd7b004e6872280a298b64fef` 的第 1 章稳定为 4 场、12 镜头、12 条 `story_scene_shot_links`；镜头分布为场次 1：1-4，场次 2：5-8，场次 3：9-11，场次 4：12。第二次同步计数仍为 `4 / 12 / 12`，API 已返回每场完整 `shotIds`。
- 出网修正与真实上游结果：后端出网解析现在会在检测到本机代理的 Fake-IP（`198.18.0.0/15`）时尝试公共 DoH，再继续执行既有 SSRF 拦截；8080 必须在正常网络上下文启动。验收图片任务 `960a1e367469e3c7ea96d208aa140aa0` 已从“本机 socket 禁止”推进到真实 `running`，最终由供应商返回 524 网关超时。系统已按“可能已计费”保护信息提示，未自动重试；因此 `shot_artifacts` 的真实图片资源回填仍待一个成功上游任务验收。
- 差距更新：`create_script_scenes` 从“未做”变为“完成并实机迁移”；故事到镜头的层级从“12 个孤立镜头”变为“4 个 Script 场次 / 12 个 Storyboard 镜头”；影视链路剩余的主要功能缺口集中在成功媒体资源回填实测、自动角色/场景资产挑选、批量镜头生成、音频和互动影游。

## 2026-09-10 Script 角色资产→镜头引用自动承接

- 新增 Script 人物匹配：`script.md` 场次的 `人物：…` 字段按角色资产名称的规范化键匹配已有 Vergestar 角色卡；仅匹配已确认的项目资产，不会因为文本出现名字就静默造出角色资产。
- 匹配结果落在 `story_scenes.character_asset_ids_json`，并进入场次来源哈希；角色名、角色卡或人物名单变化都会触发确定性更新。`StorySceneView` 原样返回该字段，前端在选择该场的镜头时会自动预选角色参考，但仍允许用户加减环境、道具和素材。
- Storyboard 镜头同步时，会把匹配角色的主版本以 `reference` 角色写入 `shot_asset_references`；已有引用按 `(shotId, assetVersionId)` 幂等跳过，不会因轮询重复写入或无意义地标记 stale。
- 回归扩展：`TestSyncNovelProductionArtifactsMapsStoryboardShotsToScriptScenesIdempotently` 现在同时校验场次角色资产 IDs、镜头归属顺序和重复同步幂等，已通过；`web npm run typecheck` 通过。
- 实机验收：项目 `31a69b1fd7b004e6872280a298b64fef` 的 4 个场次全部自动识别到“顾宁”角色卡；12 个镜头全部已有顾宁主版本的 `reference` 记录。剧本中的“白班同事”“终端系统声”暂无已确认角色卡，因此未自动虚构资产；这是预期的确认边界。
- 差距更新：角色资产持续使用从“手选、待验”缩小为“已有角色自动贯穿场次和镜头”；剩余为未匹配人物的确认式候选资产、环境/道具自动匹配，以及角色视觉素材生成后作为实际 `referenceImages` 提交。

## 2026-09-10 Script 未匹配人物→候选确认→即时回填实机通过

- Script 人物候选正式接入现有 `ProjectAssetCandidate` 工作流，新增来源 `inkos_script_character`。角色候选必须携带 InkOS 来源、场次标题和剧情定位，但不会因为剧本只出现姓名就伪造成完整人设或正式角色卡。
- Script-only 与 Script+Storyboard 两条同步路径都会检测未匹配角色。候选按角色规范化名称幂等去重，并关联当前章节；已有项目角色卡会优先匹配，不重复创建候选。
- 用户确认候选后，系统创建项目角色卡，再从 `StoryFoundation.FilesJSON` 提取对应章节的 InkOS 影视产物重同步：新角色马上写入 `story_scenes.character_asset_ids_json`，并按实际镜头范围补充 `shot_asset_references`，不需要再次调用 InkOS 或重跑模型。
- 回归：`TestSyncNovelProductionArtifactsCreatesPendingCandidatesForUnmatchedScriptCharacters` 覆盖“未匹配角色 → 待确认候选 → 确认 → Script 场次回填”；连同场次/多镜头幂等回归均通过。前端 `npm run typecheck` 通过。
- 页面实机验收：在 `/novel?project=31a69b1fd7b004e6872280a298b64fef` 的“剧本待确认角色”面板中出现白班同事（场次 1）与终端系统声（场次 4）。已通过页面鼠标点击确认“白班同事”：候选数量由 2 变为 1，页面 Toast 显示“白班同事 已创建为角色资产，并已回填当前影视场次”，镜头生成器自动参考从 1 个角色增至 2 个。
- 差距更新：未匹配人物从“静默忽略”变为“可见候选、显式确认、即时回填”；剩余是候选确认前的人设补全/角色视觉资产生成、环境与道具的自动匹配，以及成功图片任务的资源回填实测。

## 2026-09-11 InkOS Studio 实机操作对照与小说 UI 复刻（行为复刻路线）

### InkOS Studio 实机操作记录（2026-09-11，本项目本地 studio，项目根 `D:/BYW/inkos/test-project`）

- 以真实会话完整操作了一遍 InkOS 建书流程：输入创意 → Agent 输出结构化创作方案（标题/题材/篇幅/世界观/主角压力/核心冲突/前 30 章方向/节奏/禁忌）→ `propose_action` 确认卡（继续执行/取消）→ 点击继续执行 → "建书 · 执行中"任务卡（5 阶段时间线 + 逐条日志）→ FoundationReviewer 真实拒绝重试（第 1 轮 75/100 REJECTED，含分维度评分）→ 16m42s 后 `已完成`，产出《无敌下山记》（4 主要角色 + 3 次要角色 + 故事基石/卷纲等核心文件）。
- 关键结构确认：InkOS 的 `#/book/<id>` 主界面仍是**书聊 + 右侧真相面板**（章节/角色/核心文件折叠卡），聊天流内出现"写下一章 / 审计 / 导出 / 市场雷达"快捷动作；`BookDetail`（大标题 + 写下一章/仅草稿/审查模式 + 工具条 + 章节表）是 `#/book-settings/<id>` 下的次要管理视图。章节点位符号：✓ approved、◆ ready-for-review、○ drafted、✕ needs-revision。
- 视觉规范取样（用于复刻）：用户消息右对齐 `bg-secondary` 圆角块，助手消息左对齐 Markdown 渲染（Streamdown）；确认卡 = "确认动作"标签 + 标题 + 摘要 + 全文 + 右下 取消/继续执行；任务卡 = 标题 + 耗时秒表（Ns/Nm Ns）+ 状态徽章（执行中 spinner/已完成绿/失败红）+ 阶段清单（○/spinner/✓，当前阶段高亮）+ 可展开日志；左侧栏分区 = 开始创作（模式垂直列表）/ 我的创作（书籍，展开见会话）/ 互动影游 / 会话记录（相对时间"14 分钟"）/ 新建会话 / 系统 / 工具。

### 影策小说 UI 复刻实现（行为复刻，不复制 InkOS 源码；沿用既有 API 与数据契约）

- 新增 `web/src/pages/novel/agent-ui.tsx`：`AgentConfirmationCard`（确认动作卡）、`AgentTaskCard`（任务时间线卡：秒表、状态徽章、由 Job 日志中"阶段："行推导的阶段清单、可展开日志、取消按钮）、`AgentBubble`（用户右对齐气泡 / 助手 Streamdown Markdown 渲染）。
- `NovelAgentHome` 重排为 InkOS 结构：左侧导航（开始创作垂直模式列表 / 我的创作带章数 / 会话记录带相对时间 + 新建会话）+ 对话区（InkOS 气泡与确认卡 / 任务卡）+ InkOS 式输入坞（无边框 textarea 聚焦高亮、底部 当前 Skill + 模型选择 + 发送）。
- 新增会话绑定的**右侧真相面板**（InkOS BookSidebar 行为复刻）：会话存在绑定项目（或本轮建书成功返回 `storyProjectId`）时，显示 章节（状态符号 + 字数）/ 角色（主要/次要）/ 核心文件 三张折叠卡与"打开小说工作台"按钮；建书成功消息自带"打开小说工作台/刷新书架"动作，解决"建完书不知道去哪"的操作断点。
- `NovelProjectPanel` 重排为 InkOS BookDetail 风格：面包屑 + 衬线大标题 + 题材/章数/总字数统计 + 主操作按钮行（写下一章/剧本与分镜/故事资料/进入画布）+ 工具条式视图导航；右侧改为 InkOS 折叠卡（章节 ✓/○ 符号 + 字数、角色主要/次要、核心文件）。全部既有功能（Writer/审稿修订/影视生产/故事资料）保留在对应视图内。
- 回归：`web npm run typecheck` 通过；Novel Agent 链路（Bridge 17421 + 后端代理）实测 15 个 Skill 与会话列表正常返回。
- 实机 E2E（账号 1234，全新环境）：会话发创意 → Agent 返回结构化方案（Markdown 正确渲染）→ 二轮对话后 Agent 调用 `propose_action` → 新确认卡完整渲染 → 点击继续执行 → 任务卡显示"生产任务 · 秒表 · 执行中 + 启动 InkOS Agent Harness"，建书 Job 真实运行。会话刷新后消息与会话记录（含相对时间）均恢复。
- 建书 Job 全程实机验收（约 18 分钟）：任务卡阶段时间线随日志推进（生成基础设定 → 审核基础设定第 1 轮 → 第 2 轮），日志实时显示"[78] 节奏可行性…"分维度评分与"基础设定未通过审核（73/67 分），正在重新生成"，质量门禁在 UI 原样呈现；最终 `Book 三分钟未归 initialised successfully`。建书成功消息出现"打开小说工作台 / 刷新书架"动作；刷新后侧栏"我的创作"出现《三分钟未归》（会话自动绑定 `projectId`），右侧真相面板显示 角色 6 个（主要/次要）与核心文件 20 个；项目工作台显示面包屑、衬线标题、统计（0 章/0 字）、阶段卡（故事基础已就绪/章节待开始/影视待开始）与 InkOS 式右侧折叠卡。
- 实测暴露的已知问题（非本次 UI 改动引入）：① 模型可能把方案当纯文本回复而不调用 `propose_action`（短篇一问即遇），或工具调用参数格式非法被 InkOS schema 拒绝（action 字段混入换行/标签）；重试后可恢复。此类抖动上游可复现，后续可在 Bridge 增加规划回合的强提示词或自动重试。② 本地手动起服时必须同时带 `VERGESTAR_NOVEL_AGENT_URL/TOKEN` 启动后端并先启动 Bridge（node novel-agent/dist/index.js），否则 /api/novel-agent/* 全部失败；端口 17421。

## 2026-09-11 功能对齐路线图：InkOS ↔ 影策差异核对与工期判断

目标顺序（项目负责人 2026-09-11 确认）：先把影策小说部分补齐到与 InkOS **功能对等**，再按总纲推进影视化（画面→视频链路）。

### 差异核对基准

InkOS Studio 实测 API 面 140+ 路由；影策已覆盖建书/写作/审稿/修订/版本/剧本/分镜/镜头引用等核心链路（见上文 2026-09-10 各验收记录）。核对后按"对齐块"分级如下。

### T1 生产管线补齐（创作主链路，最高优先）
| 能力 | InkOS | 影策现状 | 预估 |
| --- | --- | --- | --- |
| 短篇小说生产链（short_run：大纲→大纲审→逐章写作→草稿审→修订→包装） | 完整 | 本轮已接入 Bridge→后端→投影 | 已做 |
| 仅草稿 draft / plan / compose / approve / reject | 完整 | 未接 | 1.5–2 天 |
| 章节重写 rewrite / 编辑回写 resync | 完整 | 未接（人工改稿已有版本化） | 0.5–1 天 |
| 审查模式 auto/manual + 书籍设置（目标章数/单章字数/状态/删书） | 完整 | 未接 | 0.5–1 天 |

### T2 阅读/导出/质量工具
真相文件浏览器（全部文件 + InkOS 式渲染 + 在线编辑回写 PUT truth）1 天；章节阅读器（ChapterReader 风格）0.5 天；导出 TXT/MD/EPUB + 过审过滤 0.5–1 天；detect AI 味检测 / eval / consolidate 接入 1 天。小计 3–3.5 天。

### T3 平台语义对齐（=总纲 P0/P1/P2）
Job→Vergestar Task/SSE 统一持久化、历史任务列表、断线恢复、逐章进度、单写入锁 2–3 天；流式输出（token 流 + 工具状态）1–2 天；会话管理（重命名/删除/书内会话分组）0.5–1 天。小计 3.5–6 天。

### T4 扩展创作模式（Phase 6 前段）
同人 fanfic / 番外 spinoff / 仿写 imitation / 续写（InkOS init 管线映射 + canon 导入）2–3 天；翻译译介 1 天；封面生成 0.5–1 天；题材管理 / 提示词包 0.5–1 天；市场雷达 1 天；守护进程（定时自动写作，可选）1–2 天。小计 5–9 天。

### T5 互动影游/开放世界
InkOS 完整子系统（PlayRunner / PlayStore / StoryGraph / 互动影游向导）。预估 4–6 天。

### T6 影视化后续（=总纲 Phase 5 收尾，功能对齐完成后执行）
图片任务成功回填复验 + 视频任务首跑 + 批量镜头 + ShotRevision 字段解析 + 音频 2–3 天；环境/道具自动匹配 + 角色视觉素材生成绑定 1–2 天。

### 总工期判断

- T1+T2+T3（创作主链路 + 阅读工具 + 平台底座）：**约 10–14 个全职工作日当量**，是"能像 InkOS 一样用"的最小闭环。
- 加 T4+T5（扩展模式 + 互动影游）：**全量功能对齐约 21–30 个全职工作日当量（4–6 周）**；按本项目实机验收节奏约合 15–25 个专项开发会话。
- 边界说明：InkOS 的 services（模型配置）、doctor、logs 页**不复刻**——影策有自己的渠道/模型/任务中心体系，这是总纲明确边界。

### 本轮已落地：短篇生产链（T1 第一项）

- Bridge：`confirmedIntent` 新增 `short_run`；确认提取放行 `proposed_action(action=short_run)`；生产分支复用 InkOS 原生 `runShortFictionProduction`（6 个 AgentContext：short-outline / short-outline-review / short-writer / short-draft-review / short-revise / short-package），产物 `shorts/<storyId>/`（大纲/审核/正文/销售包）全部进 `productionFiles`；因短篇无 book.json，合成最小书籍元数据（bookId=`short-<storyId>`、chapterIndex 空）走既有项目投影链路，短篇以小说项目形态进入书架与真相面板；`write_next` 等章节操作对短篇项目不适用（InkOS 语义即为整篇生产）。
- 后端：`ConfirmedIntent` 校验放行 `short_run`（无需 bookId）；`validNovelArtifactPath` 白名单新增 `shorts/*.md|json`。
- 前端：确认/请求类型扩展 `shortRun` 参数与 `short_run` action；真相面板与工作台"核心文件"过滤加入 `shorts/`。
- 回归：web typecheck 通过；Bridge build + test 5/5 通过；后端 go build 通过；Bridge/后端已重建重启。
- 实机 E2E（最终通过）：`short_run` 确认卡 → 生产 Job 约 25 分钟完成 → 大纲经 v001→v002 审核修订（质量门禁生效）→ 14 章全部写完 → 打包阶段定稿标题《必须本人签收》→ `final/full.md`（约 9883 字）+ 销售包 + 封面提示词全部产出 → 投影为 Vergestar 项目（书架出现"五年前的那单没送达"，真相面板 7 个 shorts 文件）→ 工作台"故事资料"可完整阅读正文。封面未生成属预期（需在渠道配置图像端点 `INKOS_COVER_BASE_URL`，归入 T4 封面项）。
- 调试过程中定位并修复的三个链路缺口（前两个为本次接入引入，第三个为上游缺口）：
  1. Bridge HTTP 入口 `src/index.ts` 有独立的 `confirmedIntent` 白名单，未放行 `short_run` 导致确认动作被静默降级为普通对话——已放行；
  2. 后端 `syncNovelProductionArtifacts` 把 `shorts/*` 产物当作剧本/分镜产物要求目标章节导致 502——已改为 shorts 产物跳过场次/镜头投影，只进 StoryFoundation；
  3. 上游 InkOS 短篇 agent 未使用 `completeLongForm` 续写机制，自建渠道单次输出硬上限（~8k token）导致大纲/整稿必然截断失败——已在 vendor 中给 createOutline/writeDraft/continueDraft/reviseDraft 接入续写（与 script-storyboard 同模式），并把三处显式 4096/8192 maxTokens 提升到 16_384（agnes 模型服务端思考消耗输出预算）。均已记录 `vendor/INKOS_UPSTREAM.md`，可回馈上游。
- 工作台补齐：项目无章节时"故事资料"视图不再被空状态遮挡（短篇项目的主阅读入口）。

## 2026-09-11 InkOS Studio 详细对照（UI / 操作逻辑 / 生产流程 / 生成内容）与书聊快捷动作复刻

### 1. UI 对照（逐区域）

| 区域 | InkOS Studio | 影策现状 | 对齐状态 |
| --- | --- | --- | --- |
| 应用壳 | 左 Sidebar（开始创作 12 模式 / 我的创作 / 互动影游 / 会话记录 / 新建会话 / 系统 / 工具）+ 头部（首页 / InkOS Studio、中/EN、主题） | 影策全局侧栏 + 小说页内 InkOS 式导航（8 模式 / 我的创作带章数 / 会话记录带相对时间 + 新建会话） | 基本一致（系统/工具区由影策全局设置承担，不复刻） |
| 聊天区 | 用户消息右对齐浅色块、助手 Markdown、Thought 折叠、工具执行步骤 | 同结构（Streamdown 渲染、确认卡、任务卡） | 基本一致（Thought/工具步骤流式展示仍缺 → T3 流式） |
| 确认卡 | "确认动作"+标题+摘要+全文+继续执行/取消；执行后显示"已执行" | 同布局同文案 | 一致 |
| 任务卡 | 名称+秒表+状态徽章+阶段清单（○/spinner/✓）+日志 | 同结构，阶段由 Job 日志"阶段："行推导 | 一致（完成后保留卡） |
| 书聊快捷动作 | 输入框上方 chips：写下一章/审计/导出/市场雷达 | 本轮已复刻同款 4 chips（书绑定会话才显示） | 一致 |
| 输入坞 | 圆角容器内无边框输入 + 添加 Skill/上传 + 底部模型选择 | 同布局（Skill 由左侧模式承担） | 基本一致（上传附件未接 → T2/T4） |
| 书籍真相面板 | 右侧折叠卡：进度/故事基石/章节摘要/章节（状态符号+字数）/角色（主要/次要）+文件阅读器 | 右侧三卡（章节/角色/核心文件）+工作台故事资料阅读器 | 基本一致（进度卡、伏笔卡待补 → T2） |
| 书籍管理页 | BookDetail：衬线标题+统计+写下一章/仅草稿/审查模式+工具条（真相文件/分析/评估/导出等）+章节表 | 同风格头部+统计+按钮行+四视图 | 部分一致（仅草稿/审查模式/评估等 → T1/T2） |
| 章节阅读 | ChapterReader 独立页 | 工作台"阅读与编辑"+故事资料渲染 | 部分一致（独立阅读器 → T2） |

### 2. 操作逻辑对照

| 操作 | InkOS | 影策 | 对齐状态 |
| --- | --- | --- | --- |
| 从想法到建书 | 输入创意 → 结构化方案 → 确认卡 → 生产任务卡（5 阶段+审稿拒绝重试）→ 书进书架 | 完全同流程（含 FoundationReviewer 分维度评分拒绝） | 一致 |
| 写下一章 chip | 发送"写下一章"→ Agent 规划 →（InkOS 直接执行） | 发送同命令 → 规划 → 确认卡 → 确认后执行（总纲安全设计：生产动作必须确认卡） | 语义一致，多一道确认（有意保留） |
| 审计 chip | 文本命令 → sub_agent(auditor) | 同命令 → book-audit 回合放行 sub_agent(auditor)，中文总结结论/评分/问题清单 | 一致 |
| 导出 chip | 文本命令 → sub_agent(exporter) | 同命令 → book-export 回合放行 sub_agent(exporter)，报告导出路径 | 一致（导出文件尚不回传 Vergestar 界面 → T2） |
| 市场雷达 chip | 文本命令 → Agent 研究工具扫描 | 同命令 → book-radar 研究回合 | 一致 |
| 会话 | 按书分组、重命名/删除、draft 会话、会话内恢复 | 列表+恢复+相对时间（重命名/删除/分组 → T3） | 部分 |
| 多章连续写作 | 章节计数选择 + 逐章进度 | 1/2/3/5/10 章选择（逐章进度 → T3） | 大部分一致 |

### 3. 生产流程对照

| 流程 | InkOS 阶段链 | 影策 | 对齐状态 |
| --- | --- | --- | --- |
| 建书 | 生成基础设定→保存配置→写入文件→初始化控制文档→初始快照（FoundationReviewer 多轮拒绝重试） | 同（真实复现 73/67 分拒绝重试） | 一致 |
| 写章 | 准备输入→撰写草稿→状态结算→提取事实→回写真相→审计→落盘→快照 | 同（PipelineRunner 直连） | 一致 |
| 修订 | 预审计→修订→状态结算→校验→后审计→门禁→落盘 | 同（revise_chapter 已接） | 一致 |
| 短篇 | 大纲→大纲审→逐章写作→草稿审→修订→包装 | 同（short_run 本轮接入，含续写补丁） | 一致（封面需渠道配置） |
| 剧本/分镜 | runScriptCreation/runStoryboardCreation | 同（确认动作直调原生管线） | 一致 |
| 互动影游/Play | 独立子系统 | 未接 | 缺（T5） |

### 4. 生成内容对照

| 内容 | InkOS | 影策投影 | 对齐状态 |
| --- | --- | --- | --- |
| 书籍元数据 | book.json（题材/篇幅/语言/世界观/角色矩阵/卷纲） | StoryFoundation.BookJSON + FilesJSON（幂等、来源哈希） | 一致 |
| 章节产物 | chapters/0001_*.md + index.json（状态机 ready-for-review/needs-revision 等） | ProjectUnit + ChapterVersion + Review（状态 draft/ready 映射） | 基本一致（InkOS 全状态机映射待补） |
| 真相文件 | story/*.md（current_state/pending_hooks/emotional_arcs/subplot_board/character_matrix 等） | 同名文件进 FilesJSON，真相面板/故事资料可读，repair_state/版本恢复联动 | 一致 |
| 审稿记录 | reviews/*.md + 审计问题清单 | StoryReview（最新审稿展示，历史保留） | 一致 |
| 短篇产物 | shorts/<id>/（outline/reviews/final/full.md/sales-package/cover-prompt） | 同路径全量投影 + 项目化（书架/真相面板/阅读） | 一致（封面未配渠道） |
| 导出文件 | exports/<book>/*.txt|md|epub | InkOS 工作区生成，尚未回传界面 | 部分（T2） |
| 剧本/分镜 | dramas/*/script.md、storyboards/*/{storyboard,image-prompts,assets}.json | story_scenes/shots/shot_asset_references 幂等投影 + 镜头生成面板 | 一致 |

### 5. 本轮落地：书聊快捷动作（写下一章/审计/导出/市场雷达）

- Bridge：`input.bookId` 且非确认回合的命令回合按 mode 分派——`book-audit`（放行 sub_agent(auditor)，总结审计结论/评分/问题清单）、`book-export`（放行 sub_agent(exporter)，报告导出路径）、`book-radar`（研究工具市场扫描，不修改文件）；`suppressProductionTools` 对 audit/export 回合放开；默认 bookId 规划回合（Writer 写下一章）行为不变。
- 前端：书绑定会话在输入框上方渲染同款 4 chips（Zap/SearchCheck/FileOutput/TrendingUp 图标、禁用态、横向滚动）；点击即以异步 Job 执行（任务卡进度/取消/完成后保留），结果与确认卡追加进聊天流；确认卡携带 bookId，"继续执行"可从聊天直接写章。
- 回归：web typecheck 通过；Bridge build + test 5/5；后端无改动（mode 为自由字段，bookId 链路已有）。
- 实机 E2E：chips 渲染与 InkOS 一致；"导出"chip 在 0 章书籍上得到正确中文响应（列出可导出资产、给出二选一建议），证明 audit/export 的 sub_agent 通路放行有效；"审计"在有章节书籍上的完整审计输出待首章写作后补验（机制与导出同通路）。

### 6. 对照结论与剩余差距（更新 T 清单）

- 已对齐：创建主链路 UI/流程/内容、确认卡、任务卡、真相面板、书聊快捷动作、短篇链路、剧本/分镜链路。
- 剩余（按优先级）：T1 仅草稿/审查模式/rewrite/resync/书籍设置删除书；T2 真相文件浏览器+编辑、章节阅读器、导出文件回传下载、detect/eval/consolidate、伏笔/进度卡；T3 Task/SSE 持久化、流式 Thought/工具步骤、会话分组/重命名/删除；T4 同人/番外/仿写/翻译/封面/雷达页；T5 互动影游。





### 7. 微信进度通知

- 推送脚本已就绪：`.local/notify-wechat.ps1`（支持 Server酱 SendKey / 企业微信机器人 webhook，读 `.local/wechat-push.json` 或环境变量 `WECHAT_PUSH_URL`）。
- 待项目负责人提供任一通道凭据后，后续每个开发阶段完成时自动推送进度（脚本已可直接使用）。

## 2026-09-11 T1 全量落地：BookDetail 管理操作 + 仅草稿/规划/成文确认动作

### 已实现（代码全部合入并通过构建/回归）

- **Bridge 内部管理接口**（novel-agent/src/book-management.ts，复用 InkOS StateManager，与 studio 服务端同语义）：
  - POST /agent/chapters/approve：章节状态置 approved（只改章节索引，不改正文）；
  - POST /agent/chapters/reject：拒绝本章并 rollbackToChapter 回滚到上一章（丢弃被拒章节及其状态）；
  - POST /agent/books/settings：写 book.json（targetChapters / chapterWordCount / status / writing.reviewMode），实现 InkOS 审查模式 auto/manual 持久化；
  - POST /agent/books/delete：整目录删除书籍工作区。
  - 以上产物均回传后端重投影，保持 Vergestar DB 单一真相。已 API 级实测：审查模式 manual/auto 切换 200 且正确落盘 book.json；设置保存 200；approve 在 0 章书上干净报业务错（第 1 章不存在，无法通过）。
- **Bridge 确认动作扩展**：draft_next / plan_chapter / compose_chapter 分别映射 InkOS 原生 pipeline.writeDraft / planChapter / composeChapter（仅草稿=写完即停不审计；规划=只出章节意图；成文=按意图写作）。pipeline 的 chapterReviewMode 现在读书籍设置 writing.reviewMode（Bridge 侧逐书生效）。
- **后端**：新意图放行白名单（含三个新动作，且要求 bookId 绑定）；服务层 ApproveNovelChapter / RejectNovelChapter / UpdateNovelBookSettings / DeleteNovelBook（novel_book_management.go，统一桥接调用 + 产物重投影；DeleteNovelBook 同时删除 Vergestar 项目）；路由 POST /story/projects/:projectId/inkos/chapters/:n/approve|reject、PUT /story/projects/:projectId/book-settings、POST /story/projects/:projectId/delete-inkos-book。
- **前端工作台**（对齐 InkOS BookDetail）：Writer 面板新增"仅草稿·写完即停"按钮（draft_next）；头部新增"审查：自动/手动·写完即停"切换（写 book.json.reviewMode）、"书籍设置"弹窗（单章字数/目标章数/书籍状态）与"删除"按钮（Popconfirm 确认，同时删 InkOS 工作区与 Vergestar 项目）；右栏章节行悬停出现 ✓通过/✕拒绝快捷操作；0 章项目空状态同时显示 Writer 面板（对齐 InkOS：0 章也可直接 draft/plan）。
- 回归：web typecheck 0 错误；Bridge build + test 5/5；后端 go build + 聚焦测试通过；Bridge/后端已重建重启。

### 实机验收状态

- 审查模式与书籍设置：API 级 E2E 通过。
- draft_next 链路：任务正确进入 InkOS Writer 生产流（准备章节输入 → 撰写章节草稿 → 阶段 1 创作正文第 1 章），落盘验收暂被上游渠道问题阻断（非代码缺口）：agnes-2.5-pro-beta 服务端思考曾消耗全部输出预算（46870 completion tokens、可见内容为空），随后渠道 429 限流。渠道恢复或换模型后从工作台点"仅草稿"即可补验。
- 调试中发现并修复：Bridge/后端 confirmedIntent 白名单漏放新意图导致静默降级（与 short_run 同型问题）；writer 32768 输出预算超渠道上限（vendor 补丁降 16384，已记录 INKOS_UPSTREAM.md）；gin 路由参数冲突（chapterNumber 与 unitId 同前缀，改用 /inkos/ 前缀）；Windows curl 发中文 bookId 会 mojibake（测试方法问题，非代码）。

### 本轮暴露的待办（下轮优先）

1. 短篇项目必须隐藏章节操作：短篇项目页仍显示 Writer 面板，"写下一章/仅草稿"必然失败（短篇合成 bookId 在 books/ 下无目录）。按项目类型或 foundation 文件形态判断并隐藏/降级。
2. Bridge 业务错误（如"第 1 章不存在"）应返回 4xx 而非 500，避免后端一律包装成 Bad Gateway 语义。
3. Writer 空响应（思考吃光预算）应在 core 层自动降 maxTokens 重试一次，而非直接失败。
4. draft 落盘 E2E 待渠道恢复后补验；approve/reject 带章 E2E 待首章落盘后补验。

## 2026-09-11 T2 落地：真相文件在线编辑 / 导出回传 / 质量工具 / 短篇章节操作修正

### 已实现（代码全部合入，web typecheck 0 错误、Bridge build+test 5/5、后端 go build 通过）

- **真相文件在线编辑回写**（对齐 InkOS GET/PUT truth）：
  - Bridge `POST /agent/books/truth/read | /agent/books/truth/write`：白名单与 InkOS Studio 一致（story/ 扁平文件 + outline/ 权威大纲 + roles/ 角色卡），路径穿越/反斜杠/超 512KB 拒绝；同时兼容两种路径形态（相对 story/ 的 pending_hooks.md 与含前缀的 story/pending_hooks.md，后者是 Vergestar FilesJSON 语义）。
  - 后端 `POST /story/projects/:projectId/truth/read | truth/write`（novel_book_tools.go + story.go 路由）。
  - 前端工作台"故事资料"的 StoryFoundationPanel：每个 story/ 文件增加"编辑"按钮 → textarea → 保存 → 经后端写回 InkOS 工作区 → invalidate 重投影。
  - UI 级 E2E：编辑故事基石 → 保存 → 经 /truth/read 回读内容一致（含时间戳验证标记）；验证后已将 author_intent.md 恢复默认模板。
- **导出文件回传**：Bridge `exports/list | exports/read`（遍历 exports/ 下 txt/md/epub，读按 base64 回传）；后端 `GET /story/projects/:projectId/exports | exports/content?path=`；前端故事资料底部"导出文件"区块（文件名/大小/下载按钮，Blob 触发浏览器下载）。当前无导出产物时区块自动隐藏。
- **质量工具**：Bridge `books/eval`（evaluateBookQuality 纯本地统计）、`books/consolidate`（ConsolidatorAgent，需传模型）、`books/detect`（detectChapter；未配置项目级 detection 服务时返回明确中文提示而非报错）；后端 eval/detect 路由；前端面板加"书籍评估"按钮（结果 toast 展示）。
- **短篇项目章节操作修正**（T1 待办）：foundation 含 shorts/ 文件的项目判定为短篇，Writer 面板替换为说明文案（短篇为整篇生产，不支持逐章写作），"新建章节"按钮禁用——修复短篇项目点"写下一章"必然失败的问题。

### 实机验收状态

- truth 读/写/回读、eval：API 级 + UI 级 E2E 通过（详见上）。
- 导出面板：代码就绪；真实下载验收待 exports/ 有产物后补验（依赖 book-export 在有章节书籍上运行）。
- consolidate/detect：Bridge 与后端就绪；consolidate 依赖内容生产（agnes 渠道仍在 403/429），detect 需外部检测服务配置；两者入口未上前端（consolidate 属低频操作，detect 需配置引导），随 T3 一并评估入口形态。
- draft 落盘补验：再次被 agnes 渠道 403（余额/审查）阻断，按总纲"上游瞬断"语义挂起，渠道恢复后一键补验。

### 暴露的问题与待办

1. agnes 公益渠道可用性差（思考吃输出/403/429 轮番出现），已稳定阻断内容生产类验收；建议项目负责人更换或新增一个稳定付费渠道后再推进 T3 的内容类验收。
2. StoryFoundation 编辑写回后，后端 FilesJSON 投影要到下次任务投影才刷新；可在 WriteNovelTruth 成功后主动触发一次轻量重投影（列入 T3）。

## 2026-09-11 T3 首批落地：Job DB 镜像（历史+断线恢复）/ 单写入锁 / 会话管理

### 已实现（web typecheck 0 错误；Bridge build + test 5/5；后端 go build + database/handler 测试通过；Schema v12 已迁移）

- **Job DB 镜像（Schema v12，story_agent_jobs 表）**：
  - Bridge 每个 Job 的状态/阶段/日志（最近 100 条）/结果摘要（前 800 字）/归属（sessionId/bookId/mode/confirmedIntent）在每次后端轮询时 upsert 到 DB；任务发起时即预写 queued 行（轮询失败也可见）。镜像行在轮询路径自动回填归属字段。
  - 后端新增 `GET /api/novel-agent/jobs/history`（最近 30 条历史）与 `GET /api/novel-agent/jobs/active`（用户级 queued/running，断线恢复入口）。
  - 结果正文与产物不进 DB（Bridge 是执行真相，`/agent/jobs/:id/artifacts` 按需取回），DB 只承担"用户可见历史 + 恢复指针"，避免大对象入库。
- **断线恢复**：前端 /novel 加载时若无本地活动任务指针，自动查询 active 端点；发现 running 任务即接续轮询（1500ms）至终态，成功则投影提示/刷新书架，失败给出可读原因；恢复后刷新 active/history 缓存。服务重启场景沿用既有语义：Bridge 把中断任务标记为"可重试失败"，DB 镜像同步该终态。
- **单写入锁**（Bridge 进程内按书互斥）：写书类 confirmedIntent（write_next/draft_next/compose_chapter/revise_chapter/repair_state/create_script/create_storyboard/create_book/short_run）执行前按 bookId（无书时按会话）加锁；第二个任务立即失败并提示"该书已有生产任务在运行，请等待完成或取消后再试"，不再排队双写。实测并发两个同书 draft：一个进入执行、一个被锁拒绝。
- **会话管理**：Bridge `PUT/DELETE /agent/sessions/:sessionId`（renameBookSession/deleteBookSession，作用于该会话工作区）；后端代理并同步 Vergestar DB（story_agent_sessions 重命名 / 级联删除 messages+session）；前端会话行 hover 出现 重命名（prompt）/删除（confirm）操作。已实测重命名 200 且列表生效（改回原名）。
- **任务历史 UI**：左栏新增"任务历史"折叠块（状态图标 + intent/阶段 + 相对时间，最近 8 条，15s 静默刷新），与 active 恢复共用同一镜像数据源。

### 实机验收状态

- history/active 端点 200；快速终态任务（渠道 403 失败）镜像行即时出现在历史 UI。
- 单写入锁并发验证通过（见上）。
- 会话重命名 API + UI 通过；会话删除 UI 就绪（真实删除待用户自行操作，避免误删验收数据）。
- 断线恢复：发一个 running 任务 → 重载页面 → 自动接续轮询的路径已实现（代码路径与 active 数据源已验证）；完整"真任务长跑中断恢复"待渠道恢复后与 draft 落盘补验一并执行。

### 待办（随下一批）

1. 流式输出（token 流 + 工具状态 SSE）——T3 剩余大项。
2. 逐章进度时间线（多章写作时解析 chapterIndex 增量展示）。
3. 会话列表分页/搜索（Phase 1 残留项）。
4. 渠道类挂起验收（draft 落盘 / approve 带章 / consolidate / 导出真实产物 / 长跑断线恢复）待稳定渠道。

## 2026-09-11 T3 收尾：逐章进度时间线 + 会话搜索分页；渠道状态诊断

### 已实现（web typecheck 0 错误；后端 go build + handler 测试通过）

- **逐章进度时间线**：AgentTaskCard 新增 targetChapters 进度条——多章写作（write_next 确认卡选择 2/3/5/10 章）时，任务卡实时从 Job 日志提取"第 N 章"去重章号，显示 N/M 进度格子（已落盘章绿色高亮）；首页对话区任务卡与工作台 Writer 面板任务卡均已接入（Writer 面板任务展示同步升级为 InkOS 风格任务卡，含取消按钮）。
- **会话搜索/分页**（Phase 1 残留项完成）：后端 `GET /api/novel-agent/sessions/search?q=&page=&page_size=`（DB title LIKE + 分页 + total）；前端侧栏"会话记录"新增搜索框，输入即查（空查询回退原列表）。E2E：搜"轻喜剧"返回 1 条正确会话，前端搜索框交互验证通过。

### 渠道状态诊断（2026-09-11 实测，阻断所有内容生产类验收）

用户更换的新渠道与旧渠道实测结论：
- **subapi.easy-dotnet.com（新渠道）**：gpt-5.5 / gpt-5.6-auto / codex-auto-review 三个模型，chat/completions 与 /v1/responses 两种端点、stream/非 stream、max_tokens/max_completion_tokens 变体全部返回 400 `Upstream request failed`——渠道自身上游故障或 key 未开通任何模型，需渠道商侧排查。
- **apihub.agnes-ai.com（旧渠道）**：403 `insufficient_user_quota, remaining: $-0.000098`——余额耗尽，需充值。
- 结论：内容生产类挂起验收（draft 落盘、approve 带章、consolidate、导出真实产物、长跑断线恢复）全部等一个可用渠道；代码链路均已验证到位（任务正确进入 InkOS Writer 流程后才被上游拒绝）。

### T3 之后剩余（按总纲）

- 流式输出（token 流 + 工具状态 SSE）——T3 唯一剩余大项，建议与渠道修复后的长跑验收一起做。
- T4 扩展创作模式（同人/番外/仿写/续写/翻译/封面/市场雷达页）。
- T5 互动影游/开放世界。
- T6 影视化收尾（总纲 Phase 5 剩余：图片回填复验、视频首跑、批量镜头、ShotRevision 字段、音频）。

## 2026-09-11 T4 第一批落地：同人/续写/番外/仿写/翻译五模式接入

### 已实现（web typecheck 0 错误；Bridge build + test 5/5；后端 go build 通过；服务已重启）

- 接入方式与 short_run 不同：这五种模式在 InkOS Core 里是 chat 会话内的 propose_action 动作（core agent-session 的 chat 分支原生支持确认执行），因此 Bridge 只需放行与透传，不需要新生产分支——复用 core 原生工具（createFanficBookTool / createContinuationImportTool / createSpinoffBookTool / createImitationBookTool / createTranslationCreateTool）。
- Bridge：confirmedIntent 联合扩展 +fanfic_init/continuation_import/spinoff_create/style_imitation/translation_create；HTTP 入口白名单同步；确认提取器改为可扩展集合（EXTRACTABLE_ACTIONS）放行五种 proposed_action 并透传 actionPayload（core 的 ActionPayloadSchema 已原生校验这些 payload：fanficCreate 需 sourceText/sourcePath、imitationCreate 需 referenceText/referencePath 等）；单写入锁把这五种也纳入（按会话互斥）。
- 后端：ConfirmedIntent 校验白名单同步放行（五种不强制 bookId）。
- 前端：确认卡/请求类型扩展五种 action 与 actionPayload（fanficCreate/continuationImport/spinoffCreate/imitationCreate/translationCreate 完整参数类型，含 mode=canon|au|ooc|cp、platform、targetChapters 等）；左侧"同人创作/番外创作/仿写创作/翻译译介"模式按钮即选择对应 Skill，用户在对话中给出原作/参考文本后，Agent 发出对应确认卡，点"继续执行"即走 core 原生管线，产物（同人是独立书籍目录）经既有投影进入书架。
- 边界说明：continuation_import 需要 sourcePath（导入原文章节文件，走"上传附件"路径）；translation_create 需要 filePath（上传待译文档）。附件上传 UI 未接（InkOS 的上传按钮行为），首版可把文本直接放 sourceText/referenceText。

### 实机验收状态

- 机制链路（类型/白名单/透传/锁）已全部构建验证并重启生效；确认卡真实验收依赖文本渠道，继续挂起（见渠道状态）。
- 五模式中同人/番外/仿写成功后会创建新书籍 → 自动经既有投影进书架；翻译产物在 translations/ 目录（投影白名单需在验收时确认是否要放行 translations/*.md，暂未放行）。

### 渠道状态更新（2026-09-11 晚间复查）

- subapi.easy-dotnet.com：仍然全模型 400 `Upstream request failed`（models 端点正常返回 200 与完整模型列表，证明 key 有效、路径正确，纯上游故障）。需联系渠道商。
- apihub.agnes-ai.com：余额耗尽（$-0.000098），需充值。
- 图片模型（agnes-image-2.0-flash）同样被 agnes 余额阻断，Phase 5 图片回填验收继续挂起。

## 2026-09-11 T3 收官：流式输出（SSE token 流 + 工具状态）

### 已实现（web typecheck 0 错误；Bridge build + test 5/5；后端 go build + handler 测试通过；服务已重启）

- **事件管道（三层）**：
  1. core：pi-agent 的 message_update 携带 assistantMessageEvent.text_delta（token 增量）；tool_execution_start/end 携带工具名。full-agent 的 onEvent 处理器把三类事件归一为 onStreamEvent({type: "delta"|"tool"|"tool_end"}) 转发给宿主。
  2. Bridge：jobs.ts 每个 Job 配事件总线（有界缓冲 4000 条 + EventEmitter 实时推送）；`GET /agent/jobs/:jobId/stream` SSE 端点——先重放缓冲（迟订阅者补齐），再实时推送，终态发 done 后关闭。
  3. 后端：`GET /api/novel-agent/jobs/:jobId/stream` SSE 透传代理（novel_agent_stream.go，Gin 逐块 flush；SSE 用独立请求上下文，不复用 45 分钟轮询 client）。任务状态真相仍是轮询端点——SSE 是增量 UX，不改变镜像/恢复逻辑。
- **前端**：任务运行期间自动开 EventSource；delta 累积为打字机预览（AssistantBubble 实时渲染，截断保护 8000 字）；tool 事件显示"正在调用工具：X…"；任务终态自动关流、清预览（最终消息由轮询路径追加，保持单一追加语义）。
- **实机 E2E**：浏览器 EventSource → 后端 Gin 代理 → Bridge SSE → 事件总线，全链路验证通过——draft 任务运行中实时收到 log 事件（"阶段：准备章节输入/撰写章节草稿/阶段 1 创作正文"逐条到达）+ done 终态；规划任务验证 done 事件与缓冲重放。delta 事件与 log 走同一管道（同一 push 函数/同一 SSE 帧），当前因渠道空响应无真实文本可流；渠道恢复后首次成功生成即验证。

### 渠道状态（更新）

- easy-dotnet：规划回合实测模型返回 0 token 空响应（stopReason=stop、usage 全 0）——上游故障的另一表现，仍需渠道商处理。
- agnes：余额仍为负（-0.000098），403。

### 至此 T3 全部子项完成（任务持久化/断线恢复/单写入锁/会话管理/逐章进度/搜索分页/流式输出）。下一批：T4 剩余（封面生成入口、市场雷达页）与 T5 互动影游；内容类验收继续等渠道。

## 2026-09-12 渠道恢复后挂起验收清零 + mode 未透传重大 bug 修复

### 渠道可用性

- **api.aixoras.com（渠道 4）gpt-5.5 可用**：draft/导出/规划任务全部真实跑通。注意其长上下文调用偶发断流（一次 5 分钟无事件超时、一次 connection reset），重试即可成功，属渠道瞬时问题。

### 挂起验收清零（全部实机通过）

1. **draft 落盘 E2E**：仅草稿写完第 1 章《快三分钟之外》2824 字（target 2500，长度合格），状态 drafted；阶段链完整（准备输入→创作正文→提取事实→回写真相→落盘草稿与真相文件）；真相文件 current_state.json 已回写第 1 章事实。经后端轮询投影后 DB：ProjectUnit 第 1 章 ready、2984 字（含标题）。
2. **approve 带章 E2E**：`POST /story/projects/:id/inkos/chapters/1/approve` 200；章节状态 ready → completed（正史确认），InkOS 索引同步 approved。
3. **book-export + 导出面板**：修复后 exporter 真实产出 `三分钟未归_export.txt`（8525 字节，1 章 2824 词）；导出面板列出文件并可下载。**注意 exporter 实际落点为工作区根的 *_export.txt**（非 exports/ 目录），Bridge 导出列表已兼容扫描两处。
4. **断线恢复**：active 端点镜像正常；发现并补上**工作台侧恢复缺口**（此前恢复逻辑只在首页）——NovelWriterPanel 挂载时按 bookId 查 active 并接续轮询至终态（succeeded 且含 write_next 确认卡时自动弹出确认卡）；积压任务实测被恢复逻辑接续到终态。
5. **SSE 流式**：真实任务运行中 log 事件（阶段日志）逐条实时到达浏览器，done 终态正常；delta 管道同一实现。

### 本轮发现并修复的重大 bug：Bridge HTTP 层 mode 字段未透传

- `POST /agent/jobs` 与 `POST /agent/turn` 的 body 解析从未透传 `mode`——导致 **book-audit / book-export / book-radar / repair-state 的模式化提示词与会话分支从未生效**（全部走了默认 Writer 规划提示）。此前导出/审计的"验证通过"实为模型在错误提示词下的碰巧应答。
- 已修复（两处 body 解析透传 mode）并重跑验证：修复后 exporter 真实执行（此前模型明确报告"没有导出工具"）。
- 影响面复盘：repair_state 的确认执行不受影响（confirmedIntent 分支驱动）；受影响的是规划回合提示词选择与 audit/export/radar 三条命令回合——本次全部修正。

### 遗留小项

- gpt-5.5 偶发断流（2b 阶段）建议 core 层对连接中断加一次自动重试（此前 completeLongForm 只覆盖 output-limit）。
- 审计（book-audit）回合已随 mode 修复生效，带章审计输出待下次审计时顺带查看。

## 2026-09-12 T4 剩余落地：市场雷达页 + 封面生成入口 + 互动影游接入

### 已实现（web typecheck 0 错误；Bridge build + test 5/5；后端 go build 通过；服务已重启）

- **市场雷达**（T4 收官项）：
  - Bridge `POST /agent/books/radar`：直调 core 原生 `PipelineRunner.runRadar()`（RadarAgent，抓取番茄等平台实时排行榜 + LLM 市场分析），调用方传模型配置（与 consolidate 同模式）。
  - 后端 `POST /story/projects/:projectId/radar`（透传，novel_radar.go + story.go 路由）。
  - 前端工作台"故事资料"面板新增"市场雷达"按钮（与"书籍评估"并排，取当前文本模型配置调用），结果 toast 展示。
  - **实机 E2E 通过**：真实抓取番茄小说热门榜/黑马榜约 60 条 + gpt-5.5 分析——产出市场总结（女向强情绪、仙侠幼崽团宠统治力、题材扎堆风险）与带置信度的开书推荐（0.9 置信度仙侠幼崽案）。完整可用。
- **封面生成入口**（book-cover 命令回合）：Bridge 新 mode `book-cover`——豁免生产工具压制 + 专用提示词（调 generate_cover 工具，先读 brief/book.json 提取卖点）。工作台入口下一批随 UI 整理加入（当前可在聊天让 Agent 生成）。图像模型配置齐后即可真实出图。
- **互动影游接入（T5 前哨）**：confirmedIntent + `interactive_film_create`；Bridge sessionKind 映射 `interactive-film`（core chat 分支原生确认工具 createInteractiveFilmCreationTool）；HTTP/后端/前端白名单与 payload 类型（requirements/episodeCount/budget 等）齐备。操作路径与五模式一致：互动影游模式对话 → 确认卡 → core 原生 runInteractiveFilmCreation。
- Play 开放世界（PlayRunner/StoryGraph/HUD）仍是 T5 剩余大头，未动。

### 渠道备注

- 渠道 4（api.aixoras.com）gpt-5.5 文本稳定可用；偶发断流重试即可。
- agnes（文本+图片）仍欠费；easy-dotnet 上游仍故障。封面真实出图与五模式验收可随任一渠道恢复进行。

## 2026-09-12 规划任务 120s 超时与路径误读问题修复（gpt-5.5 适配）

### 问题与修复

1. **规划任务 120 秒流空闲超时**（用户 UI 实测 21m59s 失败，错误 `LLM stream produced no event within 120000ms`）：gpt-5.5 是推理模型，规划回合读取全书状态后思考期长，首 token 超 120s 默认空闲超时。
   - **修复**：core 原生支持环境变量覆盖——Bridge 启动时注入 `INKOS_LLM_FIRST_EVENT_TIMEOUT_MS=600000` 与 `INKOS_LLM_STREAM_IDLE_TIMEOUT_MS=600000`（10 分钟）。该变量同时覆盖 pipeline 路径的 300s 默认（draft 的 5 分钟超时同源）。
   - 注意：超时变量只作用于 Bridge 进程；`.local/start-vergestar.ps1` 与手动起服都应带上（已在本机运行环境生效）。
2. **gpt-5.5 书籍路径误读**：修复超时后规划任务跑通但模型报告 `story/current_state.md` 等不存在而放弃——它直接读 `story/` 前缀，未加 `books/<书籍ID>/` 前缀（此前 agnes 模型熟悉 InkOS 路径约定，gpt-5.5 需要）。
   - **修复**：Writer 规划回合提示词补充路径说明（读状态用 books/<书籍ID>/story/... 前缀）。
3. **修复后完整验收**：write_next 规划任务成功——正确读取第 1 章状态，生成确认卡《确认续写第 2 章》（summary 准确引用冰柜霜纹、停帧回放、老陈交班权限等第 1 章真实伏笔，actionPayload.writeNext.chapterCount=1）。从 UI"让 Agent 写下一章"到确认卡的全链路在 gpt-5.5 下可用。
4. **单写入锁实战验证**：规划任务运行中重复发起被正确拒绝（"该书已有生产任务在运行"），锁语义符合预期。

### 运行提示

- 若出现同类超时，先确认 Bridge 进程是否带超时环境变量；模型侧 gpt-5.5 思考期长是行为特性，非故障。
- 后续规划/审计/导出提示词若遇其他模型的路径或工具误用，按同一模式在 backgroundTaskContext 补充说明。

## 2026-09-12 gpt-5.5 下 write_next 第 2 章完整闭环验收 + sub_agent bookId 提示修复

### 验收结果（三连发后成功，渠道断流为概率性）

- write_next 确认卡《确认续写第 2 章》（准确引用第 1 章伏笔：冰柜霜纹/停帧回放/老陈交班权限）→ 确认执行 → Writer 生产链完整跑通：准备输入 → 创作正文（第2章） → 提取事实 → 回写真相 → 校验真相文件变更（一次重试后通过） → 落盘。
- 产物：第 2 章《霜线不止》2692 字，**状态 audit-failed**（Auditor 审稿未通过——质量门禁真实工作，不是装饰）；投影后 DB：第 2 章 ProjectUnit draft、2898 字（含标题）；第 1 章 completed 保持不变。
- 前两次尝试失败原因：渠道断流（Upstream request failed）+ gpt-5.5 调 sub_agent 时自填错误 bookId。

### 新增修复：write_next 执行回合补 sub_agent bookId 提示

- gpt-5.5 调 sub_agent(agent=writer) 时自填了与 active book 不匹配的 bookId → `writer.bookId must match the active book`。Bridge 在 write_next 确认回合的 backgroundTaskContext 明确要求"调用 sub_agent 时必须省略 bookId 参数（会话已绑定当前书）"。
- 经验：换模型时 sub_agent 参数自填是最常见的不适配点，提示词约束优先于改 core。

### 现状

- 《三分钟未归》2 章在库：第 1 章 approved/completed、第 2 章 audit-failed/draft。第 2 章的复核路径：审稿与修订视图 → revise（带审计问题的修订）或 rejection 处理——这正好是下一轮 Reviser 链路的实机验收素材。
- Play 开放世界代码已接入待验收（play_start 确认卡需一次真实世界构建）。

## 2026-09-12 第 2 章 Reviser 修订验收：门禁保留语义实证 + 运维固化

### Reviser 修订验收（27341cfd）

- 以 spot-fix 模式发起第 2 章修订（instruction 携带三条具体审稿问题：老陈交班权限伏笔回收、66/98 短段合并、不新开无回收伏笔）。
- **管线完整走通**：加载修订上下文 → planner 章节意图（gpt-5.5 格式抖动触发 core 内部 memo 重试自愈）→ 修订第 2 章 → 提取事实 → 回写真相 → 状态校验。
- **结果：修订门禁保留第 2 章原文**（"Revision kept the original chapter because state settlement did not validate after retry"）——状态结算在校验阶段未通过，门禁按设计拒绝落盘，未损坏正史。这是 InkOS"生成不等于合格"语义的正确呈现，不是缺陷。
- DB 复核：第 2 章保持 draft/2898 字未被覆盖，第 1 章 completed 不变。
- 结论：Reviser 链路 + 门禁保留语义在 gpt-5.5 下实证通过；修订成功案例依赖渠道状态结算稳定性（当前 aixoras 渠道在长上下文结算调用上断流概率偏高）。

### 运维固化

- `.local/start-vergestar.ps1` 已固化 `INKOS_LLM_FIRST_EVENT_TIMEOUT_MS=600000` 与 `INKOS_LLM_STREAM_IDLE_TIMEOUT_MS=600000`，脚本启动与手动启动行为一致。

### 待验清单（下轮）

1. 第 2 章修订成功案例（等渠道结算稳定，或在更强模型下重试 spot-fix）。
2. Play play_start 确认卡（世界构建）真实验收。
3. 五模式（同人/番外/仿写/续写/翻译）确认卡验收。
4. 封面真实出图（待图片渠道）。

## 2026-09-12 交互修复：模式按钮开启新会话

- 问题：首页左侧创作模式按钮（短篇小说/剧本创作/分镜创作/互动影游等）只切换 Skill 标签，聊天区仍显示自动恢复的最近会话历史（含旧 Validation failed 错误记录），用户误以为功能相同。
- 修复：点击模式按钮即调用 startNewSession()——清空对话区显示欢迎语、进入该模式的全新会话（会话记录高亮清除，发首条消息时创建会话）。与 InkOS 行为一致：模式 = 新创作入口。
- 顺带说明：历史中的 "Validation failed for tool propose_action" 条目为早期 propose_action 参数错误任务的持久化错误记录，属正常历史，切换新会话后不再显示。
- 回归：typecheck 通过；浏览器验证点击剧本创作后旧内容清空、欢迎语出现、Skill 切换为 inkos-script-writing。

## 2026-09-12 T5-Play 验收：play_start 世界构建通过 + 游玩交互缺口

### 验收结果

- **play_start 确认卡**：首页"开放世界"模式发起《悬镜市：雾中失踪案》世界构建——确认卡完整（世界契约/自由行动模式说明/不替玩家解谜约束），确认后 1m19s 构建成功。
- **Play 工作区全套产物**：worlds/<id>/world.json + runs/main/play.db（PlayGraphDB 图数据库）+ projections/scene.md、state.md（场景与状态投影）+ transcript.jsonl。开场剧情含悬疑氛围、裁判 AI 环境压力、【建议前往：七号码头冷库】建议行动。
- **结论：Play 管线（world 构建 + 图数据库 + 投影）在 Bridge 侧完整可用。**

### 发现的缺口（下轮修复）

1. `POST /agent/turn` 也未透传 playMode（与 mode 未透传同型问题——已修 jobs 端，turn 端待补）；游玩交互首回合返回空文本疑似与此相关或为渠道空响应，待复验。
2. 游玩富 UI：当前游玩在聊天流内进行（文字 + 建议行动在正文中）；InkOS 式 PlayHUD/独立建议动作按钮/世界图片待做。
3. Play 世界到 Vergestar 项目/画布的投影未接（世界数据以 PlayStore/GraphDB 为真相留在工作区）。

## 2026-09-12 T5-Play 游玩交互闭环验收通过

- play_step 实机验证：玩家行动（推门进办公室、出示 K-17 门牌、询问档案）→ 世界实质响应（值班员脸色变化、"三号码头东侧"红圈线索、日光灯环境细节，518 字）；state.md 状态投影同步更新（action/summary 记录行动与新增线索）。
- `POST /agent/turn` 与 `POST /agent/jobs` 的 playMode 透传均已修复（此前各漏一处）；Bridge 已带双 10 分钟流超时重启。
- 注意：play 回合耗时长（gpt-5.5 实测 >2 分钟），前端 axios 无超时限制可正常等待；直连测试需自备长超时。
- **T5 Play 第一期（会话内文字游玩闭环）至此验收通过。** 剩余：游玩富 UI（PlayHUD/建议动作按钮/世界图片）、Play 世界投影到 Vergestar 项目、变体回放。

## 2026-09-12 修复：确认动作（Job）结果消息持久化缺失

- 根因：消息持久化只存在于同步回合（RunNovelAgentTurn）路径；确认动作走异步 Job 后，结果消息（含确认卡、建书结果、Play 开场、写作结果）从未落库——**刷新页面即丢**，Play 开场文本不可回放、建书结果消失均源于此。
- 修复：
  1. StartNovelAgentJob 受理成功即落用户消息（会话行不存在则创建）；
  2. Job 轮询到达 succeeded 时，以 job.ID 为幂等键补写助手消息（含 text/skills/confirmation），重复轮询不重复写入；
  3. 新会话首条（SessionID 为空）跳过用户消息预写，由结果落库统一兜住。
- 影响面：此后所有确认动作（建书/写作/短篇/剧本/分镜/修订等）的结果与确认卡在刷新后均可回放；历史存量任务（修复前完成）不在补录范围。
- 回归：go build + handler/database 测试通过；后端已重启生效。

## 当前已知限制


















- InkOS 的最终状态校验允许正文先完成、真相文件后以 `state-degraded` 进入可审阅状态；`repair_state` 人工修复入口已实机验收，修复只重建派生状态，不改写正文。
- 后端完整 `go test ./internal/service` 仍有项目原有的 `TestResourceDeletionWorkerRemovesObjectAndCompletesOutbox` 异步资源删除失败；数据库、handler、Novel Agent 聚焦测试通过。
- Writer 任务当前仍使用 Bridge 本地 Job 元数据，尚未完全迁移为 Vergestar Task/SSE；服务重启会将运行中任务标记为可重试失败。
- 当前自定义图片渠道 `api.aixoras.com` 已可被正常网络上下文中的后端访问，但 `gpt-image-2` 本次验收请求返回供应商 524。为避免可能重复计费，系统不会自动重试；成功返回一次后即可完成 `shot_artifacts` 的真实媒体回填验收。







