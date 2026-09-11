# Goose Canvas

[简体中文](README.md) | [English](README.en.md)

A standalone, lightweight AI creation canvas that runs locally. The project contains only Go and Web applications and does not rely on a Java scheduler. The Go service includes a concurrent task queue and streams task status to the Web app through SSE.

The current version uses:

- SQLite for projects, nodes, edges, asset metadata, and generation tasks, with no separate database installation.
- Volcengine Ark with Seedream image models and Seedance video models.
- Local directories for uploaded assets and final results. The Go service encodes input files as Base64 when calling a model.
- React, Vite, and XYFlow for project management and an infinite canvas.
- A Chinese and English Web interface. The language preference is stored in the current browser.

## Features

- Create and delete projects.
- Create text, image, and video nodes, move them, and connect upstream and downstream nodes.
- Upload image and video assets to a local directory.
- Generate images from text or images and videos from text or images.
- Use text nodes as additional prompts for downstream generation nodes.
- Run image and video tasks asynchronously, with up to four concurrent requests by default and an adjustable limit in Web settings.
- Sync task status through SSE with automatic reconnection without overwriting node content being edited.
- Autosave nodes with visible save status. Failed drafts remain in the browser and can be retried.
- Copy nodes, browse paginated generation history, preview results, and use explicit download actions.
- View a project task queue with queued, submitting, generating, archiving, completed, and failed states.
- Configure model credentials and concurrency in the Web interface, with changes applied immediately.
- Store uploads and generated results in the Go service without depending on TOS.
- Customize branding with `APP_NAME`, `APP_LOGO_URL`, and `APP_REPOSITORY_URL`.
- Use canvas controls consistent with Workshop: right-click empty space to add, right-drag to pan, left-drag to select, Space plus left-drag to pan, resize nodes, and drag from a node's right handle to empty space to create a downstream node.
- Manage a SQLite-backed model registry in the Web interface and add models compatible with the Volcengine Ark image and video protocols.

## Requirements

- Go 1.23+
- Node.js 18+
- A valid Volcengine Ark API Key

## Run locally

You do not need to edit a configuration file first. Start the application, open **Settings** on the home page, enter an Ark API Key, and save it. Assets are stored in `DATA_DIR/assets` by default.

For development, use two terminals:

```bash
make dev-api
```

```bash
cd web
npm install
npm run dev
```

Open `http://localhost:5173`. Vite proxies `/api` requests to `http://localhost:8080`.

You can also build the Web application and let the Go service serve both the API and static files:

```bash
make build
./bin/goose-canvas
```

Open `http://localhost:8080`.

## Usage

1. Create a project on the home page.
2. Open the canvas and add text, image, or video nodes, or upload an asset directly.
3. Drag from the right handle of an asset or text node to the left handle of a generation node.
4. Enter a prompt and select parameters such as aspect ratio, resolution, and duration.
5. Select **Start generation**. The API returns a task immediately and runs image and video generation asynchronously in the backend. SSE updates its status, which is also visible in **Task queue**.
6. The Go service downloads generated results into the local asset directory immediately, so the canvas does not depend on temporary provider URLs.

Image and video nodes display only their content on the canvas. Select a node to edit its model, prompt, and generation parameters in the inspector. Right-click a node to rename it, copy it, view generation history, edit parameters, or delete it. A copy keeps the node content and current asset, but does not copy edges or submit a task automatically.

## Model management

Built-in models are initialized in SQLite by [`internal/database/sql/001_seed_models.sql`](internal/database/sql/001_seed_models.sql). Model keys are no longer configured in `.env`. Open **Models** from the home page or canvas to add, edit, enable, disable, or delete custom models.

A model record contains its model key, task type, provider, protocol, permitted inputs, default parameters, and available options. The current base version includes the `volcengine` provider adapter. You can add any model enabled for your Ark account that supports the `ark-image-v3` or `ark-video-v3` protocol. Other providers require a corresponding Go adapter.

## Configuration

Prefer the `/settings` page. Web settings are saved in SQLite and encrypted with AES-GCM. The encryption key is stored separately at `data/.settings.key` with `0600` permissions. Settings APIs report only whether a credential is configured and never return its value. Leaving a password field blank preserves the existing credential.

Plaintext settings from older database versions are encrypted during startup migration. Historical backups are not changed and must be protected separately. Back up both the database and `.settings.key`; credentials cannot be decrypted after restoration without the key. Never upload these files to GitHub, Issues, or public attachments. Encryption protects a database file leaked by itself, but cannot protect a fully compromised machine.

When upgrading from the TOS version, old asset metadata remains in the database, but object files are not migrated automatically. Download any TOS files you need before upgrading. Old assets must be uploaded again before they can be used for generation.

The environment variables below remain available for initial setup and development compatibility. After settings have been saved in the Web interface, database values take precedence. `APP_ADDR` and `DATA_DIR` always come from the startup environment.

| Variable | Default | Description |
| --- | --- | --- |
| `APP_NAME` | `Goose Canvas` | Application name |
| `APP_LOGO_URL` | Empty | Custom logo URL; the built-in logo follows the light or dark theme when empty |
| `APP_REPOSITORY_URL` | Project GitHub URL | Repository link on the home page |
| `APP_ADDR` | `127.0.0.1:8080` | Go listen address |
| `DATA_DIR` | `./data` | Directory for SQLite, the settings key, and local assets |
| `VOLCENGINE_API_KEY` | Empty | Volcengine Ark API Key |
| `VOLCENGINE_BASE_URL` | Ark Beijing endpoint | Ark API base URL |
| `WORKER_POLL_INTERVAL` | `3s` | Polling interval for asynchronous video tasks |
| `WORKER_TASK_TIMEOUT` | `30m` | Local polling timeout for asynchronous tasks |

Web settings apply without a restart; environment variable changes require one. Do not place real credentials in `.env.example`. SQL logs do not interpolate parameter values, and local environment and database files are restricted to their owner.

## Architecture

```text
React Canvas ── HTTP (save / submit / history / download) ── Go API ── SQLite
      ▲                                                       │
      └──────── SSE (status / result snapshots) ───────────────┤
                                                              ├─ Concurrent tasks ── Ark (Base64 input)
                                                              └─ Local archive ── DATA_DIR/assets
```

Uploads first pass through the Go service and are written to a local directory. Before image-to-image or image-to-video submission, the backend reads the assets and generates `data:*;base64,...` inputs. Task records do not store Base64 input bodies. When a model returns a temporary URL, the Go service downloads it immediately and writes it atomically into the local directory.

## Tasks and recovery

- The queue runs up to four operations concurrently by default, with an independent timeout for each operation. Video tasks do not occupy an execution slot while waiting for the next poll, so a slow request does not block other available slots.
- An SSE connection sends the latest snapshot on initial connection and reconnection, then streams task changes and heartbeat events. Reverse proxies must disable SSE buffering and permit long-lived connections.
- Autosave serializes writes per node and saves 600 ms after editing stops by default. Unsaved drafts remain in the current browser. Normal navigation waits for pending saves.
- A timed-out query can **continue querying the same task**, while a failed result archive can **retry archiving**. Neither action resubmits generation.
- An uncertain submission is never retried automatically to avoid duplicate billing. **Submit a new generation** creates a new generation record after an ordinary failure.
- Cancellation is intentionally unavailable. Closing the page or stopping local polling does not cancel a provider task, and deleting a node does not cancel a submitted generation.
- Running tasks retain the connection configuration they started with, while new tasks use the latest settings. After a service restart, queryable and archivable tasks resume from SQLite.

## Current limitations

- This is a local single-user application without authentication, team permissions, or quotas. Do not expose it directly to the public internet.
- Each uploaded file is limited to 50 MB, and input assets for one generation are limited to 50 MB in total to control Base64 request memory usage.
- Each generated model result is limited to 2 GB. Temporary URL downloads are written atomically into the local asset directory.
- Assets stored in TOS by an older release are not migrated automatically and must be uploaded again before reuse.
- A one-command Docker setup is not currently included.
- The SQLite task queue is designed for a single-machine deployment. Multiple instances require database-level claiming and a distributed queue.
- Model IDs and parameters depend on the models enabled for your account. Use the Volcengine Ark console as the source of truth.

## Development verification

```bash
make test
go test -race ./...
go vet ./...
```

Run `make secrets` to scan credentials with Gitleaks installed. GitHub Actions runs tests, builds the application, and scans the complete commit history for secrets. See [SECURITY.md](SECURITY.md) for security guidance.

License: MIT.
