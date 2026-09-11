# Spec: 小说创作工作区（已被 Agent-first 总纲取代）

> 本规格描述早期手工小说工作区。后续开发必须以 `tasks/INKOS_AGENT_INTEGRATION_MASTER_PLAN.md` 为准；本文件仅保留已完成基础层的历史边界。

## Objective
在影策增加独立的“小说创作”工作区。用户能够由一个想法创建小说项目，管理章节，并将稳定的章节内容交给既有短剧、分镜和画布流程。

## Phase 1 Scope
- 小说项目以现有 `Project(type=novel)` 和 `ProjectUnit(kind=chapter)` 持久化。
- 新增 `/novel` 路由、侧栏入口和小说项目创建/浏览界面。
- 小说项目可进入现有项目工作台和画布；不复制 InkOS Studio，也不引入其 AGPL 代码。

## Boundaries
- Always: 复用现有用户、权限、API 客户端和项目数据模型；小说内容不可跨用户读取。
- Ask first: 将 InkOS `AGPL-3.0-only` 核心作为产品运行时依赖或对外分发。
- Never: 前端直接读写服务器文件；将 API Key 写入小说正文或项目描述。

## Success Criteria
- 已登录用户可从侧栏打开“小说创作”。
- 输入名称、创作想法、类型和语言后，系统创建可持久化的小说项目。
- 小说项目能进入既有章节工作台和画布入口。
- 前端构建与相关测试通过，浏览器可复验该路径。
