# voice-keyboard-backend

## Archived

This project is archived and no longer maintained.

It is the Go backend for the [voice-keyboard](https://github.com/tikhomirovv/voice-keyboard) desktop client. The pair started as an experiment: build a real-time voice-to-text stack and learn unfamiliar territory with help from coding agents.

Since then, several strong voice-typing tools have appeared, including open-source ones, so continuing this project no longer makes sense for me.

I'm keeping the repository public as a record of that learning experience.

**Related:** [voice-keyboard](https://github.com/tikhomirovv/voice-keyboard) (desktop client)

---

HTTP + WebSocket API that the desktop app talks to for **auth**, **live transcription**, **optional LLM post-processing**, and **desktop auto-updates**.

## What it provides

- **Yandex OAuth** login for the desktop app (browser + callback / deep link back to the client)
- **JWT** session (access / refresh) for API and WebSocket
- **WebSocket transcription** (`/transcribe`) streamed to **OpenAI Realtime**
- **Audio**-related HTTP helpers
- **Updater** endpoint for Tauri (`/updater/...`) and static **release** file serving
- HTML views for a minimal auth/UI shell (Fiber templates)
- Postgres persistence (users, tokens, social auth) via GORM + migrations

## Stack

| Piece | Tech |
| --- | --- |
| Language | Go 1.24 |
| HTTP | Fiber v2 |
| DI | Google Wire |
| DB | PostgreSQL, GORM, gormigrate |
| Auth | JWT, Yandex OAuth |
| Realtime | gorilla/websocket → OpenAI Realtime |
| Config | Viper + `config.yml` |
| CLI | urfave/cli (`app:start`, DB migrate/rollback) |
| Deploy sample | Docker Compose + GitLab CI templates |

## Companion client

Desktop app: [voice-keyboard](https://github.com/tikhomirovv/voice-keyboard) (Tauri / Vue / Rust). It expects `BACKEND_BASE_URL` and `TRANSCRIPTION_WS_URL` pointing at this service.

## Setup (archived snapshot)

### Prerequisites

- Go 1.24+
- Docker (for Postgres via Compose)
- Copy config: `config.yml.dist` → `config.yml` (gitignored; fill secrets locally)

### Database

```bash
make up          # postgres from docker-compose.yml
make migrate     # go run main.go database:migrations:migrate
```

### Run API

```bash
make wire        # only if you change DI wiring
make run         # go run main.go app:start
```

Health check: `GET /health` (see `make health`).

### Config highlights (`config.yml.dist`)

- `app.base_url` / `app.port` — public URL and listen port
- `database.*` — Postgres
- `auth.secret`, `auth.yandex.*` — JWT + Yandex OAuth app
- `openai.api_key` / `openai.model` — Realtime + text generation
- `releases.path` / `releases.actual_version` — updater artifacts layout
- `basic_auth.*` — optional basic auth for some surfaces
- `websocket.*` — connection limits / timeouts

`.env` / `.env.example` are mainly for Compose DB credentials in prod-style compose files.

### Docker

```bash
make build
make up-prod     # uses docker-compose.prod.yml (neutral placeholder host in sample)
```

CI samples (`.gitlab-ci.yml`) show a manual build/deploy pipeline; variable names are generic placeholders (`DEPLOY_HOST`, `SSH_PRIVATE_KEY_PRODUCTION`).

## Project notes (as left)

- Transcription depends on a live OpenAI API key and network access.
- Updater/release folders are expected on disk under `releases/` (binaries are not kept in git).
- Deploy compose/CI in the repo are examples with neutral hosts/paths, not a live production setup.

## License

See repository files for license information if present; otherwise treat as personal archived source.
