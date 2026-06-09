package store

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bbrandlegacy/sshdex/internal/profile"
)

func TestLoadMissingFileReturnsEmptyStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if got := s.List(); len(got) != 0 {
		t.Fatalf("List() len = %d, want 0", len(got))
	}
}

func TestSaveLoadRoundTripDeterministicList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profiles.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if err := s.Add(profile.Profile{
		Name:            "zeta",
		Host:            "z.example",
		Tags:            []string{"prod"},
		LocalForwards:   []string{"127.0.0.1:15432:db.internal:5432"},
		RemoteForwards:  []string{"0.0.0.0:18080:localhost:8080"},
		DynamicForwards: []string{"127.0.0.1:1080"},
		RemoteCommand:   "uptime -p",
	}); err != nil {
		t.Fatalf("Add(zeta) error: %v", err)
	}
	if err := s.Add(profile.Profile{Name: "alpha", Host: "a.example", Port: 2200}); err != nil {
		t.Fatalf("Add(alpha) error: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load(saved) error: %v", err)
	}
	got := loaded.List()
	if len(got) != 2 {
		t.Fatalf("List() len = %d, want 2", len(got))
	}
	if got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Fatalf("List() order = %q, %q; want alpha, zeta", got[0].Name, got[1].Name)
	}
	if got[0].Port != 2200 || got[1].Port != 22 {
		t.Fatalf("ports not preserved/defaulted: %#v", got)
	}
	if got[1].LocalForwards[0] != "127.0.0.1:15432:db.internal:5432" || got[1].RemoteForwards[0] != "0.0.0.0:18080:localhost:8080" || got[1].DynamicForwards[0] != "127.0.0.1:1080" || got[1].RemoteCommand != "uptime -p" {
		t.Fatalf("forwarding fields not preserved: %#v", got[1])
	}
}

func TestAddRejectsDuplicateProfileName(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := s.Add(profile.Profile{Name: "prod", Host: "one.example"}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	err = s.Add(profile.Profile{Name: " prod ", Host: "two.example"})
	if !errors.Is(err, ErrProfileExists) {
		t.Fatalf("Add duplicate error = %v, want ErrProfileExists", err)
	}
}

func TestUpdateAndDeleteMissingReturnErrors(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "profiles.json"))
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := s.Update(profile.Profile{Name: "missing", Host: "example.com"}); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Update missing error = %v, want ErrProfileNotFound", err)
	}
	if err := s.Delete("missing"); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Delete missing error = %v, want ErrProfileNotFound", err)
	}
}

func TestSaveCreatesPrivateFileAndDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode assertions are not portable on Windows")
	}
	path := filepath.Join(t.TempDir(), "nested", "profiles.json")
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if err := s.Add(profile.Profile{Name: "prod", Host: "example.com"}); err != nil {
		t.Fatalf("Add() error: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat saved file: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("saved file mode = %04o, want 0600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat saved dir: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("saved dir mode = %04o, want 0700", got)
	}
}

func TestSecurityDiagnosticsReportsTooOpenStoreAndParent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode assertions are not portable on Windows")
	}
	dir := filepath.Join(t.TempDir(), "open")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "profiles.json")
	if err := os.WriteFile(path, []byte(`{"profiles":[]}`), 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
	diagnostics := SecurityDiagnostics(path)
	text := diagnosticsText(diagnostics)
	for _, want := range []string{"parent permissions are too open", "file permissions are too open", "chmod 700", "chmod 600"} {
		if !strings.Contains(text, want) {
			t.Fatalf("diagnostics missing %q: %#v", want, diagnostics)
		}
	}
}

func TestSecurityDiagnosticsReportsStoreSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink setup is not portable on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte(`{"profiles":[]}`), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(dir, "profiles.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	diagnostics := SecurityDiagnostics(link)
	text := diagnosticsText(diagnostics)
	if !strings.Contains(text, "store path is a symlink") || strings.Contains(text, "profiles") {
		t.Fatalf("symlink diagnostics unexpected: %#v", diagnostics)
	}
}

func diagnosticsText(diagnostics []SecurityDiagnostic) string {
	var b strings.Builder
	for _, d := range diagnostics {
		b.WriteString(d.Message)
		b.WriteByte('\n')
		b.WriteString(d.Fix)
		b.WriteByte('\n')
	}
	return b.String()
}
