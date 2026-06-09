package store

import (
	"errors"
	"path/filepath"
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
