# sshdex

sshdex is a local-first SSH profile manager: an RDP-style command-line index for saving, browsing, editing, importing, and launching SSH targets without memorizing aliases or rebuilding `ssh` flags by hand.

sshdex stores SSH profile metadata and key file paths only. It does **not** store passwords, passphrases, or private key contents.

## Status

sshdex v1.0.0 is ready for general CLI use as a conservative, local-first SSH profile manager.

Implemented capabilities:

- Go single-binary CLI.
- Validated SSH profile model.
- Local JSON profile store.
- Deterministic OpenSSH argv generation.
- Safe shell-quoted dry-run preview.
- Interactive-first CLI CRUD: `add`, `list`, `show`, `edit`, `delete`.
- Connect by name: `connect NAME --dry-run` and shorthand `sshdex NAME --dry-run`.
- Search/select profile picker: `pick [--interactive] [--search Q] [--tag TAG] [--index N] [--dry-run]`.
- SSH forwarding metadata: `--local-forward`, `--remote-forward`, `--dynamic-forward`.
- Remote command metadata: `--remote-command`.
- `doctor` diagnostics.
- Import from OpenSSH config or sshdex JSON: `import [PATH] [--format openssh|sshdex] [--conflict skip|replace|rename] [--dry-run]`.
- Export/backup profile JSON: `export [PATH]`, `backup [PATH]`.
- Shell completions: `completion bash|zsh|fish`.

## Install

### Prebuilt release binaries

Download the archive for your OS/architecture from the GitHub Releases page:

<https://github.com/bbrandlegacy/sshdex/releases>

Each release includes Linux, macOS, and Windows builds plus `checksums.txt`.

Verify a downloaded asset from the directory containing the archive and checksum file:

```bash
sha256sum -c checksums.txt --ignore-missing
```

On macOS, if `sha256sum` is unavailable, filter the line for the asset you downloaded:

```bash
grep 'sshdex_1.0.0_darwin_arm64.tar.gz' checksums.txt | shasum -a 256 -c -
```

Replace the filename with the archive you downloaded.

Then unpack the archive and place `sshdex` on your `PATH`, for example:

```bash
tar -xzf sshdex_1.0.0_linux_amd64.tar.gz
install -m 0755 sshdex_1.0.0_linux_amd64/sshdex ~/.local/bin/sshdex
sshdex version
```

Windows releases are `.zip` archives containing `sshdex.exe`.

### `go install`

With Go 1.22 or newer:

```bash
go install github.com/bbrandlegacy/sshdex/cmd/sshdex@v1.0.0
sshdex version
```

For the latest development state on the default branch:

```bash
go install github.com/bbrandlegacy/sshdex/cmd/sshdex@main
```

Make sure Go's binary directory is on your `PATH` (usually `$(go env GOPATH)/bin` or `~/go/bin`).

### Build from source

```bash
git clone https://github.com/bbrandlegacy/sshdex.git
cd sshdex
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

## Quick start

```bash
sshdex add --name prod-web-01 --host 203.0.113.10 --user deploy --tag prod --tag web
sshdex list
sshdex prod-web-01 --dry-run
sshdex prod-web-01
```

Use `--store PATH` with any command to isolate tests, demos, or scripts from your default profile store:

```bash
sshdex --store /tmp/sshdex/profiles.json list
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

List numbered matches (script-friendly, non-interactive output):

```bash
sshdex pick
sshdex pick --tag prod
sshdex pick --search web
```

Select a numbered match and dry-run/connect it non-interactively:

```bash
sshdex pick --search web --index 1 --dry-run
sshdex pick --tag prod --index 2
```

Use the lightweight interactive picker when you want a keyboard-searchable prompt instead of copying an index from a separate list command:

```bash
sshdex pick --interactive --dry-run
sshdex pick --interactive --tag prod
```

The picker prompts for a search query, prints numbered matches, then accepts a selection number. Press Enter at the selection prompt to search again, or enter `q` to cancel. `--dry-run` previews the selected SSH command; without it, the selected profile is launched with the system `ssh` binary.

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

### Import from OpenSSH config or sshdex JSON

Preview importable entries:

```bash
sshdex import --dry-run
sshdex import ~/.ssh/config --dry-run
sshdex import --format sshdex ./sshdex-profiles.json --dry-run
```

Import entries:

```bash
sshdex import
sshdex import ~/.ssh/config
sshdex import --format sshdex ./sshdex-profiles.json
```

When no path is provided, `import` prompts for the import path. The default format is `openssh`; use `--format sshdex` to restore the JSON produced by `export` or `backup`. Import reads the source file and never overwrites it.

Duplicate profile names are non-destructive by default and skipped. Choose an explicit conflict policy when restoring JSON:

```bash
sshdex import --format sshdex --conflict skip ./sshdex-profiles.json     # default
sshdex import --format sshdex --conflict replace ./sshdex-profiles.json  # overwrite matching names
sshdex import --format sshdex --conflict rename ./sshdex-profiles.json   # import as name-1, name-2, ...
```

`--dry-run` prints the planned action for each profile and does not write the store.

Supported imported directives:

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

Wildcard host blocks like `Host *` are skipped. Forwarding values are validated before import or persistence: `LocalForward`/`RemoteForward` must use `[bind_address:]port:host:hostport`, `DynamicForward` must use `[bind_address:]port`, and all listen/target ports must be numeric values from 1 to 65535.

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

### Shell completions

Generate a completion script for your shell:

```bash
sshdex completion bash
sshdex completion zsh
sshdex completion fish
```

Common install locations:

```bash
# bash
mkdir -p ~/.local/share/bash-completion/completions
sshdex completion bash > ~/.local/share/bash-completion/completions/sshdex

# zsh
mkdir -p ~/.zfunc
sshdex completion zsh > ~/.zfunc/_sshdex
# Add this to ~/.zshrc if ~/.zfunc is not already in fpath:
# fpath=(~/.zfunc $fpath)
# autoload -Uz compinit && compinit

# fish
mkdir -p ~/.config/fish/completions
sshdex completion fish > ~/.config/fish/completions/sshdex.fish
```

### Doctor

```bash
sshdex doctor
```

Reports profile store path, profile count, whether `ssh` is available, and obvious invalid profile references such as missing identity files.

## Security posture

sshdex v1.0 follows a conservative local-first security posture:

- Does not store passwords.
- Does not store passphrases.
- Does not copy private key contents into the profile store.
- Stores only host/user/port metadata, SSH option metadata, tags, notes, and key file paths.
- Uses argv execution for OpenSSH rather than shell command strings.
- Provides dry-run previews that quote shell-special arguments for readability.
- Supports OpenSSH local, remote, and dynamic forwarding arguments as argv, not shell strings.
- Supports optional remote command metadata appended after the SSH target.
- Does not mutate `~/.ssh/config` during import.
- Redacts protected-looking values in CLI output/errors.

The research/architecture work behind this release treats SSH key management as a high-value security boundary. Password vaults, encrypted sync, team sharing, and MCP/AI agent access remain deliberately deferred until separate threat modeling and tests exist.

## v1.0 scope / deferred features

Deferred beyond v1.0:

- Password storage.
- age-encrypted credential vault.
- Team sharing.
- Git-backed sync.
- MCP server.
- 1Password/Bitwarden integration.
- Rich full-screen TUI profile picker.
- Visual jump-host/tunnel builder.
- Terminal init command automation.
- Native OS package installers.

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

## Release process

Releases are cut from tags named `vX.Y.Z`.

1. Update `internal/app.Version`, README install examples, and relevant tests.
2. Run the development validation ladder.
3. Tag the release, for example `git tag v1.0.0`.
4. Push the tag. The release workflow builds Linux/macOS/Windows archives and uploads `checksums.txt` to the GitHub Release.

Do not create release artifacts from an unverified working tree.
