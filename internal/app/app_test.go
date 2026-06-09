package app

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func runCLI(t *testing.T, storePath string, args ...string) (string, string, int) {
	t.Helper()
	return runCLIWithInput(t, "", storePath, args...)
}

func runCLIWithInput(t *testing.T, input, storePath string, args ...string) (string, string, int) {
	t.Helper()
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdin: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close stdin writer: %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		_ = r.Close()
	}()
	var stdout, stderr bytes.Buffer
	code := Run(argsWithStore(storePath, args...), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func argsWithStore(storePath string, args ...string) []string {
	out := []string{"--store", storePath}
	return append(out, args...)
}

func TestCLIInteractiveAddAndDeleteConfirmation(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	input := strings.Join([]string{
		"interactive-prod",
		"interactive.example.com",
		"deploy",
		"2200",
		"~/.ssh/interactive",
		"prod,web",
		"interactive notes",
		"bastion",
		"127.0.0.1:15432:db.internal:5432",
		"",
		"",
		"uptime -p",
		"",
	}, "\n")
	stdout, stderr, code := runCLIWithInput(t, input, storePath, "add")
	if code != 0 {
		t.Fatalf("interactive add code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Name:") || !strings.Contains(stdout, "added interactive-prod") {
		t.Fatalf("interactive add stdout missing prompts/result: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "show", "interactive-prod")
	if code != 0 {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{"Host: interactive.example.com", "User: deploy", "Port: 2200", "Tags: prod,web", "ProxyJump: bastion", "LocalForwards: 127.0.0.1:15432:db.internal:5432", "RemoteCommand: uptime -p"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("show missing %q: %q", want, stdout)
		}
	}

	stdout, stderr, code = runCLIWithInput(t, "y\n", storePath, "delete", "interactive-prod")
	if code != 0 {
		t.Fatalf("interactive delete code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Delete interactive-prod?") || !strings.Contains(stdout, "deleted interactive-prod") {
		t.Fatalf("interactive delete stdout unexpected: %q", stdout)
	}
}

func TestCLIAddListShowEditDelete(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	stdout, stderr, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "example.com", "--user", "deploy", "--port", "2200", "--identity-file", "~/.ssh/prod key", "--tag", "prod", "--tag", "web", "--notes", "main box")
	if code != 0 {
		t.Fatalf("add code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "added prod") {
		t.Fatalf("add stdout = %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "list", "--tag", "prod")
	if code != 0 {
		t.Fatalf("list code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "prod") || !strings.Contains(stdout, "example.com") {
		t.Fatalf("list stdout missing profile: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "show", "prod")
	if code != 0 {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Name: prod") || !strings.Contains(stdout, "IdentityFile: ~/.ssh/prod key") {
		t.Fatalf("show stdout unexpected: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "edit", "prod", "--host", "new.example.com", "--tag", "prod")
	if code != 0 {
		t.Fatalf("edit code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, _, _ = runCLI(t, storePath, "list", "--search", "new")
	if !strings.Contains(stdout, "new.example.com") {
		t.Fatalf("edited host not listed: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "delete", "prod", "--force")
	if code != 0 {
		t.Fatalf("delete code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	stdout, _, _ = runCLI(t, storePath, "list")
	if strings.Contains(stdout, "prod") {
		t.Fatalf("deleted profile still listed: %q", stdout)
	}
}

func TestCLIInteractiveEditKeepsDefaultsAndUpdatesEnteredFields(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	_, _, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "old.example.com", "--user", "deploy", "--tag", "prod")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}
	input := strings.Join([]string{
		"new.example.com",
		"",
		"",
		"",
		"prod,web",
		"updated notes",
		"",
		"",
		"",
		"",
		"",
	}, "\n")
	stdout, stderr, code := runCLIWithInput(t, input, storePath, "edit", "prod")
	if code != 0 {
		t.Fatalf("interactive edit code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Host [old.example.com]:") || !strings.Contains(stdout, "updated prod") {
		t.Fatalf("interactive edit stdout unexpected: %q", stdout)
	}
	stdout, stderr, code = runCLI(t, storePath, "show", "prod")
	if code != 0 {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{"Host: new.example.com", "User: deploy", "Tags: prod,web", "Notes: updated notes"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("show missing %q: %q", want, stdout)
		}
	}
}

func TestCLIConnectDryRunShorthandAndDoctor(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	_, _, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "example.com", "--user", "deploy")
	if code != 0 {
		t.Fatalf("add code=%d", code)
	}

	stdout, stderr, code := runCLI(t, storePath, "connect", "prod", "--dry-run")
	if code != 0 {
		t.Fatalf("connect dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "ssh deploy@example.com" {
		t.Fatalf("dry-run stdout = %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "prod", "--dry-run")
	if code != 0 {
		t.Fatalf("shorthand dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "ssh deploy@example.com" {
		t.Fatalf("shorthand stdout = %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "doctor")
	if code != 0 {
		t.Fatalf("doctor code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Profiles: 1") || !strings.Contains(stdout, "Store: "+storePath) || !strings.Contains(stdout, "SSH:") {
		t.Fatalf("doctor stdout unexpected: %q", stdout)
	}
}

func TestCLIForwardsAndRemoteCommandRoundTrip(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	stdout, stderr, code := runCLI(t, storePath,
		"add",
		"--name", "tunnel",
		"--host", "example.com",
		"--user", "deploy",
		"--local-forward", "127.0.0.1:15432:db.internal:5432",
		"--remote-forward", "0.0.0.0:18080:localhost:8080",
		"--dynamic-forward", "127.0.0.1:1080",
		"--remote-command", "uptime -p",
	)
	if code != 0 {
		t.Fatalf("add code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	stdout, stderr, code = runCLI(t, storePath, "show", "tunnel")
	if code != 0 {
		t.Fatalf("show code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{
		"LocalForwards: 127.0.0.1:15432:db.internal:5432",
		"RemoteForwards: 0.0.0.0:18080:localhost:8080",
		"DynamicForwards: 127.0.0.1:1080",
		"RemoteCommand: uptime -p",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("show missing %q: %q", want, stdout)
		}
	}

	stdout, stderr, code = runCLI(t, storePath, "connect", "tunnel", "--dry-run")
	if code != 0 {
		t.Fatalf("dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	want := "ssh -L 127.0.0.1:15432:db.internal:5432 -R 0.0.0.0:18080:localhost:8080 -D 127.0.0.1:1080 deploy@example.com 'uptime -p'"
	if strings.TrimSpace(stdout) != want {
		t.Fatalf("dry-run stdout = %q, want %q", strings.TrimSpace(stdout), want)
	}
}

func TestCLIPickListsAndSelectsProfiles(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	for _, args := range [][]string{
		{"add", "--name", "web", "--host", "web.example.com", "--user", "deploy", "--tag", "prod"},
		{"add", "--name", "db", "--host", "db.example.com", "--user", "postgres", "--tag", "prod"},
	} {
		if stdout, stderr, code := runCLI(t, storePath, args...); code != 0 {
			t.Fatalf("%v code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
	}

	stdout, stderr, code := runCLI(t, storePath, "pick", "--tag", "prod")
	if code != 0 {
		t.Fatalf("pick list code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "1\tdb\tdb.example.com") || !strings.Contains(stdout, "2\tweb\tweb.example.com") {
		t.Fatalf("pick list stdout unexpected: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "pick", "--search", "web", "--index", "1", "--dry-run")
	if code != 0 {
		t.Fatalf("pick dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.TrimSpace(stdout) != "ssh deploy@web.example.com" {
		t.Fatalf("pick dry-run stdout = %q", stdout)
	}
}

func TestCLIImportDryRunAndImportDoesNotModifySource(t *testing.T) {
	dir := t.TempDir()
	storePath := dir + "/profiles.json"
	configPath := dir + "/ssh_config"
	original := "Host prod\n  HostName prod.example.com\n  User deploy\n"
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	stdout, stderr, code := runCLI(t, storePath, "import", configPath, "--dry-run")
	if code != 0 {
		t.Fatalf("import dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "prod") || !strings.Contains(stdout, "prod.example.com") {
		t.Fatalf("dry-run stdout unexpected: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "import", configPath)
	if code != 0 {
		t.Fatalf("import code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(after) != original {
		t.Fatalf("source config modified: %q", string(after))
	}
	stdout, _, _ = runCLI(t, storePath, "list")
	if !strings.Contains(stdout, "prod") {
		t.Fatalf("imported profile missing from list: %q", stdout)
	}
}

func TestCLIInteractiveImportPromptsForPath(t *testing.T) {
	dir := t.TempDir()
	storePath := dir + "/profiles.json"
	configPath := dir + "/ssh_config"
	if err := os.WriteFile(configPath, []byte("Host prompted\n  HostName prompted.example.com\n  User ops\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	stdout, stderr, code := runCLIWithInput(t, configPath+"\n", storePath, "import")
	if code != 0 {
		t.Fatalf("interactive import code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "SSH config path:") || !strings.Contains(stdout, "imported 1 profile") {
		t.Fatalf("interactive import stdout unexpected: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "show", "prompted")
	if !strings.Contains(stdout, "Host: prompted.example.com") {
		t.Fatalf("imported profile missing: %q", stdout)
	}
}

func TestCLIInteractiveExportAndBackupPromptForPaths(t *testing.T) {
	dir := t.TempDir()
	storePath := dir + "/profiles.json"
	exportPath := dir + "/export.json"
	backupPath := dir + "/backup.json"
	_, _, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "example.com", "--user", "deploy")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}

	stdout, stderr, code := runCLIWithInput(t, exportPath+"\n", storePath, "export")
	if code != 0 {
		t.Fatalf("interactive export code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Export path:") || !strings.Contains(stdout, "exported 1 profile") {
		t.Fatalf("interactive export stdout unexpected: %q", stdout)
	}
	data, err := os.ReadFile(exportPath)
	if err != nil || !strings.Contains(string(data), "prod") {
		t.Fatalf("export file err=%v data=%q", err, string(data))
	}

	stdout, stderr, code = runCLIWithInput(t, backupPath+"\n", storePath, "backup")
	if code != 0 {
		t.Fatalf("interactive backup code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "Backup path") || !strings.Contains(stdout, "backup written") {
		t.Fatalf("interactive backup stdout unexpected: %q", stdout)
	}
	data, err = os.ReadFile(backupPath)
	if err != nil || !strings.Contains(string(data), "prod") {
		t.Fatalf("backup file err=%v data=%q", err, string(data))
	}
}

func TestCLICompletionPrintsShellScripts(t *testing.T) {
	for _, shell := range []string{"bash", "zsh", "fish"} {
		var stdout, stderr bytes.Buffer
		code := Run([]string{"completion", shell}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("completion %s code=%d stderr=%q", shell, code, stderr.String())
		}
		text := stdout.String()
		if !strings.Contains(text, "sshdex") || !strings.Contains(text, "completion") {
			t.Fatalf("completion %s output unexpected: %q", shell, text)
		}
	}
}

func TestHelpMentionsV01Commands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help code=%d stderr=%q", code, stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{"add", "list", "show", "edit", "delete", "connect", "pick", "import", "export", "backup", "completion", "doctor", "--store PATH"} {
		if !strings.Contains(text, want) {
			t.Fatalf("help missing %q: %s", want, text)
		}
	}
}

func TestDoctorReportsMissingIdentityFile(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	_, stderr, code := runCLI(t, storePath, "add", "--name", "bad", "--host", "example.com", "--identity-file", "/tmp/sshdex-definitely-missing-key")
	if code != 0 {
		t.Fatalf("add code=%d stderr=%q", code, stderr)
	}
	stdout, stderr, code := runCLI(t, storePath, "doctor")
	if code != 0 {
		t.Fatalf("doctor code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "InvalidProfiles: 1") || !strings.Contains(stdout, "missing identity file") {
		t.Fatalf("doctor did not report missing identity file: %q", stdout)
	}
}
