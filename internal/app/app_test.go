package app

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
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

func TestCLIRejectsMalformedForwardBeforePersistence(t *testing.T) {
	storePath := t.TempDir() + "/profiles.json"
	stdout, stderr, code := runCLI(t, storePath,
		"add",
		"--name", "bad-tunnel",
		"--host", "example.com",
		"--local-forward", "127.0.0.1:port:db.internal:5432",
	)
	if code == 0 {
		t.Fatalf("add malformed forward code=0 stdout=%q stderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, "local_forward listen port") {
		t.Fatalf("malformed forward error unexpected stdout=%q stderr=%q", stdout, stderr)
	}

	stdout, stderr, code = runCLI(t, storePath, "list")
	if code != 0 {
		t.Fatalf("list after rejected add code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.Contains(stdout, "bad-tunnel") {
		t.Fatalf("malformed forward profile was persisted: %q", stdout)
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

func TestCLIImportSSHDexJSONRestoresExport(t *testing.T) {
	dir := t.TempDir()
	sourceStore := filepath.Join(dir, "source.json")
	exportPath := filepath.Join(dir, "export.json")
	restoredStore := filepath.Join(dir, "restored.json")

	_, _, code := runCLI(t, sourceStore, "add", "--name", "prod", "--host", "prod.example.com", "--user", "deploy", "--tag", "prod")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}
	stdout, stderr, code := runCLI(t, sourceStore, "export", exportPath)
	if code != 0 {
		t.Fatalf("export code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	stdout, stderr, code = runCLI(t, restoredStore, "import", "--format", "sshdex", exportPath)
	if code != 0 {
		t.Fatalf("json import code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "imported 1 profile") {
		t.Fatalf("json import stdout unexpected: %q", stdout)
	}
	stdout, stderr, code = runCLI(t, restoredStore, "show", "prod")
	if code != 0 {
		t.Fatalf("show restored code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{"Host: prod.example.com", "User: deploy", "Tags: prod"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("restored profile missing %q: %q", want, stdout)
		}
	}
}

func TestCLIImportSSHDexJSONDefaultSkipIsNonDestructiveAndDryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.json")
	importPath := filepath.Join(dir, "import.json")
	if err := os.WriteFile(importPath, []byte(`{"profiles":[{"name":"prod","host":"new.example.com","user":"new"},{"name":"stage","host":"stage.example.com","user":"ops"}]}`), 0o600); err != nil {
		t.Fatalf("write import: %v", err)
	}
	_, _, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "old.example.com", "--user", "old")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}

	stdout, stderr, code := runCLI(t, storePath, "import", importPath, "--format=sshdex", "--dry-run")
	if code != 0 {
		t.Fatalf("json dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "prod\tnew.example.com\tnew\tskip") || !strings.Contains(stdout, "stage\tstage.example.com\tops\timport") {
		t.Fatalf("dry-run plan unexpected: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "list")
	if strings.Contains(stdout, "stage") {
		t.Fatalf("dry-run wrote stage profile: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "import", "--format", "sshdex", importPath)
	if code != 0 {
		t.Fatalf("json import skip code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "imported 1 profile") || !strings.Contains(stdout, "skipped 1 duplicate") {
		t.Fatalf("skip import stdout unexpected: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "show", "prod")
	if !strings.Contains(stdout, "Host: old.example.com") || strings.Contains(stdout, "new.example.com") {
		t.Fatalf("default skip replaced existing profile: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "show", "stage")
	if !strings.Contains(stdout, "Host: stage.example.com") {
		t.Fatalf("new profile not imported: %q", stdout)
	}
}

func TestCLIImportSSHDexJSONConflictReplace(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.json")
	importPath := filepath.Join(dir, "replace.json")
	if err := os.WriteFile(importPath, []byte(`{"profiles":[{"name":"prod","host":"new.example.com","user":"deploy","port":2200}]}`), 0o600); err != nil {
		t.Fatalf("write import: %v", err)
	}
	_, _, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "old.example.com", "--user", "old")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}

	stdout, stderr, code := runCLI(t, storePath, "import", "--format", "sshdex", "--conflict", "replace", importPath)
	if code != 0 {
		t.Fatalf("json import replace code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "replaced 1") {
		t.Fatalf("replace summary unexpected: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "show", "prod")
	for _, want := range []string{"Host: new.example.com", "User: deploy", "Port: 2200"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("replace missing %q: %q", want, stdout)
		}
	}
}

func TestCLIImportSSHDexJSONConflictRename(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.json")
	importPath := filepath.Join(dir, "rename.json")
	if err := os.WriteFile(importPath, []byte(`{"profiles":[{"name":"prod","host":"new.example.com","user":"deploy"}]}`), 0o600); err != nil {
		t.Fatalf("write import: %v", err)
	}
	for _, args := range [][]string{
		{"add", "--name", "prod", "--host", "old.example.com"},
		{"add", "--name", "prod-1", "--host", "existing.example.com"},
	} {
		if _, _, code := runCLI(t, storePath, args...); code != 0 {
			t.Fatalf("seed %v code=%d", args, code)
		}
	}

	stdout, stderr, code := runCLI(t, storePath, "import", "--format", "sshdex", "--conflict", "rename", "--dry-run", importPath)
	if code != 0 {
		t.Fatalf("json import rename dry-run code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "prod-2	new.example.com	deploy	rename:prod") || !strings.Contains(stdout, "rename 1") {
		t.Fatalf("rename dry-run unexpected: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "list")
	if strings.Contains(stdout, "prod-2") {
		t.Fatalf("rename dry-run wrote profile: %q", stdout)
	}

	stdout, stderr, code = runCLI(t, storePath, "import", "--format", "sshdex", "--conflict=rename", importPath)
	if code != 0 {
		t.Fatalf("json import rename code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "renamed 1") {
		t.Fatalf("rename summary unexpected: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "show", "prod-2")
	if !strings.Contains(stdout, "Host: new.example.com") || !strings.Contains(stdout, "User: deploy") {
		t.Fatalf("renamed profile missing: %q", stdout)
	}
	stdout, _, _ = runCLI(t, storePath, "show", "prod")
	if !strings.Contains(stdout, "Host: old.example.com") {
		t.Fatalf("rename modified original profile: %q", stdout)
	}
}

func TestCLIImportSSHDexJSONRejectsInvalidFlagsAndData(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.json")
	badJSONPath := filepath.Join(dir, "bad.json")
	invalidProfilePath := filepath.Join(dir, "invalid-profile.json")
	if err := os.WriteFile(badJSONPath, []byte(`{"profiles":[`), 0o600); err != nil {
		t.Fatalf("write malformed import: %v", err)
	}
	if err := os.WriteFile(invalidProfilePath, []byte(`{"profiles":[{"name":"bad","host":"example.com","notes":"SSHDX_TEST_SECRET_DO_NOT_PRINT_12345"}]}`), 0o600); err != nil {
		t.Fatalf("write invalid profile import: %v", err)
	}
	cases := []struct {
		name       string
		args       []string
		wantErr    string
		forbidText string
	}{
		{name: "unsupported format", args: []string{"import", "--format", "yaml", badJSONPath}, wantErr: "unsupported import format yaml"},
		{name: "unsupported conflict", args: []string{"import", "--format", "sshdex", "--conflict", "merge", badJSONPath}, wantErr: "unsupported conflict policy merge"},
		{name: "missing format value", args: []string{"import", "--format"}, wantErr: "--format requires openssh or sshdex"},
		{name: "missing conflict value", args: []string{"import", "--conflict"}, wantErr: "--conflict requires skip, replace, or rename"},
		{name: "malformed json", args: []string{"import", "--format", "sshdex", badJSONPath}, wantErr: "unexpected end"},
		{name: "invalid profile redacts", args: []string{"import", "--format", "sshdex", invalidProfilePath}, wantErr: "protected material", forbidText: "SSHDX_TEST_SECRET_DO_NOT_PRINT_12345"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runCLI(t, storePath, tc.args...)
			if code == 0 {
				t.Fatalf("%s code=0 stdout=%q stderr=%q", tc.name, stdout, stderr)
			}
			combined := stdout + stderr
			if !strings.Contains(combined, tc.wantErr) {
				t.Fatalf("%s missing %q in stdout=%q stderr=%q", tc.name, tc.wantErr, stdout, stderr)
			}
			if tc.forbidText != "" && strings.Contains(combined, tc.forbidText) {
				t.Fatalf("%s leaked forbidden text in stdout=%q stderr=%q", tc.name, stdout, stderr)
			}
		})
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

func TestCLIDoesNotLeakProtectedSentinelInOutputsOrErrors(t *testing.T) {
	const sentinel = "SSHDX_TEST_SECRET_DO_NOT_PRINT_12345"
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.json")
	unsafeStore := `{"profiles":[{"name":"prod","host":"example.com","notes":"` + sentinel + `"}]}`
	if err := os.WriteFile(storePath, []byte(unsafeStore), 0o600); err != nil {
		t.Fatalf("write unsafe store: %v", err)
	}
	commands := [][]string{
		{"list"},
		{"show", "prod"},
		{"connect", "prod", "--dry-run"},
		{"export", filepath.Join(dir, "export.json")},
		{"backup", filepath.Join(dir, "backup.json")},
		{"doctor"},
	}
	for _, args := range commands {
		stdout, stderr, _ := runCLI(t, storePath, args...)
		assertNoLeak(t, sentinel, stdout, stderr)
	}

	stdout, stderr, _ := runCLI(t, filepath.Join(dir, "clean.json"), "add", "--name", "bad", "--host", "example.com", "--notes", sentinel)
	assertNoLeak(t, sentinel, stdout, stderr)

	stdout, stderr, _ = runCLI(t, filepath.Join(dir, "clean.json"), "completion", sentinel)
	assertNoLeak(t, sentinel, stdout, stderr)

	_, _, code := runCLI(t, filepath.Join(dir, "clean.json"), "add", "--name", "prod", "--host", "example.com")
	if code != 0 {
		t.Fatalf("seed valid profile code=%d", code)
	}
	for _, args := range [][]string{
		{"export", filepath.Join(dir, sentinel+"-export.json")},
		{"backup", filepath.Join(dir, sentinel+"-backup.json")},
		{"doctor"},
	} {
		stdout, stderr, _ = runCLI(t, filepath.Join(dir, "clean.json"), args...)
		assertNoLeak(t, sentinel, stdout, stderr)
	}
}

func TestDoctorReportsUnsafeStorePermissionsWithoutProfileContents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode assertions are not portable on Windows")
	}
	dir := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	storePath := filepath.Join(dir, "profiles.json")
	_, _, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "sensitive-host.example", "--identity-file", "/tmp/sshdex-sensitive-missing-key")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}
	if err := os.Chmod(storePath, 0o644); err != nil {
		t.Fatalf("chmod store: %v", err)
	}

	stdout, stderr, code := runCLI(t, storePath, "doctor")
	if code != 0 {
		t.Fatalf("doctor code=%d stderr=%q", code, stderr)
	}
	for _, want := range []string{"StoreSecurityFindings:", "store parent permissions are too open", "store file permissions are too open", "chmod 700", "chmod 600"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("doctor missing %q: %q", want, stdout)
		}
	}
	for _, leaked := range []string{"sensitive-host.example", "prod", "/tmp/sshdex-sensitive-missing-key"} {
		if strings.Contains(stdout, leaked) {
			t.Fatalf("doctor leaked stored profile content %q while reporting permissions: %q", leaked, stdout)
		}
	}
}

func TestDoctorReportsStoreSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup is not portable on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	_, _, code := runCLI(t, target, "add", "--name", "prod", "--host", "example.com")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}
	link := filepath.Join(dir, "profiles-link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	stdout, stderr, code := runCLI(t, link, "doctor")
	if code != 0 {
		t.Fatalf("doctor symlink code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "store path is a symlink") || !strings.Contains(stdout, "replace it with a regular file") {
		t.Fatalf("doctor symlink output unexpected: %q", stdout)
	}
}

func TestExportAndBackupCreatePrivateFilesAndDirs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode assertions are not portable on Windows")
	}
	dir := t.TempDir()
	storePath := filepath.Join(dir, "profiles.json")
	_, _, code := runCLI(t, storePath, "add", "--name", "prod", "--host", "example.com", "--user", "deploy")
	if code != 0 {
		t.Fatalf("seed add code=%d", code)
	}
	for _, dest := range []string{filepath.Join(dir, "nested-export", "profiles.json"), filepath.Join(dir, "nested-backup", "profiles.json")} {
		cmd := "export"
		if strings.Contains(dest, "backup") {
			cmd = "backup"
		}
		stdout, stderr, code := runCLI(t, storePath, cmd, dest)
		if code != 0 {
			t.Fatalf("%s code=%d stdout=%q stderr=%q", cmd, code, stdout, stderr)
		}
		info, err := os.Stat(dest)
		if err != nil {
			t.Fatalf("stat %s: %v", dest, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Fatalf("%s mode = %04o, want 0600", dest, got)
		}
		dirInfo, err := os.Stat(filepath.Dir(dest))
		if err != nil {
			t.Fatalf("stat dir %s: %v", filepath.Dir(dest), err)
		}
		if got := dirInfo.Mode().Perm(); got != 0o700 {
			t.Fatalf("%s dir mode = %04o, want 0700", dest, got)
		}
	}
}

func assertNoLeak(t *testing.T, sentinel, stdout, stderr string) {
	t.Helper()
	combined := stdout + stderr
	if strings.Contains(combined, sentinel) {
		t.Fatalf("output leaked sentinel %q\nstdout=%q\nstderr=%q", sentinel, stdout, stderr)
	}
}

func TestDoctorReportsMissingIdentityFile(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatalf("chmod temp dir: %v", err)
		}
	}
	storePath := dir + "/profiles.json"
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
