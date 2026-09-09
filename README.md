# Goose Canvas

一个独立、轻量、可本地运行的 AI 创作画布。项目只保留 Go 与 Web 两端，不依赖 Java 调度器；Go 服务内置任务队列和轮询 Worker。

当前版本默认使用：

- SQLite 保存项目、节点、连线、素材元数据和生成任务；无需安装数据库。
- 火山引擎方舟调用 Seedream 图片能力和 Seedance 视频能力。
- 火山 TOS 保存上传素材与最终生成结果，并给模型生成短期签名 URL。
- React + Vite + XYFlow 提供项目列表和无限画布。

## 已支持

- 创建、删除项目。
- 创建文本、图片、视频节点，拖动节点并连接上下游。
- 上传图片和视频素材到 TOS。
- 文生图、图生图、文生视频、图生视频。
- 文本节点作为下游生成节点的附加提示词。
- Go 内置异步视频提交、状态轮询、失败记录与结果归档。
- 私有 TOS Bucket；浏览器和模型通过短期签名 URL 访问素材。
- `APP_NAME`、`APP_LOGO_URL` 和 `APP_REPOSITORY_URL` 品牌入口预留。
- 与 Workshop 一致的基础画布操作：空白处右键添加、右键拖拽平移、左键框选、空格加左键平移、节点拖拽缩放，以及从节点右侧拖线到空白处快速创建下游节点。
- SQLite 模型注册表与可视化模型管理入口；支持添加兼容火山 Ark 图片/视频协议的新模型。

## 运行要求

- Go 1.23+
- Node.js 18+
- 一个可用的火山方舟 API Key
- 一个 TOS Bucket 及其访问凭证

TOS 和方舟是两套独立凭证。Bucket 需要能被运行 Goose Canvas 的机器访问，同时签名 URL 必须能被火山模型服务从公网读取。建议使用私有 Bucket，并给凭证授予该 Bucket 的读取、写入、删除和 Bucket 检查权限。

## 本地启动

复制配置文件：

```bash
cp .env.example .env
```

至少填写：

```dotenv
VOLCENGINE_API_KEY=your-ark-api-key
TOS_BUCKET=your-bucket
TOS_ACCESS_KEY=your-tos-access-key
TOS_SECRET_KEY=your-tos-secret-key
```

开发模式使用两个终端：

```bash
make dev-api
```

```bash
cd web
npm install
npm run dev
```

访问 `http://localhost:5173`。Vite 会把 `/api` 请求代理到 `http://localhost:8080`。

也可以先构建 Web，再由 Go 服务同时提供 API 和静态页面：

```bash
make build
./bin/goose-canvas
```

访问 `http://localhost:8080`。

## 使用方式

1. 在首页创建项目。
2. 进入画布，添加文本、图片或视频节点，也可以直接上传素材。
3. 从素材/文本节点右侧连接点拖到生成节点左侧连接点。
4. 在生成节点填写提示词并选择比例、清晰度、时长等参数。
5. 点击“开始生成”。图片结果同步返回；视频任务由 Go Worker 自动轮询。
6. 生成结果会先由 Go 下载，再写回你的 TOS，画布不长期依赖模型厂商的临时结果 URL。

画布中的图片和视频节点只展示内容框；选中节点后，在右侧参数面板编辑模型、提示词和生成参数。右键点击节点可重命名、打开参数或删除。

## 模型管理

内置模型由 [`internal/database/sql/001_seed_models.sql`](internal/database/sql/001_seed_models.sql) 初始化到 SQLite，不再通过 `.env` 配置模型 Key。首页和画布顶部均可进入“模型管理”，添加、编辑、启停或删除自定义模型。

模型记录包含模型 Key、任务类型、供应商、调用协议、允许的输入、默认参数和可选参数。当前基础版内置 `volcengine` 供应商适配器，用户可以直接添加任何兼容 `ark-image-v3` 或 `ark-video-v3` 协议且已在其方舟账号开通的模型。接入其他供应商仍需实现对应 Go 适配器。

## 配置

| 配置项 | 默认值 | 说明 |
| --- | --- | --- |
| `APP_NAME` | `Goose Canvas` | 平台名称 |
| `APP_LOGO_URL` | 空 | Logo URL；为空时显示内置占位标识 |
| `APP_REPOSITORY_URL` | 项目 GitHub 地址 | 首页仓库链接 |
| `APP_ADDR` | `:8080` | Go 监听地址 |
| `DATA_DIR` | `./data` | SQLite 和临时文件目录 |
| `VOLCENGINE_API_KEY` | 空 | 火山方舟 API Key |
| `VOLCENGINE_BASE_URL` | 方舟北京地址 | 方舟 API 根地址 |
| `TOS_ENDPOINT` | 北京区域 | TOS Endpoint |
| `TOS_REGION` | `cn-beijing` | TOS Region |
| `TOS_BUCKET` | 空 | Bucket 名称 |
| `TOS_ACCESS_KEY` | 空 | TOS Access Key |
| `TOS_SECRET_KEY` | 空 | TOS Secret Key |
| `TOS_PREFIX` | `goose-canvas` | 对象 Key 前缀 |
| `TOS_PRESIGN_TTL` | `24h` | 给模型读取输入素材的签名有效期，最大 7 天 |

修改 `.env` 后需要重启 Go 服务。密钥仅由 Go 服务读取，不会下发到 Web 端。不要提交 `.env`。

## 架构

```text
Browser / React Canvas
          |
          v
       Go API  ------ SQLite
          |
          +---------- TOS (input/output assets)
          |
          +---------- Volcengine Ark
          |
          +---------- embedded generation worker
```

上传先经过 Go 服务写入 TOS，不需要为浏览器配置 TOS 上传权限或暴露 Access Key。图生图和图生视频所需 URL 由后端按任务即时签名，因此对象存储是当前基础版本的必选依赖。

## 当前边界

- 当前为本地单用户版本，没有登录、团队权限和配额系统，请勿直接暴露到公网。
- 当前仅实现原生 TOS；COS、OSS、S3 兼容层留待后续抽象。
- 当前不提供 Docker 一键启动。
- 任务队列使用 SQLite，适合单机部署；多实例部署需增加数据库级抢占和分布式队列。
- 模型 ID 与参数可能因账号开通范围变化，请以火山方舟控制台实际可用模型为准。

## 开发验证

```bash
make test
```

许可证：MIT。
