---
title: Installation
nav_order: 2
---

# Installation
{: .no_toc }

## Table of contents
{: .no_toc .text-delta }

1. TOC
{:toc}

---

## Requirements

- Linux or macOS (x86-64 or ARM64)
- A working kubeconfig (`~/.kube/config` or `KUBECONFIG` env var)
- Go 1.26+ only required for building from source

---

## Option 1 — Install script (recommended)

Downloads the correct pre-built binary for your platform and installs the k9s plugin automatically.

```bash
curl -fsSL https://raw.githubusercontent.com/st-tripathi/kblog/main/install.sh | bash
```

**Custom install directory** (no `sudo` required):

```bash
INSTALL_DIR=$HOME/.local/bin \
  curl -fsSL https://raw.githubusercontent.com/st-tripathi/kblog/main/install.sh | bash
```

Add `~/.local/bin` to your PATH if not already there:

```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc
# zsh: replace ~/.bashrc with ~/.zshrc
```

**Pin to a specific version:**

```bash
curl -fsSL https://raw.githubusercontent.com/st-tripathi/kblog/main/install.sh | VERSION=v0.1.0 bash
```

---

## Option 2 — Release tarball

Download the archive for your platform from the [Releases page](https://github.com/st-tripathi/kblog/releases):

| Platform | Archive |
|---|---|
| Linux x86-64 | `kblog_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `kblog_<version>_linux_arm64.tar.gz` |

Extract and install:

```bash
tar -xzf kblog_<version>_linux_amd64.tar.gz
sudo mv kblog /usr/local/bin/
kblog --help
```

Install the k9s plugin bundled in the archive:

```bash
cat plugin/plugins.yaml >> ~/.config/k9s/plugins.yaml
```

---

## Option 3 — Build from source

Requires [Go 1.26+](https://go.dev/doc/install).

```bash
git clone https://github.com/st-tripathi/kblog.git
cd kblog

# Install to /usr/local/bin (requires sudo)
sudo make install

# Or install to a local directory (no sudo)
make install INSTALL_PATH=$HOME/.local/bin
```

`make install` builds the binary, copies it to the target directory, and installs the k9s plugin in one step.

---

## Verify installation

```bash
kblog --help
```

You should see the full usage output with all available flags.
