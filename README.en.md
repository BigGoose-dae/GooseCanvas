# Goose Canvas

[简体中文](README.md) | [English](README.en.md)

Goose Canvas is an open-source, local-first canvas for AI text, image, audio, and video creation. It lets you organize text, image, audio, and video nodes on an infinite canvas, connect assets and prompts into workflows, and generate content with Volcengine models.

## Features

- Create and manage multiple creative projects.
- Add, move, resize, rename, copy, and delete nodes on an infinite canvas.
- Organize prompts and assets with text, image, audio, and video nodes.
- Generate, rewrite, or expand text with Doubao Seed 2.1 Pro, generate images from text or images, generate audio from reference clips, and generate videos from text or images.
- Upload local images, audio, and videos and connect them to generation nodes. Audio nodes accept up to three reference clips.
- Autosave node content, positions, and parameters, with browser draft recovery and retry after a failed save.
- Run multiple generation tasks concurrently in the backend, with each task isolated from the others.
- Stream queued, submitting, generating, archiving, completed, and failed states through SSE.
- Browse the task queue and paginated generation history, preview results, and download files.
- Configure model connections, task concurrency, and polling timeouts in the Web interface.
- Manage the model registry and add, edit, enable, disable, or delete custom models.
- Switch between Chinese and English interfaces and between dark and light themes.

Goose Canvas does not provide task cancellation. After a task has been submitted to a model provider, cancelling it locally cannot guarantee that the provider task will stop.

## Technology

- Go and Gin
- React, Vite, and XYFlow
- SQLite
- Server-Sent Events (SSE)
- Volcengine APIs

SQLite stores projects, nodes, edges, asset metadata, model definitions, and generation tasks. Uploaded assets and generated results are stored in local directories.

## Requirements

- Go 1.23+
- Node.js 18+
- A valid Volcengine Ark API Key; a dedicated audio credential can also be configured

## Quick start

Development mode runs the Go API and Web development server separately.

Terminal one:

```bash
make dev-api
```

Terminal two:

```bash
cd web
npm install
npm run dev
```

Open `http://localhost:5173`. Vite proxies `/api` requests to `http://localhost:8080`.

On first launch, open **Settings**, enter an Ark API Key, and save it.

## Build and run

```bash
make build
./bin/goose-canvas
```

Open `http://localhost:8080`. The Go service serves both the API and the built Web application.

## Usage

1. Create a project on the home page.
2. Open the canvas and add text, image, audio, or video nodes, or upload an asset directly.
3. Enter a writing instruction directly in a text node, or connect upstream text nodes to add context. Asset and text nodes can also feed image, audio, and video nodes.
4. Select a node and configure its model, prompt or text instruction, aspect ratio, resolution, duration, and other parameters.
5. Select **Start generation**. The task enters the backend queue and its status updates through SSE.
6. When generation completes, view text results in the node or generation history, and preview or download image, audio, and video results.

The canvas supports these common controls:

- Right-click empty space to add a node or upload an asset.
- Right-drag to pan the canvas.
- Left-drag to select nodes.
- Hold Space and left-drag to pan.
- Drag from a node's right handle to empty space to create a downstream node.
- Right-click a node to rename, copy, view history, edit parameters, or delete it.

Copying a node keeps its content and current asset, but does not copy edges or submit a generation task automatically.

## Model management

Goose Canvas includes these Volcengine model definitions:

| Type | Display name | Model key | Protocol | Defaults |
| --- | --- | --- | --- | --- |
| Text | Doubao Seed 2.1 Pro | `doubao-seed-2-1-pro-260628` | `ark-chat-v3` | `temperature=0.7`, 4096 tokens, thinking disabled |
| Audio | Doubao Seed Audio 1.0 Reference | `doubao-seed-audio-1.0-reference` | `doubao-audio-v3` | `mp3`, 24 kHz, standard speech/loudness/pitch |
| Image | Seedream 5.0 Lite | `doubao-seedream-5-0-lite-260128` | `ark-image-v3` | `1:1`, `2K` |
| Video | Seedance 2.0 | `doubao-seedance-2-0-260128` | `ark-video-v3` | `16:9`, `720p`, 5 seconds |

The model management page configures a model key, task type, provider, protocol, input types, default parameters, and available options. Goose Canvas currently supports the `volcengine` provider with the `ark-chat-v3`, `ark-image-v3`, `doubao-audio-v3`, and `ark-video-v3` protocols.

Model availability depends on the models enabled for the corresponding Volcengine account.

## Settings

Use the `/settings` page to configure:

- Application name
- Ark API Key
- Ark service URL
- Doubao audio API Key, or an audio App ID and Access Key
- Doubao audio service URL
- Maximum concurrent requests (1–32)
- Task polling timeout (1–1440 minutes)

Settings are stored in SQLite, with credentials encrypted using AES-256-GCM. The encryption key is stored at `DATA_DIR/.settings.key`, and the API never returns the original credential value to the page. Leaving the password field blank preserves the saved credential.

Environment variables can also provide initial configuration:

| Variable | Default | Description |
| --- | --- | --- |
| `APP_NAME` | `Goose Canvas` | Application name |
| `APP_LOGO_URL` | Empty | Custom logo URL; uses the built-in theme logo when empty |
| `APP_REPOSITORY_URL` | Project GitHub URL | Repository link displayed on the home page |
| `APP_ADDR` | `127.0.0.1:8080` | Go service listen address |
| `DATA_DIR` | `./data` | Directory for SQLite, the settings key, and local assets |
| `VOLCENGINE_API_KEY` | Empty | Volcengine Ark API Key |
| `VOLCENGINE_BASE_URL` | Ark Beijing endpoint | Ark API base URL |
| `VOLCENGINE_AUDIO_API_KEY` | Empty | Dedicated Doubao audio API Key |
| `VOLCENGINE_AUDIO_APP_ID` | Empty | Doubao audio App ID; configure it together with the Access Key |
| `VOLCENGINE_AUDIO_ACCESS_KEY` | Empty | Doubao audio Access Key; configure it together with the App ID |
| `VOLCENGINE_AUDIO_ENDPOINT` | `https://openspeech.bytedance.com/api/v3/tts/create` | Doubao audio generation endpoint |
| `WORKER_POLL_INTERVAL` | `3s` | Status polling interval for asynchronous video tasks |
| `WORKER_TASK_TIMEOUT` | `30m` | Polling timeout for asynchronous tasks |

After Web settings have been saved, database configuration takes precedence. `APP_ADDR` and `DATA_DIR` always come from the startup environment. Restart the service after changing environment variables.

## Files and task processing

The Go service validates uploaded file types and stores them under `DATA_DIR/assets`. Before calling a model, the backend reads input assets and creates `data:*;base64,...` values. Generation task records do not store Base64 file bodies.

Text results are written back to the node and retained in generation history. When an image or audio API returns Base64 content, the service decodes and saves it locally. When an image, audio, or video API returns a temporary file URL, the service downloads and archives it immediately, so generated results do not depend on provider URLs remaining available.

The task queue runs up to four requests concurrently by default. Video tasks do not occupy an execution slot while waiting for their next status poll. After a service restart, tasks that can still be queried or archived resume from SQLite.

A timed-out task can continue querying the original provider task, and a failed archive can retry archiving. Neither action submits a new generation request. Submitting again after an ordinary failure creates a new task and may incur another charge.

## Architecture

```text
React Canvas ── HTTP (save / submit / history / download) ── Go API ── SQLite
      ▲                                                       │
      └──────── SSE (status / result snapshots) ───────────────┤
                                                              ├─ Concurrent tasks ── Volcengine (Base64 input)
                                                              └─ Local archive ── DATA_DIR/assets
```

## Security and limitations

- Goose Canvas is designed for local, single-user use and does not include login, team permissions, or quotas.
- The default listener is `127.0.0.1:8080`. Configure HTTPS, authentication, and access control before listening on an external interface.
- Each uploaded file is limited to 50 MB, and input assets for one generation are limited to 50 MB in total.
- Each generated result is limited to 2 GB.
- Never commit or publicly share the database, settings key, environment files, or asset directory.
- The SQLite task queue is intended for a single-machine deployment. Multiple instances require additional database claiming and a distributed queue.

See [SECURITY.md](SECURITY.md) for more security guidance.

## Development verification

```bash
make test
go test -race ./...
go vet ./...
```

With [Gitleaks](https://github.com/gitleaks/gitleaks) installed, scan the repository for exposed credentials:

```bash
make secrets
```

GitHub Actions runs Go tests, Web tests, the Web production build, and a complete Git history secret scan.

## License

[MIT](LICENSE)
