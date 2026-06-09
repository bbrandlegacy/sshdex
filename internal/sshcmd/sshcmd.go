package sshcmd

import (
	"strconv"
	"strings"

	"github.com/bbrandlegacy/sshdex/internal/profile"
)

func BuildArgs(p profile.Profile) ([]string, error) {
	normalized, err := profile.Validate(p)
	if err != nil {
		return nil, err
	}
	args := []string{}
	if normalized.Port != 22 {
		args = append(args, "-p", strconv.Itoa(normalized.Port))
	}
	if normalized.IdentityFile != "" {
		args = append(args, "-i", normalized.IdentityFile)
	}
	if normalized.ProxyJump != "" {
		args = append(args, "-J", normalized.ProxyJump)
	}
	for _, forward := range normalized.LocalForwards {
		args = append(args, "-L", forward)
	}
	for _, forward := range normalized.RemoteForwards {
		args = append(args, "-R", forward)
	}
	for _, forward := range normalized.DynamicForwards {
		args = append(args, "-D", forward)
	}
	target := normalized.Host
	if normalized.User != "" {
		target = normalized.User + "@" + normalized.Host
	}
	args = append(args, target)
	if normalized.RemoteCommand != "" {
		args = append(args, normalized.RemoteCommand)
	}
	return args, nil
}

func Preview(p profile.Profile) (string, error) {
	args, err := BuildArgs(p)
	if err != nil {
		return "", err
	}
	rendered := make([]string, 0, len(args)+1)
	rendered = append(rendered, "ssh")
	for _, arg := range args {
		rendered = append(rendered, shellQuote(arg))
	}
	return strings.Join(rendered, " "), nil
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && !strings.ContainsRune("@%_+=:,./-", r) {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
