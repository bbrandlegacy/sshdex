# Research: sshdex — SSH Connection Manager CLI/TUI

**Date:** 2026-06-08
**Status:** BMAD-ready product brief input
**Category:** Developer Tool / DevOps

---

## Problem

Every developer manages SSH connections. The workflow today is terrible:
- Memorize or grep `~/.ssh/config` for hostnames
- Google the right flags every time (`-i`, `-L`, `-J` for jump hosts)
- Passwords stored nowhere or insecurely in plaintext scripts
- No way to group/label connections (personal, work, client A, client B)
- SSH config has no UX — it's a flat text file with cryptic syntax
- Windows RDP app does this RIGHT (saved profiles, one-click connect, visual list) — SSH has no equivalent
- Switching between machines means re-copying SSH config manually

---

## Existing Competition

### sshs
- **Language:** Go
- **GitHub:** quantumsheep/sshs
- **Stars:** ~2,000+
- **Features:** TUI browser over `~/.ssh/config`, fuzzy search, keyboard navigation, launches SSH from selection
- **Limitations:** Read-only of existing config — no add/edit/delete from TUI, no password storage, no grouping/tags, no cross-machine sync, no encrypted credential store
- **Status:** Active, but narrow scope

### Storm (stormssh)
- **Language:** Python
- **GitHub:** emre/storm
- **Stars:** ~4,000
- **Features:** Add/delete/edit SSH hosts from CLI (`storm add`, `storm delete`), list hosts, search
- **Limitations:** Python dependency, no TUI, no encryption, no password storage, no UI to browse/select, CLI-only management
- **Status:** Moderately active, older project

### assh (Advanced SSH config)
- **Language:** Go
- **GitHub:** moul/assh
- **Stars:** ~3,000
- **Features:** SSH config manager with includes, templates, hooks, ProxyJump generation, Kubernetes integration
- **Limitations:** Config management only — no TUI, no credential storage, no interactive connection browser, power-user only
- **Status:** Active

### Shuttle (Mac app)
- **Language:** Swift (macOS only)
- **Type:** macOS menu bar app
- **Features:** SSH shortcuts from menu bar, simple profile storage
- **Limitations:** macOS only, GUI not CLI, no encryption, no cross-platform
- **Status:** Maintained

### sshwifty
- **Language:** Go
- **Type:** Web-based SSH client
- **Features:** SSH in browser, self-hosted
- **Limitations:** Requires running server, not a CLI tool
- **Status:** Active

### Teleport
- **Language:** Go
- **Type:** Enterprise access platform
- **Features:** Certificate-based auth, audit logs, team access controls
- **Limitations:** Enterprise product, heavyweight, requires infrastructure, overkill for personal use
- **Pricing:** Paid for enterprise features

### tmux + SSH scripts
- Common pattern: shell scripts or tmux sessionizer that wraps SSH
- No encryption, no UX, personal scripts only

---

## Feature Gap Matrix

| Feature | sshs | Storm | assh | sshdex (proposed) |
|---------|------|-------|------|---------------------|
| TUI browser | Yes | No | No | Yes |
| Add/edit hosts | No | Yes (CLI) | Yes (CLI) | Yes (TUI) |
| Encrypted password store | No | No | No | Yes (age) |
| Grouping / tags | No | No | No | Yes |
| Jump host management | No | Partial | Yes | Yes |
| Port forwarding profiles | No | No | Partial | Yes |
| Cross-machine sync | No | No | No | Yes (encrypted export) |
| Windows support | No | No | Partial | Yes |
| Terminal customization on connect | No | No | No | Yes |
| Key file management | No | No | No | Yes |

---

## Proposed Tool: `sshdex`

### Core Vision
Windows RDP-style profile manager for SSH. Pick your machine from a beautiful TUI list, connect. Secrets encrypted locally. Single Go binary, works on macOS/Linux/Windows.

### Key Features

**Core (v0.1)**
- `sshdex add` — wizard to add SSH profile (host, user, port, key file, password optional)
- `sshdex list` — TUI browser: scroll through profiles, fuzzy search, group by tag
- `sshdex connect <name>` — connect by name without remembering IP/user
- `sshdex <name>` — shorthand connect
- Profile storage: encrypted JSON vault using `age`
- Groups/tags: organize by project, client, environment
- Import from `~/.ssh/config` — auto-migrate existing setup

**TUI (v0.2)**
- RDP-style visual profile picker (your PlexLinker tcell skills apply directly)
- Search bar at top — fuzzy filter as you type
- Profile detail panel: shows host, user, port, last connected, tags
- Quick actions: connect, edit, delete, copy SSH command
- Status indicators: last connected timestamp, connection count
- Keyboard shortcuts (hjkl / arrow keys)
- Recent connections list

**Security (v0.2)**
- Passwords encrypted at rest with `age`
- Vault unlock: passphrase or key file
- Optional: no password storage (key-only auth)
- `sshdex lock` — clear decrypted state from memory

**Advanced (v0.3)**
- **Port forwarding profiles** — save tunnel configs, launch with one key (`-L 5432:localhost:5432`)
- **Jump host chains** — define ProxyJump sequences visually
- **Terminal customization on connect** — send init string after connecting (set PS1, run tmux, run `screen`, source profile)
- **SSH key management** — list keys, associate key per profile, generate new key
- **Connection scripts** — run local script before/after connect (set env, open browser, etc.)
- **Multi-hop sessions** — visual chain builder for bastion → target patterns

**Sync (v1.0)**
- `sshdex export` — encrypted bundle for cross-machine sync
- `sshdex import bundle.sshdex` — import on new machine
- Git-backed sync (optional, encrypted)
- 1Password / Bitwarden export

**AI Integration**
- `sshdex mcp` — MCP server exposing `sshdex_connect`, `sshdex_list` tools
- AI agent can SSH into a machine and run commands without seeing credentials
- Same pattern as envctl's secure exec proxy

---

## Terminal Customization Feature (Key Differentiator)

No existing tool does this. When connecting, `sshdex` can:

```yaml
profile: prod-web-01
host: 192.168.1.10
user: deploy
init_commands:
  - "export PS1='[PROD] \u@\h:\w\$ '"
  - "tmux new-session -A -s main"
  - "cd /var/www/app"
startup_message: "⚠️  PRODUCTION SERVER"
```

Injected via SSH pseudo-terminal after connect. Dev sees custom prompt, right directory, tmux session — automatically.

---

## Platform Notes

### macOS
- `age` encryption, keychain integration optional
- Launch via Terminal / iTerm2
- `brew install` distribution

### Linux
- GNOME Keyring / KWallet integration optional
- Works in any terminal
- `apt`/`yum` packages + `go install`

### Windows
- Windows Credential Manager integration optional
- PowerShell / Windows Terminal support
- OpenSSH for Windows (built-in since Win10)
- `.exe` single binary via `go install` or installer

---

## Target Users
- Developers managing 5+ servers (everyone loses count)
- DevOps engineers with dozens of client/environment hosts
- Freelancers with per-client server access
- Homelab enthusiasts (Raspberry Pi clusters, NAS boxes, VMs)
- Teams that need to share server access securely

---

## Monetization Path
- Free / MIT core
- Pro: encrypted sync, team profiles, audit log, 1Password integration — $4/mo
- Team: shared encrypted vault, read-only profiles for junior devs — $6/user/mo

---

## Build Order
1. Profile CRUD + connect command (no TUI) — Week 1
2. `age` encrypted vault — Week 1
3. TUI browser (tcell, your existing skills) — Week 2
4. Import from `~/.ssh/config` — Week 2
5. Port forwarding profiles — Week 3
6. Terminal init commands — Week 3
7. Export/import bundle — Week 4

---

## Resume / Portfolio Signal
- Encryption (age)
- TUI (complex multi-pane)
- Process management (SSH subprocess, PTY handling)
- Cross-platform (Windows SSH quirks are non-trivial)
- Security thinking (credential storage, vault design)

---

## Risks
- PTY/pseudo-terminal handling is non-trivial on Windows — `golang.org/x/crypto/ssh` helps
- Password storage is security-sensitive — must be done right or skip it (key-only auth first)
- **Mitigation:** v0.1 supports key-file auth only, no password storage; add encrypted passwords in v0.2 after vault design is solid
