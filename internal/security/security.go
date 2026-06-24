package security

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const Redacted = "[REDACTED]"

var (
	privateKeyBlockPattern  = regexp.MustCompile(`(?is)-----BEGIN [^-]*PRIVATE KEY-----.*?(?:-----END [^-]*PRIVATE KEY-----|$)`)
	privateKeyMarkerPattern = regexp.MustCompile(`(?i)-----BEGIN [^-]*PRIVATE KEY-----|-----END [^-]*PRIVATE KEY-----`)
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b(password|passwd|passphrase|token|access[_-]?token|refresh[_-]?token|api[_-]?key|secret|vault[_-]?key|sync[_-]?secret|mcp[_-]?response)\b\s*[:=]\s*("[^"]*"|'[^']*'|[^\r\n,;]+)`)
	credentialOnlyPattern   = regexp.MustCompile(`(?i)^\s*(password|passwd|passphrase|token|access[_-]?token|refresh[_-]?token|api[_-]?key|secret|vault[_-]?key|sync[_-]?secret|private[_-]?key|mcp[_-]?response)\s*$`)
	testSentinelPattern     = regexp.MustCompile(`SSHDX_TEST_SECRET[[:alnum:]_\-]*`)
)

// Redact removes protected material from text before it crosses an output or
// error boundary. It intentionally recognizes the fake sentinel used in tests;
// do not use real secrets in tests.
func Redact(text string) string {
	if text == "" {
		return text
	}
	redacted := privateKeyBlockPattern.ReplaceAllString(text, Redacted)
	redacted = privateKeyMarkerPattern.ReplaceAllString(redacted, Redacted)
	redacted = secretAssignmentPattern.ReplaceAllStringFunc(redacted, func(match string) string {
		idx := strings.IndexAny(match, ":=")
		if idx < 0 {
			return Redacted
		}
		return strings.TrimSpace(match[:idx]) + string(match[idx]) + Redacted
	})
	redacted = testSentinelPattern.ReplaceAllString(redacted, Redacted)
	return SanitizeControls(redacted)
}

// SanitizeControls makes redacted text safe to place in a single terminal line.
func SanitizeControls(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, text)
}

// ErrorString returns a redacted error string suitable for stderr.
func ErrorString(err error) string {
	if err == nil {
		return ""
	}
	return Redact(err.Error())
}

// ValidateMetadata rejects protected or output-hostile profile metadata before
// persistence. The returned error names only the field and class of problem, not
// the rejected value.
func ValidateMetadata(field, value string) error {
	if value == "" {
		return nil
	}
	if ContainsControl(value) {
		return fmt.Errorf("%s contains control characters", field)
	}
	if LooksProtected(value) {
		return fmt.Errorf("%s contains protected material; sshdex stores metadata only", field)
	}
	return nil
}

func ContainsControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

// LooksProtected identifies material that must not be stored or printed by
// sshdex metadata flows. It errs on the side of rejecting explicit credential
// assignments, private-key material, exact credential labels, and test sentinels
// without rejecting ordinary prose like "passwordless login" or "secret project".
func LooksProtected(value string) bool {
	if value == "" {
		return false
	}
	if testSentinelPattern.MatchString(value) {
		return true
	}
	if privateKeyMarkerPattern.MatchString(value) {
		return true
	}
	if secretAssignmentPattern.MatchString(value) {
		return true
	}
	return credentialOnlyPattern.MatchString(value)
}

func RejectOptionLike(field, value string) error {
	if strings.HasPrefix(strings.TrimSpace(value), "-") {
		return fmt.Errorf("%s must not start with '-'", field)
	}
	return nil
}
