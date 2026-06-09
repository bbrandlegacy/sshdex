package app

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func runCLI(t *testing.T, storePath string, args ...string) (string, string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(argsWithStore(storePath, args...), &stdout, &stderr)
	return stdout.String(), stderr.String(), code
}

func argsWithStore(storePath string, args ...string) []string {
	out := []string{"--store", storePath}
	return append(out, args...)
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

func TestHelpMentionsV01Commands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("help code=%d stderr=%q", code, stderr.String())
	}
	text := stdout.String()
	for _, want := range []string{"add", "list", "show", "edit", "delete", "connect", "import", "doctor", "--store PATH"} {
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
