package profile

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Profile describes a saved SSH target. v0.1 stores metadata only: no
// passwords, passphrases, or private key contents belong in this struct.
type Profile struct {
	Name            string     `json:"name"`
	Host            string     `json:"host"`
	User            string     `json:"user,omitempty"`
	Port            int        `json:"port"`
	IdentityFile    string     `json:"identity_file,omitempty"`
	Tags            []string   `json:"tags,omitempty"`
	Notes           string     `json:"notes,omitempty"`
	ProxyJump       string     `json:"proxy_jump,omitempty"`
	LocalForwards   []string   `json:"local_forwards,omitempty"`
	RemoteForwards  []string   `json:"remote_forwards,omitempty"`
	DynamicForwards []string   `json:"dynamic_forwards,omitempty"`
	RemoteCommand   string     `json:"remote_command,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	LastConnectedAt *time.Time `json:"last_connected_at,omitempty"`
	ConnectionCount int        `json:"connection_count"`
}

var (
	ErrMissingName = errors.New("profile name is required")
	ErrMissingHost = errors.New("profile host is required")
)

// Validate normalizes a profile and returns an error when required fields are
// missing or unsafe metadata is malformed.
func Validate(p Profile) (Profile, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Host = strings.TrimSpace(p.Host)
	p.User = strings.TrimSpace(p.User)
	p.IdentityFile = strings.TrimSpace(p.IdentityFile)
	p.Notes = strings.TrimSpace(p.Notes)
	p.ProxyJump = strings.TrimSpace(p.ProxyJump)
	p.RemoteCommand = strings.TrimSpace(p.RemoteCommand)

	if p.Name == "" {
		return Profile{}, ErrMissingName
	}
	if p.Host == "" {
		return Profile{}, ErrMissingHost
	}
	if strings.HasPrefix(p.Host, "-") {
		return Profile{}, fmt.Errorf("host must not start with '-'")
	}
	if strings.HasPrefix(p.User, "-") {
		return Profile{}, fmt.Errorf("user must not start with '-'")
	}
	for field, value := range map[string]string{"name": p.Name, "host": p.Host, "user": p.User, "identity_file": p.IdentityFile, "notes": p.Notes, "proxy_jump": p.ProxyJump, "remote_command": p.RemoteCommand} {
		if err := rejectUnsafeMetadata(field, value); err != nil {
			return Profile{}, err
		}
	}
	var err error
	if p.LocalForwards, err = normalizeForwardList("local_forward", p.LocalForwards); err != nil {
		return Profile{}, err
	}
	if p.RemoteForwards, err = normalizeForwardList("remote_forward", p.RemoteForwards); err != nil {
		return Profile{}, err
	}
	if p.DynamicForwards, err = normalizeForwardList("dynamic_forward", p.DynamicForwards); err != nil {
		return Profile{}, err
	}
	if p.Port == 0 {
		p.Port = 22
	}
	if p.Port < 1 || p.Port > 65535 {
		return Profile{}, fmt.Errorf("invalid port %d: must be between 1 and 65535", p.Port)
	}

	tags, err := normalizeTags(p.Tags)
	if err != nil {
		return Profile{}, err
	}
	p.Tags = tags

	return p, nil
}

func normalizeTags(tags []string) ([]string, error) {
	out := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			continue
		}
		if err := rejectUnsafeMetadata("tag", normalized); err != nil {
			return nil, err
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate tag %q", normalized)
		}
		seen[key] = struct{}{}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeForwardList(field string, values []string) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		normalized := strings.TrimSpace(value)
		if normalized == "" {
			continue
		}
		if strings.HasPrefix(normalized, "-") {
			return nil, fmt.Errorf("%s must not start with '-'", field)
		}
		if err := rejectUnsafeMetadata(field, normalized); err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

func rejectUnsafeMetadata(field, value string) error {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%s contains control characters", field)
		}
	}
	upper := strings.ToUpper(value)
	if strings.Contains(upper, "-----BEGIN") && strings.Contains(upper, "PRIVATE KEY-----") {
		return fmt.Errorf("%s appears to contain private key material", field)
	}
	return nil
}
