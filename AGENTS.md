# AGENTS.md — AI Guide for ctfdevkit-cli

This file tells AI assistants (OpenAI Codex, Devin, GitHub Copilot, Claude Code, etc.)
everything they need to know about this project and how to work with the owner.

---

## Project overview

**ctfdevkit-cli** is a Go CLI tool built for [Cyberbrein NL](https://cyberbrein.nl).
It lets CTF (Capture the Flag) participants spin up a local challenge environment
with a single command. Under the hood it orchestrates Docker Compose, generates
trusted HTTPS certificates via mkcert, and manages `/etc/hosts` bindings.

**GitHub repo:** `stichting-Cyberbrein-nl/ctfdevkit-cli`  
**Go module:** `github.com/stichting-Cyberbrein-nl/ctfdevkit-cli`  
**Owner:** sympact06

### What it ships

| Component | What it is |
|-----------|-----------|
| `devkit` binary | The CLI — built from `cmd/devkit/main.go` |
| Docker payload | A Laravel app + MySQL + Caddy reverse proxy (`sympactdev/ctfdevkit:<version>`) |
| `manifest.json` | Single source of truth for latest versions and download URLs — **auto-updated by CI on every release** |

### Stack

- **Language:** Go 1.24.2
- **CLI framework:** Cobra (`github.com/spf13/cobra`)
- **TUI:** Bubble Tea + Lip Gloss (charmbracelet)
- **Releases:** GoReleaser v2
- **CI/CD:** GitHub Actions

---

## Repository layout

```
cmd/devkit/main.go          Entry point — injects version via -ldflags
internal/
  assignments/              Auto-detects CTF assignment directories
  browser/                  Opens URLs in the default browser
  certs/                    mkcert install + TLS cert generation
  cli/                      All Cobra commands (one file per command)
  config/                   config.json loader/saver + env overrides
  docker/                   Docker daemon detection, compose helpers
  doctor/                   Health-check runner
  hosts/                    /etc/hosts + Windows hosts file management
  output/                   Colored terminal output helpers
  payload/                  Downloads + installs the Docker Compose bundle
  platform/                 OS/arch detection (Linux, macOS, Windows, WSL)
  ports/                    Port conflict detection + forced release
  prereqs/                  Installs Docker + mkcert if missing
  releases/                 Fetches manifest.json, downloads + verifies binaries
  state/                    Tracks installed versions (state.json)
  tui/                      Interactive TUI menu (Bubble Tea)
  update/                   Self-update logic (atomic binary replacement)
scripts/
  install.sh                Linux one-liner installer (Debian / Ubuntu / Kali)
  install.ps1               Windows one-liner installer (PowerShell)
.goreleaser.yaml            GoReleaser config — builds 5 platform targets
.github/workflows/
  ci.yml                    Runs on every push to main: vet, test, cross-compile check
  release.yml               Runs on tag push (v*.*.*): GoReleaser + manifest.json update
manifest.json               Auto-updated by release workflow — do not edit manually
Makefile                    Local cross-compile helpers (make build, make test)
```

---

## Key data flows

### Self-update
```
user: devkit self-update
  → fetches manifest.json from raw.githubusercontent.com/…/main/manifest.json
  → compares .cli.version with current binary version (injected at build time)
  → if newer: downloads the platform asset, verifies SHA256, replaces binary
      Linux/macOS: atomic os.Rename
      Windows:     detached batch script (handles file-lock)
```

### Release pipeline
```
git tag v1.2.3 && git push origin v1.2.3
  → GitHub Actions: release.yml
      → GoReleaser builds 5 binaries + archives + checksums.txt
      → gh release create with all assets
      → jq updates manifest.json (version + URLs + SHA256 per platform)
      → git commit "chore: update manifest for v1.2.3 [skip ci]" → push to main
```

### Payload install
```
devkit setup
  → downloads docker-compose.yml + Caddyfile + .env.example from payload bundle
  → generates mkcert TLS cert for configured domain
  → binds domain in /etc/hosts (or Windows hosts file)
  → devkit up → docker compose up -d
```

---

## Platform support

The CLI targets **Linux, macOS, and Windows** (native + WSL).

| Platform | Binary | Notes |
|----------|--------|-------|
| linux-amd64 | `.tar.gz` | Primary target |
| linux-arm64 | `.tar.gz` | Raspberry Pi, ARM servers |
| darwin-amd64 | `.tar.gz` | Intel Mac |
| darwin-arm64 | `.tar.gz` | Apple Silicon |
| windows-amd64 | `.zip` | Native Windows — hosts file via PowerShell UAC elevation |

**Important platform notes:**
- `GOARCH=amd64` must be set for `go test` locally — the dev machine defaults to `386`
- Windows hosts file automation uses PowerShell UAC elevation (triggers a UAC dialog)
- `mkcert -install` on Windows requires Administrator; the CLI shows a hint if it fails
- WSL is detected via `WSL_DISTRO_NAME` env var or `/proc/version` content

---

## Config + state files

| File | Location (Linux/macOS) | Location (Windows) |
|------|------------------------|-------------------|
| `config.json` | `~/.config/devkit/config.json` | `%APPDATA%\devkit\config.json` |
| `state.json` | `~/.config/devkit/state.json` | `%APPDATA%\devkit\state.json` |
| Payload dir | `~/.local/share/devkit/payload/` | `%LOCALAPPDATA%\devkit\payload\` |
| Certs dir | `~/.local/share/devkit/payload/certs/` | `%LOCALAPPDATA%\devkit\payload\certs\` |

`manifest.json` is fetched at runtime from:
```
https://raw.githubusercontent.com/stichting-Cyberbrein-nl/ctfdevkit-cli/main/manifest.json
```
Override with env var `DEVKIT_MANIFEST_URL` or `--manifest-url` flag.

---

## Commands

| Command | What it does |
|---------|-------------|
| `devkit setup` | Full first-time setup: prereqs → certs → hosts → pull image |
| `devkit up` | Start Docker Compose stack |
| `devkit down` | Stop stack |
| `devkit reset` | Stop + wipe volumes |
| `devkit status` | Show container status |
| `devkit logs` | Stream container logs |
| `devkit shell` | Interactive shell inside the app container |
| `devkit doctor` | Health check |
| `devkit certs` | Regenerate TLS certificates |
| `devkit bind-hosts` | Update /etc/hosts entry |
| `devkit self-update` | Update the CLI binary |
| `devkit update` | Pull the latest Docker payload image |
| `devkit version` | Show current version |

---

## ⚠️ CRITICAL: ALWAYS ASK BEFORE RELEASING

**Before pushing any git tag or triggering a release, you MUST ask the owner:**

> "Do you want to create a release (new version tag) or just commit these changes to main?"

**A release means:**
1. Pushing a `vX.Y.Z` tag to GitHub
2. GitHub Actions builds 5 platform binaries
3. A public GitHub Release is created with download assets
4. `manifest.json` is automatically updated with new URLs + SHA256 hashes
5. All existing users who run `devkit self-update` will be offered this version

**This is irreversible once published.** Deleting a release confuses users who are
mid-update. Always confirm with the owner first.

### When the owner says "commit" / "push" / "save changes"
→ `git add` + `git commit` + `git push origin main`  
→ Do **not** create a tag  
→ Do **not** touch `manifest.json` manually

### When the owner explicitly says "release" / "new version" / "ship it"
→ Ask: **"Which version number? (current: vX.Y.Z)"**  
→ After confirmation: `git tag vX.Y.Z && git push origin vX.Y.Z`  
→ GoReleaser and the release workflow handle everything else automatically

### Version numbering
Follow semantic versioning (`MAJOR.MINOR.PATCH`):
- **PATCH** (`1.0.1`) — bug fixes, no new features
- **MINOR** (`1.1.0`) — new features, backwards compatible
- **MAJOR** (`2.0.0`) — breaking changes

---

## Development workflow

### Build locally
```bash
# Build for the current machine (force amd64)
GOARCH=amd64 go build -o devkit ./cmd/devkit

# Cross-compile for all platforms
make build          # outputs to dist/

# Run tests
make test           # = GOARCH=amd64 go test ./...
```

### Add a new CLI command
1. Create `internal/cli/<name>.go` with a `newXxxCmd()` function
2. Register it in `internal/cli/root.go` inside `func init()`

### Test self-update locally
```bash
DEVKIT_MANIFEST_URL=file:///path/to/manifest.json devkit self-update
```

---

## Things to never do without asking

- `git tag` + `git push origin <tag>` — triggers a public release
- Edit `manifest.json` by hand — it is auto-maintained by CI
- Force-push to `main`
- Delete published releases or tags (breaks self-update for users mid-download)
- Change the archive naming format in `.goreleaser.yaml` without updating
  the manifest URL patterns in `internal/config/config.go` and the install scripts

---

## Owner preferences

- Commits in English
- Commit messages are concise and describe *why*, not just *what*
- No unnecessary refactoring alongside feature work — one thing per PR/commit
- Ask before adding new dependencies to `go.mod`
- The local Go environment uses `GOARCH=386` by default — always use `GOARCH=amd64`
  for builds and tests
