# kblog

[![License: MIT](https://img.shields.io/badge/License-MIT-blueviolet.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.26%2B-00ADD8.svg?style=flat&logo=go)](https://golang.org)
[![Kubernetes API](https://img.shields.io/badge/K8s%20API-v1.28%2B-326CE5.svg?style=flat&logo=kubernetes)](https://kubernetes.io)
[![Platform support](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-lightgrey.svg)](https://golang.org)
[![CI](https://github.com/st-shivam/kblog/actions/workflows/ci.yml/badge.svg)](https://github.com/st-shivam/kblog/actions/workflows/ci.yml)

**`kblog`** is a Kubernetes log-tailing TUI that streams pod and deployment logs with real-time K8s event injection, interactive JSON inspection, severity filtering, and themes.

Works as both a **standalone CLI tool** and a **drop-in k9s plugin** (press `Shift-L` on any pod or deployment). A single Go binary, no runtime dependencies.

**[Full documentation →](docs/)**

---

## Features

- **Concurrent multi-container streaming** — stream all containers in a pod or all replicas of a deployment at once, each line color-coded by source.
- **Live K8s event injection** — `OOMKilled`, `CrashLoopBackOff`, liveness probe failures injected directly into the log timeline.
- **Interactive JSON inspector** — press `Enter` on any JSON log line for a syntax-highlighted key-value overlay.
- **Live search** — plain text, regex, or `key=value` field filters (e.g. `level=error request_id=abc`).
- **6 color themes** — Terminal (default), Midnight, Dracula, Catppuccin Macchiato, Nord, Monokai. Cycle with `t`.
- **Clipboard copy** — single line or multiline visual selection (`c`/`y`).
- **50k-line rolling buffer** with O(1) incremental filter path.

---

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/st-shivam/kblog/main/install.sh | bash
```

Then:

```bash
kblog --pod <pod-name> --namespace <namespace>
kblog --deployment <deployment-name> --namespace <namespace>
```

See [Installation](https://github.com/st-shivam/kblog/blob/main/docs/installation.md) for all options including manual installs, version pinning, and build from source.

---

## TUI layout

```
+-------------------------------------------------------------+
| kblog v0.1.0 | Context: production | Pod: auth-service-xyz  |
+------------------------------------------------+------------+
| 12:00:01 [auth] INF -> Initializing DB...      | Containers |
| 12:00:02 [auth] INF -> DB Connection OK        | [x] auth   |
| 12:00:03 [K8S EVENT] Liveness probe succeeded  | [ ] helper |
| 12:00:04 [auth] ERR -> Auth failed for user x  |            |
|   {                                            |            |
|     "user_id": "usr_9013",                     |            |
|     "error_code": "INVALID_TOKEN"              |            |
|   }                                            |            |
| 12:00:05 [auth] INF -> Tailing...              |            |
+------------------------------------------------+------------+
| Filter: /auth-failed                           | Help: q=ex |
+-------------------------------------------------------------+
```

---

## Key bindings

| Key | Action |
|---|---|
| `q` / `Esc` | Quit |
| `up`/`k` `down`/`j` | Scroll |
| `pgup` / `pgdown` | Page up / down |
| `f` | Follow (lock to latest) |
| `/` | Search |
| `0`–`3` | Level filter (All / ERROR / WARN / DEBUG) |
| `Enter` | Open JSON inspector |
| `v` | Visual selection |
| `c` / `y` | Copy to clipboard |
| `w` | Toggle line wrap |
| `s` | Toggle sort order |
| `t` | Cycle themes |
| `l` | Toggle container filter sidebar (focus lands there immediately) |
| `Tab` | Switch focus between log view and sidebar _(sidebar must be open)_ |

---

## k9s plugin

`kblog` integrates as a k9s plugin. The install script sets it up automatically. Press `Shift-L` on any pod or deployment to open kblog in place of the default log view.

See [k9s Integration](https://github.com/st-shivam/kblog/blob/main/docs/k9s-integration.md) for manual setup.

---

## Build from source

Requires Go 1.26+.

```bash
git clone https://github.com/st-shivam/kblog.git
cd kblog
make install INSTALL_PATH=$HOME/.local/bin
```

---

## License

[MIT](LICENSE)
