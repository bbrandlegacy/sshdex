# sshdex

sshdex is a local-first SSH profile manager: an RDP-style command-line index for saving, browsing, editing, importing, and launching SSH targets without memorizing aliases or rebuilding `ssh` flags by hand.

v0.1 is intentionally conservative. It stores SSH profile metadata and key file paths only. It does not store passwords, passphrases, or private key contents.

## Status

v0.4 is in progress on the finish-line train. v0.3.0 is the current public release.

Implemented capabilities:

- Go single-binary CLI skeleton.
- Validated SSH profile model.
- Local JSON profile store.
- Deterministic OpenSSH argv generation.
- Safe shell-quoted dry-run preview.
- Interactive-first CLI CRUD: `add`, `list`, `show`, `edit`, `delete`.
- Connect by name: `connect NAME --dry-run` and shorthand `sshdex NAME --dry-run`.
- Search/select profile picker: `pick [--search Q] [--tag TAG] [--index N] [--dry-run]`.
- SSH forwarding metadata: `--local-forward`, `--remote-forward`, `--dynamic-forward`.
- Remote command metadata: `--remote-command`.
- `doctor` diagnostics.
- Import from OpenSSH config: `import [PATH] [--dry-run]`.
- Export/backup profile JSON: `export [PATH]`, `backup [PATH]`.

## Install / build from source

```bash
go build -o sshdex ./cmd/sshdex
./sshdex version
```

For local smoke testing without touching your real profile store:

```bash
STORE=/tmp/sshdex-demo/profiles.json
mkdir -p /tmp/sshdex-demo
go build -o /tmp/sshdex-demo/sshdex ./cmd/sshdex
/tmp/sshdex-demo/sshdex --store "$STORE" doctor
```

## Commands

### Add a profile

Interactive default:

```bash
sshdex add
```

`add` prompts for name, host, user, port, identity file, tags, notes, ProxyJump, forwards, and remote command. For scripts/automation, flags are still accepted:

```bash
sshdex add \
  --name prod-web-01 \
  --host 203.0.113.10 \
  --user deploy \
  --port 22 \
  --identity-file ~/.ssh/id_ed25519 \
  --tag prod \
  --tag web \
  --notes "main production web host"
```

Optional SSH flow flags can be repeated where OpenSSH allows them:

```bash
sshdex add \
  --name prod-db-tunnel \
  --host bastion.example.com \
  --user deploy \
  --local-forward 127.0.0.1:15432:db.internal:5432 \
  --dynamic-forward 127.0.0.1:1080 \
  --remote-command "uptime -p"
```

### List profiles

```bash
sshdex list
sshdex list --tag prod
sshdex list --search web
```

### Show a profile

```bash
sshdex show prod-web-01
```

### Edit a profile

```bash
sshdex edit prod-web-01
```

`edit NAME` prompts through the existing values and keeps defaults when you press Enter. Automation flags are still accepted:

```bash
sshdex edit prod-web-01 --host 203.0.113.11 --tag prod --tag web
sshdex edit prod-db-tunnel --local-forward 127.0.0.1:15433:db.internal:5432
```

### Delete a profile

```bash
sshdex delete prod-web-01
```

`delete NAME` asks for confirmation. For non-interactive automation, use `--force`:

```bash
sshdex delete prod-web-01 --force
```

### Pick/search profiles

List numbered matches:

```bash
sshdex pick
sshdex pick --tag prod
sshdex pick --search web
```

Select a numbered match and dry-run/connect it:

```bash
sshdex pick --search web --index 1 --dry-run
sshdex pick --tag prod --index 2
```

### Connect / dry run

Dry-run prints the OpenSSH command preview without launching SSH:

```bash
sshdex connect prod-web-01 --dry-run
sshdex prod-web-01 --dry-run
```

Non-dry-run launches the system `ssh` binary with argv arguments, not a shell-concatenated string.

```bash
sshdex connect prod-web-01
sshdex prod-web-01
```

### Import from SSH config

Preview importable entries:

```bash
sshdex import --dry-run
sshdex import ~/.ssh/config --dry-run
```

Import entries:

```bash
sshdex import
sshdex import ~/.ssh/config
```

When no path is provided, `import` prompts for the SSH config path. Import reads the source SSH config and never overwrites it. Duplicate profile names are skipped.

Supported imported directives in v0.1:

- `Host`
- `HostName`
- `User`
- `Port`
- `IdentityFile`
- `ProxyJump`
- `LocalForward`
- `RemoteForward`
- `DynamicForward`
- `RemoteCommand`

Wildcard host blocks like `Host *` are skipped.

### Export / backup

```bash
sshdex export
sshdex backup
```

Both commands prompt for a destination path when omitted. For automation, pass the path directly:

```bash
sshdex export ./sshdex-profiles.json
sshdex backup ./sshdex-profiles.backup.json
```

Export and backup write the same secret-free profile JSON format used by the local store.

### Doctor

```bash
sshdex doctor
```

Reports profile store path, profile count, whether `ssh` is available, and obvious invalid profile references such as missing identity files.

### Test-only store override

All commands accept a global store override:

```bash
sshdex --store /tmp/sshdex/profiles.json list
```

## Security posture

sshdex v0.1 follows a conservative local-first security posture:

- Does not store passwords.
- Does not store passphrases.
- Does not copy private key contents into the profile store.
- Stores only metadata and key file paths.
- Uses argv execution for OpenSSH rather than shell command strings.
- Provides dry-run previews that quote shell-special arguments for readability.
- Supports OpenSSH local, remote, and dynamic forwarding arguments as argv, not shell strings.
- Supports optional remote command metadata appended after the SSH target.
- Does not mutate `~/.ssh/config` during import.

The research/architecture work behind this release treats SSH key management as a high-value security boundary. Password vaults, encrypted sync, team sharing, and MCP/AI agent access are deliberately deferred until separate threat modeling and tests exist.

## v0.1 limits / deferred features

Deferred beyond v0.1:

- Password storage.
- age-encrypted credential vault.
- Team sharing.
- Git-backed sync.
- MCP server.
- 1Password/Bitwarden integration.
- Rich TUI profile picker.
- Visual jump-host/tunnel builder.
- Terminal init command automation.
- Production Windows installer.

## Development validation

Use the release validation ladder:

```bash
go test ./...
go vet ./...
go test -race ./...
go test -cover ./...
go build -o /tmp/sshdex-preprod/sshdex ./cmd/sshdex
git diff --check
test -z "$(gofmt -l cmd internal)"
```

Optional smoke test:

```bash
STORE=/tmp/sshdex-smoke/profiles.json
BIN=/tmp/sshdex-preprod/sshdex
mkdir -p /tmp/sshdex-smoke
$BIN --store "$STORE" add --name demo --host example.com --user deploy --tag demo
$BIN --store "$STORE" list --search demo
$BIN --store "$STORE" demo --dry-run
$BIN --store "$STORE" doctor
```
