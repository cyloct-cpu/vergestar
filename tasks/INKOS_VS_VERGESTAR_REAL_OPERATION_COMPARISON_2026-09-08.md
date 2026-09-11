# InkOS 与 Vergestar 小说创作真实操作对照报告

日期：2026-09-08  
目的：用真实浏览器操作和本地运行证据判断 Vergestar 与 InkOS 的实际差距，避免仅凭代码或设计文档下结论。

## 一、实验边界

- InkOS 参考源码：`D:\BYW\inkos`
- InkOS Studio 实例：`http://127.0.0.1:4567`
- Vergestar：`D:\BYW\vergestar`
- Vergestar Web：`http://127.0.0.1:13000/novel`
- Vergestar Novel Agent Bridge：内部 `127.0.0.1:17421`
- InkOS 本机服务只作为观察对象，没有接入 Vergestar 运行链路。
- InkOS 实测新建对照书：《对照实验：雨夜档案》；没有修改已有《都市无敌仙尊》。
- Vergestar 实测输入使用同一故事想法，并通过已有登录会话发送。

## 二、InkOS 真实流程

### 1. 从 0 开始

在 InkOS 点击“长篇小说”后，系统新建独立会话，空白状态只显示：

```text
告诉我你想写什么——题材、世界观、主角、核心冲突
添加 Skill
上传图片或资料
模型选择
```

用户只提交一段自然语言：

```text
我要创作一部中文长篇小说《对照实验：雨夜档案》，都市悬疑题材。
主角周宁是夜班档案管理员，在一份被系统注销的失踪人员档案中发现自己的签名。
第一阶段计划 3 章，每章约 800 字。请先建立创作方案并告诉我下一步。
```

没有先填写书名、题材、章节表单。

### 2. Agent 组织创作方案

InkOS 返回的不是泛泛建议，而是结构化方案，至少包括：

- 书名和题材；
- 目标平台；
- 章节数、单章字数、语言；
- 世界观与规则；
- 主角压力；
- 核心冲突；
- 第一阶段各章的推进钩子；
- 视角、节奏和禁忌。

同时生成结构化确认动作：

```text
建立长篇创作方案：对照实验：雨夜档案
确认后将创建……
继续执行
取消
```

### 3. 确认后运行生产 Pipeline

点击“继续执行”后，InkOS 展示一个真实生产任务，而不是让聊天请求一直等待：

```text
建书  执行中
○ 生成基础设定
○ 保存书籍配置
○ 写入基础设定文件
○ 初始化控制文档
○ 创建初始快照
```

实测约 5 分钟后进入 FoundationReviewer：

```text
阶段：审核基础设定（第1轮）
Foundation review: 63/100 REJECTED
基础设定未通过审核，正在重新生成...
```

审核维度包括：核心冲突、开篇节奏、世界一致性、角色区分度、节奏可行性。低分不会直接当成成功交付，而是进入重试。

此前对另一部 InkOS 建书的实测还观察到：

```text
68/100 REJECTED
78/100 REJECTED
```

这说明拒绝和重试是稳定的生产机制，不是偶然 UI 文案。

### 4. 建书后的产物和工作台

InkOS 建书结果会进入书籍工作台。真实可见对象包括：

- 章节列表；
- 角色列表，并区分主要/次要；
- 核心文件：故事基石、卷纲规划、当前状态、伏笔池、情感弧线、支线进度；
- 世界观阅读面板；
- “写下一章”“审计”“导出”“市场雷达”等 Agent 生产动作；
- 每个任务的耗时、状态、Skill、参考依据、操作结果；
- 失败任务、失败原因、复核入口和再次写作入口；
- 会话记录和独立新会话。

### 5. 写作和审稿闭环

已有 InkOS 书籍中真实看到：

```text
写作 · 都市无敌仙尊 · 36m 58s · 失败
专业 Skill inkos-long-writing
本轮参考依据：第 1 章 · sqlite-fts5-bm25
查看操作结果
已写出第 1 章……字数 2656，但审稿未通过，状态 audit-failed
```

审计输出包含：

- 空间关系不清；
- 冲突强度和爽点不足；
- 配角功能化；
- AI 句式标记；
- 术语密度；
- 伏笔推进状态；
- 敏感词阻断。

也有第二章 `state-degraded` 状态，说明 InkOS 不把“生成出文本”当作“质量合格”。

## 三、Vergestar 真实流程

### 已实际具备

Vergestar `/novel` 当前能完成：

1. 进入 Novel Agent 对话区；
2. 选择长篇小说、短篇小说、剧本、分镜等入口；
3. 选择一个 Skill；
4. 输入自然语言创作想法；
5. 调用 Novel Agent；
6. 返回 `propose_action` 确认卡；
7. 点击“确认创建”；
8. 创建后台 Job；
9. 运行 InkOS `runAgentSession`、`sub_agent(agent="architect")` 和 `PipelineRunner`；
10. 运行 FoundationReviewer，并在拒绝后重试；
11. 写入 InkOS 书籍工作目录；
12. 最近已加入基础设定导出和 Vergestar `StoryFoundation` 同步代码。

### Vergestar 真实观测到的缺点

- 首页的“我的创作”只显示 Vergestar 既有 Project，不像 InkOS 那样显示 Agent 会话、书籍会话和任务历史；
- Skill 当前主要是左侧固定选择，不是 InkOS 那种由 Agent 根据意图调用 `use_skill`，并展示本轮激活 Skill；
- 对话区只显示确认卡摘要，不展示完整结构化建书方案的分层阅读体验；
- 生产任务依赖前端轮询，阶段日志已接入但仍不是完整 SSE/事件流；
- Bridge 的 Job 仍保存在 Node 内存 Map，Bridge 重启后任务状态会丢失；
- 刷新页面会丢失当前 React 对话历史；已增加活动 Job ID 恢复尝试，但完整会话恢复尚未完成；
- InkOS 的“核心文件、章节、角色、伏笔、情感弧线、支线进度”尚未成为 Vergestar 小说项目的完整可视化工作台；
- InkOS 建书产物到 Vergestar 项目的同步目前只完成 `StoryFoundation` 基础设定投影代码，尚未完成角色资产、章节计划、故事记忆、版本和场次的全量映射；
- Vergestar 当前模型调用使用的是前端解析后的渠道配置，实测过程中曾因旧后端数据目录、端口占用和旧令牌造成 401/Connection error，说明启动编排和配置单一来源还不够稳定；
- Vergestar 还没有 InkOS 那种写作→审稿→修订→状态降级/恢复的连续生产体验；当前已有审稿、版本、记忆、分支原生数据表，但主要仍是人工操作面板；
- Vergestar 的异常任务与错误展示仍需要按任务维度保存原始阶段、失败原因、重试次数和来源依据。

## 四、功能差异矩阵

| 能力 | InkOS 实测 | Vergestar 当前实测/代码 | 差距 |
|---|---|---|---|
| 自然语言起稿 | 完整 | 已有 | 小 |
| 自动理解创作意图 | 有 | 目前强依赖固定 Skill/模式 | 中 |
| Agent 自动 Skill 选择 | `use_skill`、会话内激活 | 主要是左侧手动 Skill | 大 |
| 结构化创作方案 | 完整展示世界观、冲突、章节方向 | 主要显示摘要和确认卡 | 大 |
| 生产确认 | 完整 | 已有 `确认创建` | 小 |
| 生产任务 | 任务卡、耗时、阶段、结果 | Bridge Job + 前端轮询 | 中 |
| Architect | 已有 | 已接入并实测 | 小 |
| FoundationReviewer | 已有多轮评分、拒绝、重试 | 已接入并实测 | 小 |
| 书籍配置 | 真实书籍实体 | 已生成 InkOS book.json | 中 |
| 核心文件 | 可读面板 | 主要停留在隐藏工作目录/基础投影 | 大 |
| 章节规划 | 章节列表、章节状态和依据 | Vergestar ProjectUnit 尚未自动导入完整规划 | 大 |
| 角色资产 | 角色卡、主要/次要、持续使用 | 原生角色 API 存在，尚未由 Agent 自动同步 | 大 |
| 故事记忆 | 自动整理会话记忆、当前状态、伏笔和情感弧线 | 有 StoryMemory 表，但主要人工添加 | 大 |
| 写作 | Writer + 状态文件 + 质量门禁 | 尚未完成连续 Writer 闭环 | 很大 |
| 审稿 | Auditor 真实输出问题并阻断 | 有手工 StoryReview，Agent 审稿尚未闭环 | 很大 |
| 修订 | Reviser 根据审稿问题返工 | 尚未接入生产 Agent | 很大 |
| 版本回溯 | 快照和状态文件联动 | 有原生章节版本 API，尚未和 Agent 快照打通 | 大 |
| 多分支 | 分支互动/预测/选择 | 有 StoryBranch 表，未接 Agent 分支工具 | 大 |
| 影视转化 | Script/Storyboard/interactive-film 入口和 Agent 工具 | 既有短剧/画布能力存在，但小说到场次/镜头尚未 Agent 化 | 很大 |
| 失败恢复 | 任务失败、降级、复核、重试 | 需要持久化 Task/SSE | 大 |
| 会话历史 | 书籍、会话记录、新会话 | Vergestar 页面内存状态为主 | 大 |
| 证据与来源 | 参考依据、检索方式、Skill、状态 | 目前只部分保存来源哈希和基础设定 | 中到大 |

## 五、差距的根本原因

不是同一个模型导致的主要差异。真实证据表明，在 InkOS 和 Vergestar 使用相同的 `BAI · qwen3.8-flash` 时，决定输出差异的主要因素是：

1. InkOS 有面向不同生产阶段的 Agent Harness 和角色分工；
2. InkOS 有 `PipelineRunner`，把 Architect、Reviewer、Writer、Auditor、Reviser 组织为有状态生产流程；
3. InkOS 有大量可读/可写的故事真相文件；
4. InkOS 把任务状态、耗时、Skill、检索依据、错误、重试和结果都呈现在 UI；
5. InkOS 通过质量门禁阻止低质量结果直接进入正史；
6. Vergestar 当前虽然已经接入 Core 的关键底层，但前端产品层、数据库投影层和可恢复任务层还没有完全承接 InkOS 的生产语义。

## 六、当前完成度判断

按“能否像 InkOS 一样从想法生产一本可继续写的小说”判断：

- Vergestar 的 Agent-first 入口：约 60%～70%；
- 建书 Harness：约 70%～80%，因为 Architect、Reviewer、重试已经真实运行；
- InkOS 书籍工作台：约 25%～35%；
- 连续长篇写作：约 15%～25%；
- 审稿修订闭环：约 15%～25%；
- 记忆/版本/分支 Agent 化：约 20%～30%；
- 小说到剧本/分镜/视频的完整生产链：约 15%～25%。

这些是工程完成度估计，不是模型质量评分；依据是本次真实 UI 操作、运行日志、源码结构和已生成文件。

## 七、必须按此顺序继续重构

### P0：生产任务和会话持久化

- 将 Bridge 内存 Job 迁移到 Vergestar Task/SSE 或独立持久化任务表；
- Job 必须有 user、session、intent、status、stage、retryCount、source、resultRef；
- 服务重启后可恢复/重试/标记未知状态；
- 页面刷新后恢复会话、确认卡和活动任务；
- 每本书只允许一个生产写入会话。

### P1：把 InkOS 书籍结构投影到 Vergestar

- `book.json -> Project`；
- `story/outline/* -> ProjectUnit` 或专用 Foundation/Outline 记录；
- `story/roles/* -> Character Asset`；
- `current_state/pending_hooks/emotional_arcs -> StoryMemory`；
- snapshot -> `StoryChapterVersion`/Foundation snapshot；
- 保存 `inkosBookId`、`sourceHash`、文件相对路径和导入批次。

### P2：复刻 InkOS 生产工作台交互

- 书籍会话列表；
- 核心文件面板；
- 章节计划与章节状态；
- 角色/场景/伏笔/情感弧线面板；
- 每个操作的 Skill、参考依据、耗时、结果、失败原因和重试；
- Writer/Auditor/Reviser 任务卡。

### P3：接入连续长篇 Agent

- `plan_story_chapter`；
- `write_story_chapter`；
- `audit_story_chapter`；
- `revise_story_chapter`；
- `settle_story_memory`；
- 自动版本快照和失败降级；
- 质量不合格时不推进正史。

### P4：接入影视生产

- 小说章节确认后生成剧本场次；
- 场次确认后生成镜头/分镜；
- 角色和场景资产自动引用；
- 画布、图片、视频、音频任务保持来源链；
- 下游内容过期时标记 stale，不静默覆盖。

## 八、结论

Vergestar 目前已经不再是最初的“手工章节编辑器”，因为完整 InkOS Architect/FoundationReviewer 建书链路已真实接通；但它仍然不是完整 InkOS 产品体验。

最大差距不在“模型有没有选对”，而在：

```text
InkOS = Agent Harness + Skills + Pipeline + Story Truth Files + Quality Gates + Durable UX
Vergestar 当前 = InkOS Core 建书能力 + Vergestar 原生数据基础 + 尚未完成的工作台/持久化/连续生产闭环
```

因此，后续不能继续增加孤立的手工表单功能；应优先完成 P0/P1，再实现 Writer/Auditor/Reviser 和核心文件工作台。只有这样，用户从一句想法开始，才能真正得到一部可持续创作、可审稿修订、可转影视的小说。
