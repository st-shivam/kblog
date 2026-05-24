---
title: k9s Integration
nav_order: 4
---

# k9s Integration
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

`kblog` ships as a drop-in replacement for the k9s log viewer. Once installed, press `Shift-L` on any Pod or Deployment to open `kblog` in place of the default log screen.

---

## How k9s plugins work

When you highlight a resource in k9s and press a configured hotkey:

1. k9s suspends its render loop.
2. It executes the plugin command, handing over full terminal control.
3. k9s passes the resource name, namespace, and cluster context as flags.
4. When the command exits (press `q` in `kblog`), k9s restores the previous view.

---

## Automatic setup

The install script and `make install` install the plugin automatically:

```bash
curl -fsSL https://raw.githubusercontent.com/st-tripathi/kblog/main/install.sh | bash
# or
make install
```

Restart k9s after installation. The `Shift-L` binding will appear in the hotkey bar.

---

## Manual setup

If you prefer to configure the plugin manually, add the following to `~/.config/k9s/plugins.yaml` (create the file if it doesn't exist):

```yaml
plugins:
  kblog-pod:
    shortCut: Shift-L
    confirm: false
    description: "kblog"
    scopes:
      - pods
    command: kblog
    background: false
    args:
      - --context
      - $CONTEXT
      - --namespace
      - $NAMESPACE
      - --pod
      - $NAME

  kblog-deployment:
    shortCut: Shift-L
    confirm: false
    description: "kblog"
    scopes:
      - deployments
    command: kblog
    background: false
    args:
      - --context
      - $CONTEXT
      - --namespace
      - $NAMESPACE
      - --deployment
      - $NAME
```

Restart k9s to load the configuration.

---

## Plugin variables

k9s injects these variables automatically when invoking the plugin:

| Variable | Description |
|---|---|
| `$CONTEXT` | Active kubeconfig cluster context |
| `$NAMESPACE` | Active namespace |
| `$NAME` | Name of the highlighted resource |

---

## Troubleshooting

**`Shift-L` does nothing or shows "command not found"**

k9s cannot find the `kblog` binary in its PATH. Ensure `kblog` is installed in a directory that's on your PATH (e.g. `/usr/local/bin`), or specify the absolute path in `plugins.yaml`:

```yaml
command: /usr/local/bin/kblog
```

**Logs or events fail to load**

Your kubeconfig context may lack permissions to read pod logs or cluster events. Verify:

```bash
kubectl auth can-i get pods --namespace <ns>
kubectl auth can-i get events --namespace <ns>
```

**Colors look wrong or modal is distorted**

Ensure your terminal supports 24-bit truecolor. Set `COLORTERM=truecolor` in your shell profile if needed. Tested terminals: iTerm2, Alacritty, WezTerm, GNOME Terminal, Kitty.

---

## Plugin config path by OS

| OS | Default path |
|---|---|
| macOS / Linux | `~/.config/k9s/plugins.yaml` |
| macOS (Homebrew k9s) | `~/Library/Application Support/k9s/plugins.yaml` |
