# 插件开发协议说明

影策插件采用“一份 Manifest 声明全部能力”的方式开发。一个插件包可以同时贡献渠道 Provider、工作流、画布节点、转换器、素材源、Agent、命令等能力，但它们共享同一个版本、权限、启用状态和卸载生命周期。

当前协议版本是 `yingce.plugin/v1`。上传插件使用 ZIP 容器，后端按 Manifest 做校验和执行映射，不会执行上传包中的业务代码。

## 1. 现在能做什么

上传插件当前可稳定使用的能力是：

- 通过 `contributes.providers` 接入一个 HTTP 生成接口，并把影策统一生成请求映射成上游请求。
- 通过 `create`、`poll`、`cancel` 支持同步任务或异步任务。
- 通过 `resultKind` 把返回结果识别为 `image`、`video` 或 `audio`。
- 通过 `requiresPublicMediaUrls` 让宿主在调用前把本机/内部素材转换成短期公网 URL。
- 通过 `resultEphemeral: true` 让宿主在拿到短期结果 URL 后立即下载并转存。
- 通过 `contributes.workflows` 描述某个 Provider 下的具体工作流或模型入口。
- 通过 `contributes.canvasNodes` 注册 schema 驱动的画布节点。
- 通过 `contributes.transforms`、`assetSources`、`commands`、`agents` 等字段声明后续贡献面。

当前内置插件包括 RunningHub 工作流、ComfyUI Bridge 工作流、Eagle 素材连接器、AI 提示词优化器和肖像清理。这些插件不是通过上传包安装的，而是作为宿主内置插件在前端注册。自定义上传插件走的是后端统一插件协议。

## 2. 包格式与限制

上传文件必须是 `.yingce-plugin`，本质是一个 ZIP 包。总大小最大 `16 MiB`，最多 `256` 个文件，单个文件最大 `8 MiB`，`manifest.json` 最大 `512 KiB`。

包内根路径允许以下内容：

```text
my-plugin.yingce-plugin
├── manifest.json            # 必须存在
├── web/entry.js             # 可选；声明 Web 入口时必须存在
├── web/assets/...           # 可选静态资源
├── assets/...               # 可选静态资源
├── docs/...                 # 可选文档
├── README.md                # 可选
└── LICENSE                  # 可选
```

校验规则：

- 不允许根目录外的其他路径。
- 不允许绝对路径、`..` 路径、反斜杠路径或符号链接。
- 不允许重复文件。
- `manifest.json` 必须是 UTF-8 JSON。
- 如果 Manifest 声明了 `entry`，它只能指向包内 `web/` 文件，并且文件必须真实存在。
- 如果声明了 `entry`，还必须声明 `runtime.web` 为 `sandbox` 或 `worker`。
- 如果包里有 `web/` 文件但没有声明 `entry`，上传会失败。
- Manifest 不能包含 Cookie、Token、API Key 等敏感信息。

## 3. Manifest 基础结构

一个最小但可用的生成渠道插件如下：

```json
{
  "apiVersion": "yingce.plugin/v1",
  "id": "acme-generation",
  "name": "Acme Generation",
  "version": "1.0.0",
  "author": "Acme",
  "description": "接入 Acme 图片生成接口",
  "permissions": ["generation.run", "media.read"],
  "contributes": {
    "providers": [
      {
        "id": "acme-image",
        "label": "Acme Image",
        "capabilities": ["image"],
        "scopes": ["user.custom-channel", "canvas"],
        "baseUrl": "https://api.acme.example.com",
        "auth": { "type": "bearer", "field": "apiKey" },
        "create": {
          "method": "POST",
          "path": "/v1/images",
          "fields": {
            "prompt": "request.prompt",
            "aspect_ratio": "request.aspectRatio"
          }
        },
        "response": {
          "resultPaths": ["data.images"],
          "resultKind": "image",
          "resultEphemeral": true
        }
      }
    ]
  }
}
```

关键字段：

| 字段 | 说明 |
| --- | --- |
| `apiVersion` | 当前必须是 `yingce.plugin/v1`。 |
| `id` | 插件全局唯一 ID。小写字母、数字、`-`、`_`、`.`，最长 96 字符，首字符不能是分隔符。 |
| `name` | 插件名称，最长 160 字符。 |
| `version` | 插件版本。 |
| `author` / `vendor` | 作者或供应商，最长 120 字符。 |
| `description` | 插件说明。 |
| `documentation` | 详细文档，可以写成 Markdown。 |
| `permissions` | 插件需要的最小权限集合。 |
| `configuration.fields` | 用户或管理员需要填写的安全配置项。 |
| `contributes` | 所有能力贡献点。 |
| `entry` | 可选 Web 入口。当前主要用于预留隔离运行时合同。 |
| `runtime.web` | 声明 Web 运行时，必须是 `sandbox` 或 `worker`。 |

ID 至少要声明一个贡献点；没有 `providers` 时，也可以只声明其他贡献。

## 4. 权限

当前权限枚举：

- `canvas.read`
- `canvas.write`
- `asset.read`
- `asset.search`
- `asset.import`
- `asset.upload`
- `generation.run`
- `ai.text`
- `media.read`
- `usage.read`
- `external.open`

插件应只申请实际需要的权限。上传插件不会自动获得宿主完整权限，权限不足的贡献能力不会生效。

## 5. Provider 能力

`providers` 是上传插件最核心的能力，用于把影策统一生成请求映射到外部 HTTP API。

### capabilities

支持：

- `text`
- `image`
- `video`
- `audio`

### scopes

支持：

- `admin.system-channel`
- `user.custom-channel`
- `canvas`
- `creation`
- `agent`

`user.custom-channel` 表示插件可以出现在用户自定义渠道侧。`admin.system-channel` 表示可以作为系统渠道配置的一部分。`canvas`、`creation`、`agent` 表示能力可以在画布、创作或 Agent 场景中使用。

### baseUrl 与鉴权

`baseUrl` 写到接口根地址即可。请求路径必须写在 `create`、`poll`、`cancel`、`agent` 里，并且必须是相对路径。

鉴权由宿主统一注入。插件 Manifest 中的 `auth` 是声明，真正的 Key 保存在渠道配置里，不会出现在 Manifest、前端日志或任务正文中。

示例：

```json
{
  "baseUrl": "https://api.acme.example.com",
  "auth": { "type": "bearer", "field": "apiKey" }
}
```

宿主默认注入 `Authorization: Bearer <apiKey>`。部分内置协议有特殊 Header，例如 Claude 或 Gemini 类接口，但自定义上传插件应优先使用标准 Bearer 或简单 API Key Header。

## 6. 请求映射

`create.fields` 描述上游请求体。左侧是上游字段名，右侧是影策统一请求表达式。

示例：

```json
{
  "create": {
    "method": "POST",
    "path": "/v1/video",
    "fields": {
      "prompt": "request.prompt",
      "duration": "request.duration",
      "aspect_ratio": "request.aspectRatio",
      "resolution": "request.resolution",
      "first_frame_image": "request.images.0.url",
      "end_frame_image": "request.images.1.url"
    }
  }
}
```

当前统一请求可用字段：

| 表达式 | 类型 | 说明 |
| --- | --- | --- |
| `request.model` | string | 当前模型 ID。 |
| `request.prompt` | string | 提示词。 |
| `request.images` | array | 图片参考帧或输入图。 |
| `request.videos` | array | 视频参考。 |
| `request.audios` | array | 音频参考。 |
| `request.imageCount` | number | 生成图片数量。 |
| `request.duration` | number | 视频时长，单位秒。 |
| `request.aspectRatio` | string | 画幅比例。 |
| `request.resolution` | string | 分辨率档位。 |
| `request.quality` | string | 质量档位。 |
| `request.generateAudio` | boolean | 是否生成音频。 |
| `request.watermark` | boolean | 是否加水印。 |
| `request.operation` | string | 生成操作或动作类型。 |
| `request.extra` | object | 平台附加字段。 |

媒体数组元素支持：

- `request.images.0.url`
- `request.images.0.dataUrl`
- `request.images.0.kind`
- `request.images.0.ephemeral`

`videos` 和 `audios` 同理。

路径映射也支持数组下标，例如 `request.images.0.url`。如果字段解析为空，宿主会省略该字段，因此可以安全映射可选输入。

### 通用变换

右侧表达式可以加变换：

```text
request.prompt | trim
request.duration | int
request.generateAudio | bool
request.resolution | lower
request.resolution | omit_auto
```

当前常用变换：

- `trim`
- `lower` / `lowercase`
- `upper` / `uppercase`
- `bool` / `boolean`
- `int` / `integer`
- `omit_zero`
- `omit_empty`
- `omit_auto`
- `resolution_p`
- `omit_if_model_contains:X`
- `omit_unless_model_contains:X`
- `omit_if_model_equals:X`
- `omit_unless_model_equals:X`

`resolution_p` 会把纯数字 `720` 变成 `720p`。旧变换 `video-resolution` 保留兼容，新插件不建议使用。

## 7. 响应映射

同步接口只需要 `create` 和 `response`。异步接口需要额外声明 `poll`。

### 同步示例

```json
{
  "response": {
    "resultPaths": ["data.images"],
    "resultKind": "image",
    "resultEphemeral": true
  }
}
```

### 异步示例

```json
{
  "create": { "method": "POST", "path": "/v1/tasks" },
  "poll": { "method": "GET", "path": "/v1/tasks/{{taskId}}" },
  "response": {
    "taskIdPaths": ["data.task_id"],
    "statusPaths": ["data.status"],
    "errorPaths": ["data.error.code"],
    "messagePaths": ["data.error.message"],
    "resultPaths": ["data.output.videos"],
    "resultKind": "video",
    "resultEphemeral": true
  }
}
```

响应字段：

| 字段 | 说明 |
| --- | --- |
| `taskIdPaths` | 从创建或查询响应中读取任务 ID。 |
| `statusPaths` | 读取上游状态。 |
| `errorPaths` | 判断上游是否返回错误。 |
| `messagePaths` | 读取错误或提示消息。 |
| `textPaths` | 读取文本结果。 |
| `reasoningPaths` | 读取思考文本。 |
| `resultUrlPaths` | 读取单个结果 URL。 |
| `resultPaths` | 读取结果数组。 |
| `resultKind` | 结果类型：`image`、`video`、`audio`。 |
| `resultEphemeral` | 结果 URL 是否短期有效。 |

状态会归一化为：

- `pending`
- `processing`
- `succeeded`
- `failed`
- `cancelled`

如果查询响应里已经有结果但状态仍是 `pending`，宿主会判定为成功。这可以兼容部分上游在完成后不再准确改状态的行为。

## 8. Workflows

`workflows` 用于声明某个 Provider 下可被用户选择的模型入口或工作流。

示例：

```json
{
  "contributes": {
    "workflows": [
      {
        "id": "acme-video-lite",
        "label": "Acme Video Lite",
        "providerId": "acme-image",
        "capability": "video",
        "parameters": [
          { "name": "duration", "type": "number", "required": true },
          { "name": "resolution", "type": "string", "values": ["480p", "720p", "1080p"] }
        ],
        "defaults": {
          "duration": 5,
          "resolution": "720p"
        }
      }
    ]
  }
}
```

`providerId` 必须指向同一个插件包中的 Provider ID。当前上传插件的工作流声明主要作为目录和参数描述；真正的 HTTP 执行仍由 Provider 的声明式映射完成。

## 9. CanvasNodes

插件可以贡献画布节点：

```json
{
  "contributes": {
    "canvasNodes": [
      {
        "id": "acme-storyboard",
        "label": "Acme Storyboard",
        "defaultTitle": "Acme 分镜",
        "defaultSize": { "width": 320, "height": 240 },
        "renderer": "declarative",
        "schema": {
          "type": "object",
          "properties": {
            "title": { "type": "string" },
            "content": { "type": "string" }
          }
        }
      }
    ]
  }
}
```

`declarative` 是当前可用的基础渲染方式。宿主会按 schema 展示 `node.metadata.pluginData` 中的字段。适合配置面板、结构化文本、简单状态或轻量数据节点。

`sandbox` 是为隔离 UI 预留的渲染模式。当前版本会显示“插件节点等待隔离运行时”占位，不会执行插件 Web 代码。

## 10. 其他贡献点

| 贡献点 | 当前状态 | 说明 |
| --- | --- | --- |
| `providers` | 可用 | 声明式 HTTP 生成能力。 |
| `workflows` | 可用 | 描述 Provider 下的工作流或模型入口。 |
| `canvasNodes` | 部分可用 | `declarative` 可用；`sandbox` 未启用执行。 |
| `transforms` | 合同已定义 | 需要配合受控运行时，当前不建议依赖其执行业务逻辑。 |
| `commands` | 合同已定义 | 命令贡献的结构存在，但完整 UI 执行链路仍在完善。 |
| `assetSources` | 合同已定义 | 外部素材源能力有内置实现，上传包侧需要宿主继续完善。 |
| `usageObservers` | 合同已定义 | 用量观察者合同存在。 |
| `aiCapabilities` | 合同已定义 | 例如提示词优化器类能力。 |
| `agents` | 合同已定义 | Agent 贡献合同存在。 |
| `importExport` | 合同已定义 | 导入导出贡献合同存在。 |

这些贡献点都写在同一个 `contributes` 对象下，但目前不是每个都已经有完整的上传插件执行链路。开发插件时应以 `providers`、`workflows`、`declarative canvasNodes` 作为主路径。

## 11. 生命周期与 API

管理员可以通过前端“插件中心”上传插件，也可以直接调用接口：

| 操作 | API |
| --- | --- |
| 上传插件 | `POST /api/plugins` |
| 下载插件包 | `GET /api/plugins/:id/package` |
| 平台启用 | `POST /api/plugins/:id/enable` |
| 平台停用 | `POST /api/plugins/:id/disable` |
| 用户启用/停用 | `PUT /api/plugins/:id/activation` |
| 卸载插件 | `DELETE /api/plugins/:id` |
| 查看插件列表 | `GET /api/plugins` |

规则：

- 上传和卸载需要管理员权限。
- 官方内置应用 ID 不能被上传插件覆盖。
- 上传插件属于平台级插件，默认由管理员控制可用性。
- 内置官方应用通常支持用户级启用/停用。
- 清单校验失败时整包不会安装。
- 卸载会同时清理用户插件状态和平台可用状态。

## 12. UI 插件如何出现在页面上

当前上传插件不是自由接管任意页面的组件。前端页面会出现在哪里，由宿主现有入口决定：

- Provider 会进入对应渠道或模型选择目录。
- Workflow 会进入对应能力的工作流选择目录。
- `declarative canvasNodes` 会注册进画布节点注册表，可出现在节点创建菜单和画布中。
- 插件详情文档会显示在插件中心详情弹窗。
- 开发协议文档会显示在“上传插件”弹窗左侧。

如果插件想拥有独立的一级页面、侧边栏入口、导航 Tab 或自定义大屏 UI，目前协议还没有支持通用的 `contributes.pages` 挂载机制。这类需求需要先规划宿主侧的页面挂载点和安全边界，不能只在 Manifest 中随意声明一个页面路径。

内置前端插件的做法不同。它们通过前端注册器直接调用 `registerPlugin()`，并且可以结合宿主内置 UI。这个机制只面向随应用一起发布的可信插件，不适合作为第三方上传插件的通用挂载方式。

## 13. Web Runtime 现状

上传插件可以声明：

```json
{
  "entry": "web/entry.js",
  "surfaces": ["fullscreen"],
  "runtime": { "web": "sandbox" }
}
```

但当前版本的宿主没有完整执行上传 Web 代码。上传包会被校验和保存，`entry` 合同也已经存在；真正的隔离沙箱渲染和消息协议还没有上线。因此，第三方插件现阶段不要依赖 Web 代码执行，也不要假设自定义 JS 能访问主页面 DOM、登录态或本地存储。

## 14. 安全边界

- API Key 只能保存在渠道或插件配置中，不能写入 Manifest。
- 外部请求必须通过后端 Provider runtime 发出。
- 宿主统一处理鉴权、超时、SSRF 防护、轮询、下载和错误。
- 上传插件不能声明 `host:` 执行器。
- 插件不能直接执行数据库操作、访问本地文件或绕过平台计费。
- 短期结果 URL 必须标记 `resultEphemeral: true`，让宿主立即下载转存。
- 插件申请的权限必须是最小权限。

## 15. 开发建议

1. 先确认目标服务只需要 HTTP 请求映射。如果是，优先做 `providers`。
2. 使用异步任务时，明确 `create` 返回任务 ID，`poll` 按 `{{taskId}}` 查询。
3. 上游如果返回短期 URL，一定要设置 `resultEphemeral: true`。
4. 尽量使用官方统一字段，不要让宿主增加插件专属逻辑。
5. 枚举值尽量写全，让画布 UI 可以直接展示。
6. 参考素材为空时依赖宿主的“空值省略”行为。
7. 不要把插件做成一个随意跳转或隐藏执行的外部脚本。
8. 本地打包后先在后端日志中确认清单校验、包校验和渠道可用性。

## 16. 相关代码位置

| 内容 | 路径 |
| --- | --- |
| Manifest 类型 | `backend/internal/protocol/types.go` |
| Manifest 校验与映射 | `backend/internal/protocol/manifest.go` |
| ZIP 包校验 | `backend/internal/protocol/package.go` |
| 插件安装与卸载 | `backend/internal/service/protocol_registry.go` |
| 插件启用状态 | `backend/internal/service/plugin_management.go` |
| 插件 API | `backend/internal/handler/plugin.go` |
| 前端插件类型 | `web/src/lib/plugins/plugin-types.ts` |
| 前端插件注册器 | `web/src/lib/plugins/plugin-registry.ts` |
| 插件画布节点注册 | `web/src/lib/canvas/node-registry/node-registry.ts` |
| 本协议文档 | `web/src/pages/plugins/plugin-development-guide.md` |
