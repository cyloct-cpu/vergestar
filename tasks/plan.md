# Implementation Plan: 小说创作工作区（已被 Agent-first 总纲取代）

> 后续工作请先读取 `tasks/INKOS_AGENT_INTEGRATION_MASTER_PLAN.md`。本文件只记录早期手工工作区已经完成的基础数据层。

## Architecture Decisions
- 第一阶段以既有 Project/ProjectUnit 为唯一业务存储，避免双写和未评估的 InkOS AGPL 运行时耦合。
- 后续小说专业能力通过 Story 服务边界接入；模型、任务、权限继续由影策统一管理。

## Task List
### Phase 1
- [x] Task 1: 增加小说导航、路由和项目创建入口。
- [x] Task 2: 为小说项目补充章节创建、编辑和剧本/画布转换体验。
- [x] Task 3: 验证 API、前端构建和浏览器路径。

### Subsequent Modules
- [x] 审稿记录、章节版本快照/恢复与隔离分支的原生持久化基础。
- [x] 故事记忆的原生持久化基础。
- [x] 章节到剧本场次的原生数据层与 API 基础。
- [x] 场次到既有镜头和分镜生产的来源链接基础。
- [ ] 角色与场景资产的小说工作区界面与镜头映射。
- [ ] 视频生成与来源追踪。
- [x] 多分支故事候选计划的原生持久化基础。

## Risks
- InkOS 为 AGPL-3.0-only：在产品内直接引入前必须完成许可评估。
- 小说内容改变后下游剧本和分镜会过期：后续模块必须保存来源哈希与链接。
