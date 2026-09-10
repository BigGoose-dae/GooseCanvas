# Security

Goose Canvas currently targets a single user on a local machine. It has no login or team authorization. The default listener is `127.0.0.1:8080`; do not expose it directly to the public Internet. An authenticated reverse proxy is required before any broader deployment.

## Credentials and backups

- Configure the Ark credential through the settings page. Read APIs return only whether it is configured, never its original value.
- Web settings are encrypted with AES-256-GCM in SQLite. The encryption key is stored separately in `data/.settings.key`, with owner-only permissions. The database, its WAL/SHM sidecars and `.env` also use owner-only permissions.
- Retain both the database and encryption key in a private backup. Neither belongs in Git, release archives, screenshots, Issues or support logs. A full copy of the data directory contains both ciphertext and its key and must be treated as sensitive.
- Legacy `.env` configuration remains supported and is plaintext. Existing plaintext database settings migrate when the updated server starts. Previously created backups are not retroactively encrypted.
- SQL logging omits parameter values. Provider errors redact the configured API key. Do not enable SDK request/body debug logging when using real credentials.
- Use HTTPS for the remote model endpoint. Loopback HTTP remains available for local development. Local mode rejects non-local Host headers and cross-site writes.
- Uploaded media and generated results are stored under `DATA_DIR/assets` with owner-only directory and file permissions. Treat a backup of this directory as private user content.

## Before publishing

Run `make secrets`. It rejects tracked environment files, key files and database backups, scans all reachable Git history with Gitleaks, and scans staged/unstaged changes and current non-ignored files. GitHub Actions repeats the history scan for pushes and pull requests. Scanner output is redacted.

No automated scan proves the absence of every secret. Review newly added files before staging, and enable GitHub secret scanning/push protection where available in the repository settings. Do not bypass a finding merely because the credential has been removed from the latest version: history may still contain it.

If a credential was published, revoke/rotate it at the provider first, inspect usage, then remove it from history and any release assets or logs. Deleting the working-tree file or adding `.gitignore` alone does not revoke a published credential.

## Reporting

Do not include live tokens, database files or encryption keys in public reports. Contact the maintainer privately before publishing details that could expose users' credentials or data.
