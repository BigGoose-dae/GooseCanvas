# Goose Canvas

[简体中文](README.md) | [English](README.en.md)

Goose Canvas 是一个开源、本地优先的 AI 文本、图片、音频与视频创作画布。你可以在无限画布中组织文本、图片、音频和视频节点，将素材与提示词连接成工作流，并通过火山引擎模型完成内容生成。

## 核心功能

- 创建和管理多个创作项目。
- 在无限画布中添加、拖动、缩放、重命名、复制和删除节点。
- 使用文本、图片、音频和视频节点组织提示词与素材；文本节点正文支持 Markdown 自动渲染。
- 使用 Doubao Seed 2.1 Pro 生成、改写或扩展文本，并支持文生图、图生图、参考音频生成、文生视频和图生视频。
- 上传本地图片、音频和视频，并通过节点连线作为生成输入。音频节点最多可接入 3 段参考音频。
- 自动保存节点内容、位置和参数，保存失败时保留浏览器草稿并支持重试。
- 在后端并发执行多个生成任务，任务之间相互独立。
- 通过 SSE 实时同步排队、提交、生成、归档、完成和失败状态。
- 查看任务队列、分页生成历史、结果预览及下载入口。
- 在 Web 页面配置模型连接、任务并发数和查询超时。
- 管理模型注册表，添加、编辑、启停或删除自定义模型。
- 支持中文和英文界面、深色和浅色主题。

Goose Canvas 不提供任务取消功能。生成任务提交到模型服务后，本地取消无法保证第三方任务同步终止。

## 技术栈

- Go、Gin
- React、Vite、XYFlow
- SQLite
- Server-Sent Events（SSE）
- 火山引擎 API

SQLite 保存项目、节点、连线、素材元数据、模型定义和生成任务。上传素材与生成结果保存在本地文件夹中。

## 运行要求

- Go 1.23+
- Node.js 18+
- 一个可用的火山引擎方舟 API Key；使用语音专用凭证时也可另行配置

## 快速开始

开发模式需要分别启动 Go API 和 Web 开发服务器。

终端一：

```bash
make dev-api
```

终端二：

```bash
cd web
npm install
npm run dev
```

打开 `http://localhost:5173`。Vite 会将 `/api` 请求代理到 `http://localhost:8080`。

首次打开后进入「设置」，填写方舟 API Key 并保存。

## 构建运行

```bash
make build
./bin/goose-canvas
```

打开 `http://localhost:8080`。Go 服务会同时提供 API 和已构建的 Web 页面。

## 使用方法

1. 在首页创建一个项目。
2. 进入画布，添加文本、图片、音频或视频节点，也可以直接上传素材。
3. 文本节点正文与生成提示词分别保存：在节点中编辑正文，在右侧参数面板填写提示词；也可以从上游文本节点连线补充上下文。素材或文本节点也可以连接到图片、音频、视频节点。
4. 选中节点，设置模型、提示词或文本指令，以及画面比例、清晰度和时长等参数。
5. 点击「开始生成」。任务会进入后端队列，并通过 SSE 实时更新状态。
6. 生成完成后，可以在节点或生成历史中查看文本结果，并预览或下载图片、音频、视频结果。

画布支持以下常用操作：

- 在空白处右键添加节点或上传素材。
- 右键拖动画布进行平移。
- 左键拖动进行框选。
- 按住空格并使用左键平移画布。
- 从节点右侧连接点拖到空白处，快速创建下游节点。
- 右键点击节点进行重命名、复制、查看历史、编辑参数或删除。

复制节点会保留节点内容和当前素材，但不会复制连线或自动提交生成任务。

## 模型管理

Goose Canvas 内置以下火山引擎模型定义：

| 类型 | 显示名称 | 模型 Key | 协议 | 默认参数 |
| --- | --- | --- | --- | --- |
| 文本 | Doubao Seed 2.1 Pro | `doubao-seed-2-1-pro-260628` | `ark-chat-v3` | `temperature=0.7`、4096 Token、关闭思考 |
| 音频 | Doubao Seed Audio 1.0 Reference | `doubao-seed-audio-1.0-reference` | `doubao-audio-v3` | `mp3`、24 kHz、标准语速/音量/音调 |
| 图片 | Seedream 5.0 Lite | `doubao-seedream-5-0-lite-260128` | `ark-image-v3` | `1:1`、`2K` |
| 视频 | Seedance 2.0 | `doubao-seedance-2-0-260128` | `ark-video-v3` | `16:9`、`720p`、5 秒 |

模型管理页面支持配置模型 Key、任务类型、供应商、调用协议、输入类型、默认参数和可选参数。目前支持 `volcengine` 供应商，以及 `ark-chat-v3`、`ark-image-v3`、`doubao-audio-v3` 和 `ark-video-v3` 协议。

模型是否可调用取决于对应火山引擎账号的开通范围。

## 设置

推荐在 `/settings` 页面完成配置。页面支持修改：

- 应用名称
- 方舟 API Key
- 方舟服务地址
- 豆包语音 API Key，或语音 App ID 与 Access Key
- 豆包语音服务地址
- 最大并发请求数（1–32）
- 任务查询超时（1–1440 分钟）

设置保存在 SQLite 中，凭证使用 AES-256-GCM 加密。加密密钥位于 `DATA_DIR/.settings.key`，API 不会向页面回传凭证原值。密码框留空时会保留已经保存的凭证。

也可以通过环境变量提供首次启动配置：

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_NAME` | `Goose Canvas` | 应用名称 |
| `APP_LOGO_URL` | 空 | 自定义 Logo URL；为空时使用内置主题 Logo |
| `APP_REPOSITORY_URL` | 项目 GitHub 地址 | 首页显示的仓库链接 |
| `APP_ADDR` | `127.0.0.1:8080` | Go 服务监听地址 |
| `DATA_DIR` | `./data` | SQLite、配置密钥和本地素材目录 |
| `VOLCENGINE_API_KEY` | 空 | 火山引擎方舟 API Key |
| `VOLCENGINE_BASE_URL` | 方舟北京地址 | 方舟 API 根地址 |
| `VOLCENGINE_AUDIO_API_KEY` | 空 | 豆包语音专用 API Key |
| `VOLCENGINE_AUDIO_APP_ID` | 空 | 豆包语音 App ID；需与 Access Key 同时配置 |
| `VOLCENGINE_AUDIO_ACCESS_KEY` | 空 | 豆包语音 Access Key；需与 App ID 同时配置 |
| `VOLCENGINE_AUDIO_ENDPOINT` | `https://openspeech.bytedance.com/api/v3/tts/create` | 豆包语音生成地址 |
| `WORKER_POLL_INTERVAL` | `3s` | 异步视频任务状态查询间隔 |
| `WORKER_TASK_TIMEOUT` | `30m` | 异步任务查询超时 |

Web 设置保存后，以数据库配置为准；`APP_ADDR` 和 `DATA_DIR` 始终由启动环境决定。环境变量变更需要重启服务。

## 文件与任务处理

上传文件由 Go 服务校验类型后保存到 `DATA_DIR/assets`。调用模型时，后端读取输入素材并生成 `data:*;base64,...` 数据，生成任务表中不会保存 Base64 文件正文。

文本结果会写回节点并保存到生成历史。图片或音频 API 返回 Base64 内容时，服务会解码并保存到本地。图片、音频或视频 API 返回临时文件链接时，服务会立即下载并归档到本地目录，因此生成结果不依赖第三方临时链接长期有效。

任务队列默认最多并发执行 4 个请求。视频任务等待下一次状态查询时不会占用执行名额。服务重启后，可继续查询或归档的任务会从 SQLite 中恢复。

查询超时可以继续查询原任务，归档失败可以重新归档；这两个操作都不会重新提交生成请求。普通失败后重新提交会创建新的生成任务，并可能产生新的费用。

## 架构

```text
React Canvas ── HTTP（保存 / 提交 / 历史 / 下载）── Go API ── SQLite
      ▲                                            │
      └──────── SSE（状态 / 结果快照）───────────────┤
                                                   ├─ 并发任务执行 ── 火山引擎（Base64 输入）
                                                   └─ 本地素材归档 ── DATA_DIR/assets
```

## 安全与使用边界

- Goose Canvas 面向本地单用户使用，没有内置登录、团队权限和配额系统。
- 默认只监听 `127.0.0.1:8080`。监听外部网卡时，请自行配置 HTTPS、认证和访问控制。
- 单个上传文件最大 50MB，一次生成的输入素材合计最大 50MB。
- 单个生成结果最大 2GB。
- 数据库、配置密钥、环境文件和素材目录都不应提交到 Git 或公开分享。
- SQLite 任务队列适合单机部署；多实例部署需要额外的数据库抢占机制和分布式队列。

更多安全说明见 [SECURITY.md](SECURITY.md)。

## 开发验证

```bash
make test
go test -race ./...
go vet ./...
```

安装 [Gitleaks](https://github.com/gitleaks/gitleaks) 后，可以运行以下命令检查仓库中的凭证泄漏：

```bash
make secrets
```

GitHub Actions 会执行 Go 测试、前端测试、前端构建和提交历史密钥扫描。

## License

[MIT](LICENSE)
