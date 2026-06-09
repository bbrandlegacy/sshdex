package sshconfig

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bbrandlegacy/sshdex/internal/profile"
)

func ParseString(input string) ([]profile.Profile, error) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	blocks := []block{}
	var current *block
	for scanner.Scan() {
		tokens, err := parseTokens(scanner.Text())
		if err != nil {
			return nil, err
		}
		if len(tokens) == 0 {
			continue
		}
		key := strings.ToLower(tokens[0])
		switch key {
		case "host":
			b := block{aliases: tokens[1:]}
			blocks = append(blocks, b)
			current = &blocks[len(blocks)-1]
		default:
			if current == nil || len(tokens) < 2 {
				continue
			}
			value := strings.Join(tokens[1:], " ")
			switch key {
			case "hostname":
				current.hostName = value
			case "user":
				current.user = value
			case "port":
				port, err := strconv.Atoi(value)
				if err != nil {
					return nil, fmt.Errorf("invalid port %q", value)
				}
				current.port = port
			case "identityfile":
				current.identityFile = value
			case "proxyjump":
				current.proxyJump = value
			case "localforward":
				current.localForwards = append(current.localForwards, normalizeForwardDirective(tokens[1:]))
			case "remoteforward":
				current.remoteForwards = append(current.remoteForwards, normalizeForwardDirective(tokens[1:]))
			case "dynamicforward":
				current.dynamicForwards = append(current.dynamicForwards, normalizeForwardDirective(tokens[1:]))
			case "remotecommand":
				current.remoteCommand = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	out := []profile.Profile{}
	now := time.Now().UTC()
	for _, b := range blocks {
		if !b.importable() {
			continue
		}
		for _, alias := range b.aliases {
			if wildcard(alias) {
				continue
			}
			host := b.hostName
			if host == "" {
				host = alias
			}
			p, err := profile.Validate(profile.Profile{Name: alias, Host: host, User: b.user, Port: b.port, IdentityFile: b.identityFile, ProxyJump: b.proxyJump, LocalForwards: b.localForwards, RemoteForwards: b.remoteForwards, DynamicForwards: b.dynamicForwards, RemoteCommand: b.remoteCommand, CreatedAt: now, UpdatedAt: now})
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		}
	}
	return out, nil
}

type block struct {
	aliases                                 []string
	hostName, user, identityFile, proxyJump string
	localForwards                           []string
	remoteForwards                          []string
	dynamicForwards                         []string
	remoteCommand                           string
	port                                    int
}

func (b block) importable() bool {
	for _, a := range b.aliases {
		if !wildcard(a) {
			return true
		}
	}
	return false
}

func wildcard(s string) bool { return strings.ContainsAny(s, "*?") || strings.HasPrefix(s, "!") }

func normalizeForwardDirective(parts []string) string {
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 2 {
		return cleaned[0] + ":" + cleaned[1]
	}
	return strings.Join(cleaned, " ")
}

func parseTokens(line string) ([]string, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, nil
	}
	var tokens []string
	var b strings.Builder
	inQuote := false
	quote := rune(0)
	escaped := false
	for _, r := range line {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if inQuote {
			if r == quote {
				inQuote = false
				continue
			}
			b.WriteRune(r)
			continue
		}
		if r == '#' {
			break
		}
		if r == '\'' || r == '"' {
			inQuote = true
			quote = r
			continue
		}
		if r == ' ' || r == '\t' {
			if b.Len() > 0 {
				tokens = append(tokens, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	if inQuote {
		return nil, fmt.Errorf("unterminated quote in ssh config line")
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens, nil
}
