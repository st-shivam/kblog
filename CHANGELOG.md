# Changelog

## v0.1.0 — 2026-05-25

First public release of kblog.

### What's in this release

**Multi-container log streaming**
Stream every container in a pod — or every replica of a deployment — in a single unified view. Each source gets a stable color-coded prefix so you can immediately tell who said what, even when containers are logging simultaneously.

**Live K8s event injection**
Pod lifecycle events appear inline with your logs as they happen — OOMKills, crash loop restarts, liveness probe failures, scheduling events. No more alt-tabbing to `kubectl get events` while something is on fire.

**Interactive JSON inspector**
Press `Enter` on any structured log line to open a syntax-highlighted key-value overlay. Works on mixed log streams regardless of framework or language. Press `Enter` or `Esc` to dismiss.

**Live search and field filtering**
Press `/` to search by plain text, regex, or `key=value` pairs (e.g. `level=error service=auth`). Non-matching lines dim in real-time; you never leave the stream.

**Severity level filters**
Keys `0`–`3` filter by ALL / ERROR / WARN / DEBUG. kblog auto-detects severity from common log formats (structured JSON `level` field, logrus, zap, stdlib prefixes).

**5 color themes**
Midnight (default), Dracula, Catppuccin Macchiato, Nord, and Monokai. Cycle with `t`.

**Clipboard copy**
`c` or `y` copies the current line. `v` starts a visual selection — extend it with the cursor, then `c` to copy the whole block. Works on macOS and Linux without any extra tools.

**k9s plugin**
Press `Shift-L` on any Pod or Deployment in k9s to open kblog in place of the default log view. The install script wires this up automatically.

### Platform support

Linux x86-64 and ARM64. macOS builds are not included in this release.

### Installation

```bash
curl -fsSL https://raw.githubusercontent.com/st-tripathi/kblog/main/install.sh | bash
```

Or grab a tarball from the assets below. See the [full installation guide](https://st-tripathi.github.io/kblog/installation) for all options.
