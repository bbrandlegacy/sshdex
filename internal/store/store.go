package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bbrandlegacy/sshdex/internal/profile"
)

var (
	ErrProfileExists   = errors.New("profile already exists")
	ErrProfileNotFound = errors.New("profile not found")
)

type Store struct {
	path     string
	profiles map[string]profile.Profile
}

type diskFile struct {
	Profiles []profile.Profile `json:"profiles"`
}

func Load(path string) (*Store, error) {
	s := &Store{path: path, profiles: map[string]profile.Profile{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return s, nil
	}
	var disk diskFile
	if err := json.Unmarshal(data, &disk); err != nil {
		return nil, err
	}
	for _, p := range disk.Profiles {
		normalized, err := profile.Validate(p)
		if err != nil {
			return nil, fmt.Errorf("invalid stored profile: %w", err)
		}
		if _, exists := s.profiles[normalized.Name]; exists {
			return nil, fmt.Errorf("duplicate stored profile")
		}
		s.profiles[normalized.Name] = normalized
	}
	return s, nil
}

func (s *Store) Save() error {
	profiles := s.List()
	data, err := json.MarshalIndent(diskFile{Profiles: profiles}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".sshdex-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	return nil
}

type SecurityDiagnostic struct {
	Severity string
	Path     string
	Message  string
	Fix      string
}

func SecurityDiagnostics(path string) []SecurityDiagnostic {
	var diagnostics []SecurityDiagnostic
	dir := filepath.Dir(path)
	if info, err := os.Lstat(dir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			diagnostics = append(diagnostics, SecurityDiagnostic{
				Severity: "warning",
				Path:     dir,
				Message:  "store parent is a symlink",
				Fix:      "use a real directory owned by you with mode 0700",
			})
		} else if info.IsDir() && info.Mode().Perm()&0o077 != 0 {
			diagnostics = append(diagnostics, SecurityDiagnostic{
				Severity: "warning",
				Path:     dir,
				Message:  fmt.Sprintf("store parent permissions are too open: %04o", info.Mode().Perm()),
				Fix:      "chmod 700 " + dir,
			})
		}
	}

	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			diagnostics = append(diagnostics, SecurityDiagnostic{
				Severity: "warning",
				Path:     path,
				Message:  "store path is a symlink",
				Fix:      "replace it with a regular file with mode 0600",
			})
			return diagnostics
		}
		if info.Mode().IsRegular() && info.Mode().Perm()&0o077 != 0 {
			diagnostics = append(diagnostics, SecurityDiagnostic{
				Severity: "warning",
				Path:     path,
				Message:  fmt.Sprintf("store file permissions are too open: %04o", info.Mode().Perm()),
				Fix:      "chmod 600 " + path,
			})
		}
	}
	return diagnostics
}

func (s *Store) Add(p profile.Profile) error {
	normalized, err := profile.Validate(p)
	if err != nil {
		return err
	}
	if _, exists := s.profiles[normalized.Name]; exists {
		return fmt.Errorf("%w", ErrProfileExists)
	}
	s.profiles[normalized.Name] = normalized
	return nil
}

func (s *Store) Update(p profile.Profile) error {
	normalized, err := profile.Validate(p)
	if err != nil {
		return err
	}
	if _, exists := s.profiles[normalized.Name]; !exists {
		return fmt.Errorf("%w", ErrProfileNotFound)
	}
	s.profiles[normalized.Name] = normalized
	return nil
}

func (s *Store) Delete(name string) error {
	name = strings.TrimSpace(name)
	if _, exists := s.profiles[name]; !exists {
		return fmt.Errorf("%w", ErrProfileNotFound)
	}
	delete(s.profiles, name)
	return nil
}

func (s *Store) Get(name string) (profile.Profile, error) {
	name = strings.TrimSpace(name)
	p, exists := s.profiles[name]
	if !exists {
		return profile.Profile{}, fmt.Errorf("%w", ErrProfileNotFound)
	}
	return p, nil
}

func (s *Store) List() []profile.Profile {
	names := make([]string, 0, len(s.profiles))
	for name := range s.profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]profile.Profile, 0, len(names))
	for _, name := range names {
		out = append(out, s.profiles[name])
	}
	return out
}

func (s *Store) Path() string { return s.path }
