package profile

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/bbrandlegacy/sshdex/internal/security"
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
	for field, value := range map[string]string{"name": p.Name, "host": p.Host, "user": p.User, "identity_file": p.IdentityFile, "proxy_jump": p.ProxyJump, "remote_command": p.RemoteCommand} {
		if err := security.RejectOptionLike(field, value); err != nil {
			return Profile{}, err
		}
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
		if err := security.RejectOptionLike("tag", normalized); err != nil {
			return nil, err
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
		if err := security.RejectOptionLike(field, normalized); err != nil {
			return nil, err
		}
		if err := rejectUnsafeMetadata(field, normalized); err != nil {
			return nil, err
		}
		if err := validateForwardSyntax(field, normalized); err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

func validateForwardSyntax(field, value string) error {
	switch field {
	case "local_forward", "remote_forward":
		return validateLocalOrRemoteForward(field, value)
	case "dynamic_forward":
		return validateDynamicForward(field, value)
	default:
		return nil
	}
}

func validateLocalOrRemoteForward(field, value string) error {
	parts, err := splitForwardFields(value)
	if err != nil {
		return fmt.Errorf("%s has invalid syntax; want [bind_address:]port:host:hostport", field)
	}

	allowZeroListen := field == "remote_forward"
	switch len(parts) {
	case 2:
		listenPort, socketPath := parts[0], parts[1]
		if err := validateForwardPortRange(field, "listen", listenPort, allowZeroListen); err != nil {
			return err
		}
		return validateForwardSocketPath(field, "target socket", socketPath)
	case 3:
		if looksLikeSocketPath(parts[0]) {
			if err := validateForwardSocketPath(field, "listen socket", parts[0]); err != nil {
				return err
			}
			if err := validateForwardEndpoint(field, "host", parts[1], true); err != nil {
				return err
			}
			return validateForwardPort(field, "host", parts[2])
		}
		listenPort, host, hostPort := parts[0], parts[1], parts[2]
		if err := validateForwardPortRange(field, "listen", listenPort, allowZeroListen); err != nil {
			return err
		}
		if err := validateForwardEndpoint(field, "host", host, true); err != nil {
			return err
		}
		return validateForwardPort(field, "host", hostPort)
	case 4:
		bind, listenPort, host, hostPort := parts[0], parts[1], parts[2], parts[3]
		if err := validateForwardEndpoint(field, "bind address", bind, false); err != nil {
			return err
		}
		if err := validateForwardPortRange(field, "listen", listenPort, allowZeroListen); err != nil {
			return err
		}
		if err := validateForwardEndpoint(field, "host", host, true); err != nil {
			return err
		}
		return validateForwardPort(field, "host", hostPort)
	default:
		return fmt.Errorf("%s has invalid syntax; want [bind_address:]port:host:hostport", field)
	}
}

func validateDynamicForward(field, value string) error {
	parts, err := splitForwardFields(value)
	if err != nil {
		return fmt.Errorf("%s has invalid syntax; want [bind_address:]port", field)
	}

	var bind, listenPort string
	switch len(parts) {
	case 1:
		listenPort = parts[0]
	case 2:
		bind, listenPort = parts[0], parts[1]
		if err := validateForwardEndpoint(field, "bind address", bind, false); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s has invalid syntax; want [bind_address:]port", field)
	}
	return validateForwardPort(field, "listen", listenPort)
}

func splitForwardFields(value string) ([]string, error) {
	fields := []string{}
	start := 0
	inBracket := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case '[':
			if inBracket {
				return nil, fmt.Errorf("nested bracket")
			}
			inBracket = true
		case ']':
			if !inBracket {
				return nil, fmt.Errorf("unmatched bracket")
			}
			inBracket = false
		case ':':
			if !inBracket {
				fields = append(fields, value[start:i])
				start = i + 1
			}
		}
	}
	if inBracket {
		return nil, fmt.Errorf("unmatched bracket")
	}
	fields = append(fields, value[start:])
	return fields, nil
}

func validateForwardPort(field, role, value string) error {
	return validateForwardPortRange(field, role, value, false)
}

func validateForwardPortRange(field, role, value string, allowZero bool) error {
	if value == "" {
		return fmt.Errorf("%s %s port is required", field, role)
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return fmt.Errorf("%s %s port must be numeric and between %d and 65535", field, role, minForwardPort(allowZero))
		}
	}
	port, err := strconv.Atoi(value)
	if err != nil || port < minForwardPort(allowZero) || port > 65535 {
		return fmt.Errorf("%s %s port must be numeric and between %d and 65535", field, role, minForwardPort(allowZero))
	}
	return nil
}

func minForwardPort(allowZero bool) int {
	if allowZero {
		return 0
	}
	return 1
}

func looksLikeSocketPath(value string) bool {
	return strings.HasPrefix(value, "/")
}

func validateForwardSocketPath(field, role, value string) error {
	if value == "" {
		return fmt.Errorf("%s %s is required", field, role)
	}
	if hasWhitespace(value) {
		return fmt.Errorf("%s %s must not contain whitespace", field, role)
	}
	if !looksLikeSocketPath(value) {
		return fmt.Errorf("%s %s must be an absolute path", field, role)
	}
	return nil
}

func validateForwardEndpoint(field, role, value string, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s %s is required", field, role)
		}
		return nil
	}
	if hasWhitespace(value) {
		return fmt.Errorf("%s %s must not contain whitespace", field, role)
	}
	if strings.HasPrefix(value, "[") || strings.HasSuffix(value, "]") {
		if len(value) < 3 || !strings.HasPrefix(value, "[") || !strings.HasSuffix(value, "]") || value[1:len(value)-1] == "" {
			return fmt.Errorf("%s %s has invalid bracket syntax", field, role)
		}
	}
	return nil
}

func hasWhitespace(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

func rejectUnsafeMetadata(field, value string) error {
	return security.ValidateMetadata(field, value)
}
