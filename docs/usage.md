
# Usage

- [CLI flags](#cli-flags)
- [Examples](#examples)
- [Keyboard reference](#keyboard-reference)
- [Search syntax](#search-syntax)
- [JSON inspector](#json-inspector)
- [Container sidebar](#container-sidebar)

---

## CLI flags

```
kblog --pod <name> [flags]
kblog --deployment <name> [flags]
```

| Flag | Description | Default |
|---|---|---|
| `--pod` | Pod name to tail | — |
| `--deployment` | Deployment name — tails all replicas | — |
| `--namespace` | Kubernetes namespace | current context / `default` |
| `--context` | Kubeconfig context | current context |
| `--tail` | Number of initial log lines | `200` |
| `--theme` | Starting color theme | `terminal` |
| `--version` | Print version and exit | — |

> `--pod` and `--deployment` are mutually exclusive. One must be provided.

---

## Examples

Tail a single pod:

```bash
kblog --pod auth-service-6f9d-x7b2p --namespace production
```

Tail all replicas of a deployment with a specific context:

```bash
kblog --deployment auth-service --namespace production --context prod-cluster
```

Start with a different theme:

```bash
kblog --pod payment-gateway --namespace billing --theme nord
```

Available themes: `terminal` · `midnight` · `dracula` · `catppuccin` · `nord` · `monokai`

---

## Keyboard reference

### Navigation

| Key | Action |
|---|---|
| `↑` / `k` | Scroll up |
| `↓` / `j` | Scroll down |
| `PgUp` / `PgDn` | Page up / page down |
| `f` | Follow — jump to latest and lock auto-scroll |
| `Tab` | Switch focus between log view and sidebar _(only when the sidebar is open)_ |

### Search & filter

| Key | Action |
|---|---|
| `/` | Open search bar |
| `0` | Show all lines |
| `1` | Show ERROR only |
| `2` | Show WARN and above |
| `3` | Show DEBUG and above |
| `l` | Toggle container sidebar |
| `Space` | Toggle container on/off _(sidebar focus)_ |

### Viewing & copying

| Key | Action |
|---|---|
| `Enter` | Open / close JSON inspector on selected line |
| `v` | Start visual selection |
| `c` / `y` | Copy selected line(s) to clipboard |
| `w` | Toggle line wrap |
| `s` | Toggle sort order (oldest ↑ / newest ↓) |
| `t` | Cycle themes |

### General

| Key | Action |
|---|---|
| `q` | Quit |
| `Esc` | Cancel selection, or quit |

---

## Search syntax

Open the search bar with `/`. Three modes are supported:

**Plain text** — case-insensitive substring match:

```
auth failed
```

**Regex** — full Go regex syntax:

```
(ERROR|WARN).*timeout
```

**Field filter** — `key=value` pairs for structured JSON logs:

```
level=error service=auth request_id=abc123
```

Multiple `key=value` pairs are ANDed together. Filters match against any key in the parsed JSON payload.

---

## JSON inspector

When `kblog` detects JSON in a log line, pressing `Enter` opens a syntax-highlighted key-value overlay. Press `Enter` or `Esc` to close.

The inspector works on mixed log streams — plain-text lines pass through normally and are still selectable.

---

## Container sidebar

Press `l` to open the **Filter Containers** sidebar. Focus lands there immediately — use `↑`/`↓` to navigate the list and `Space` to toggle individual containers on or off without stopping the stream. Press `Tab` to switch focus back to the log view while keeping the sidebar visible, or press `l` again to close it.

All containers are enabled by default.
