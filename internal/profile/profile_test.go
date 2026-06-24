package profile

import (
	"strings"
	"testing"
	"time"
)

const fakeProtectedSentinel = "SSHDX_TEST_SECRET_DO_NOT_PRINT_12345"

func TestValidateNormalizesValidProfile(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	p := Profile{
		Name:            "  prod-web-01  ",
		Host:            "  192.0.2.10  ",
		User:            " deploy ",
		Port:            0,
		IdentityFile:    " ~/.ssh/id_ed25519 ",
		Tags:            []string{" prod ", " web "},
		Notes:           " production host ",
		ProxyJump:       " bastion ",
		LocalForwards:   []string{" 127.0.0.1:15432:db.internal:5432 "},
		RemoteForwards:  []string{" 0.0.0.0:18080:localhost:8080 "},
		DynamicForwards: []string{" 127.0.0.1:1080 "},
		RemoteCommand:   " uptime ",
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	got, err := Validate(p)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}

	if got.Name != "prod-web-01" {
		t.Fatalf("Name not normalized: %q", got.Name)
	}
	if got.Host != "192.0.2.10" {
		t.Fatalf("Host not normalized: %q", got.Host)
	}
	if got.User != "deploy" {
		t.Fatalf("User not normalized: %q", got.User)
	}
	if got.Port != 22 {
		t.Fatalf("Port default = %d, want 22", got.Port)
	}
	if got.IdentityFile != "~/.ssh/id_ed25519" {
		t.Fatalf("IdentityFile not normalized: %q", got.IdentityFile)
	}
	if got.ProxyJump != "bastion" {
		t.Fatalf("ProxyJump not normalized: %q", got.ProxyJump)
	}
	if got.LocalForwards[0] != "127.0.0.1:15432:db.internal:5432" {
		t.Fatalf("LocalForwards not normalized: %#v", got.LocalForwards)
	}
	if got.RemoteForwards[0] != "0.0.0.0:18080:localhost:8080" {
		t.Fatalf("RemoteForwards not normalized: %#v", got.RemoteForwards)
	}
	if got.DynamicForwards[0] != "127.0.0.1:1080" {
		t.Fatalf("DynamicForwards not normalized: %#v", got.DynamicForwards)
	}
	if got.RemoteCommand != "uptime" {
		t.Fatalf("RemoteCommand not normalized: %q", got.RemoteCommand)
	}
	if len(got.Tags) != 2 || got.Tags[0] != "prod" || got.Tags[1] != "web" {
		t.Fatalf("Tags not normalized: %#v", got.Tags)
	}
}

func TestValidateRejectsMissingName(t *testing.T) {
	_, err := Validate(Profile{Host: "example.com", Port: 22})
	if err == nil {
		t.Fatal("Validate() nil error, want missing name error")
	}
}

func TestValidateRejectsMissingHost(t *testing.T) {
	_, err := Validate(Profile{Name: "prod", Port: 22})
	if err == nil {
		t.Fatal("Validate() nil error, want missing host error")
	}
}

func TestValidateRejectsInvalidPort(t *testing.T) {
	badPorts := []int{-1, 65536}
	for _, port := range badPorts {
		_, err := Validate(Profile{Name: "prod", Host: "example.com", Port: port})
		if err == nil {
			t.Fatalf("Validate() nil error for port %d, want invalid port error", port)
		}
	}
}

func TestValidateRejectsDuplicateTagsAfterNormalization(t *testing.T) {
	_, err := Validate(Profile{Name: "prod", Host: "example.com", Tags: []string{"prod", " prod "}})
	if err == nil {
		t.Fatal("Validate() nil error, want duplicate tag error")
	}
}

func TestValidateRejectsOptionLikeHostAndUser(t *testing.T) {
	cases := []Profile{
		{Name: "-badname", Host: "example.com"},
		{Name: "badhost", Host: "-oProxyCommand=touch/tmp/pwn"},
		{Name: "baduser", Host: "example.com", User: "-lroot"},
		{Name: "badidentity", Host: "example.com", IdentityFile: "-oProxyCommand=touch/tmp/pwn"},
		{Name: "badjump", Host: "example.com", ProxyJump: "-W target:22"},
		{Name: "badcommand", Host: "example.com", RemoteCommand: "-oProxyCommand=touch/tmp/pwn"},
		{Name: "badtag", Host: "example.com", Tags: []string{"-oProxyCommand=pwn"}},
	}
	for _, tc := range cases {
		if _, err := Validate(tc); err == nil {
			t.Fatalf("Validate(%#v) nil error, want option-like value rejected", tc)
		}
	}
}

func TestValidateRejectsProtectedMetadataWithoutLeakingValue(t *testing.T) {
	cases := []Profile{
		{Name: "sentinel", Host: "example.com", Notes: fakeProtectedSentinel},
		{Name: "password", Host: "example.com", Notes: "password=" + fakeProtectedSentinel},
		{Name: "passphrase", Host: "example.com", IdentityFile: "/tmp/passphrase=" + fakeProtectedSentinel},
		{Name: "token", Host: "example.com", Tags: []string{"token=" + fakeProtectedSentinel}},
	}
	for _, tc := range cases {
		_, err := Validate(tc)
		if err == nil {
			t.Fatalf("Validate(%#v) nil error, want protected metadata rejected", tc)
		}
		if strings.Contains(err.Error(), fakeProtectedSentinel) {
			t.Fatalf("Validate error leaked sentinel: %v", err)
		}
	}
}

func TestValidateRejectsPrivateKeyLikeOrMultilineMetadata(t *testing.T) {
	cases := []Profile{
		{Name: "key", Host: "example.com", IdentityFile: "-----BEGIN OPENSSH PRIVATE KEY-----"},
		{Name: "notes", Host: "example.com", Notes: "line one\nline two"},
		{Name: "cmd", Host: "example.com", RemoteCommand: "echo ok\nwhoami"},
	}
	for _, tc := range cases {
		if _, err := Validate(tc); err == nil {
			t.Fatalf("Validate(%#v) nil error, want unsafe metadata rejected", tc)
		}
	}
}

func TestValidateAllowsOrdinaryCredentialWordsInProse(t *testing.T) {
	_, err := Validate(Profile{Name: "prod", Host: "example.com", Notes: "passwordless login for secret project"})
	if err != nil {
		t.Fatalf("Validate ordinary prose error: %v", err)
	}
}

func TestValidateAcceptsForwardSyntaxVariants(t *testing.T) {
	p := Profile{
		Name: "forwards",
		Host: "example.com",
		LocalForwards: []string{
			"15432:db.internal:5432",
			"127.0.0.1:15433:db.internal:5432",
			":15434:db.internal:5432",
			"/tmp/sshdex-local.sock:db.internal:5432",
			"15435:/tmp/sshdex-remote.sock",
		},
		RemoteForwards: []string{
			"18080:localhost:8080",
			"0.0.0.0:18081:localhost:8080",
			"0:localhost:8080",
			"/tmp/sshdex-remote.sock:localhost:8080",
			"18082:/tmp/sshdex-local.sock",
		},
		DynamicForwards: []string{
			"1080",
			"127.0.0.1:1081",
			":1082",
		},
	}

	got, err := Validate(p)
	if err != nil {
		t.Fatalf("Validate() unexpected error: %v", err)
	}
	if got.LocalForwards[0] != "15432:db.internal:5432" || got.DynamicForwards[0] != "1080" {
		t.Fatalf("forward variants not preserved: %#v", got)
	}
}

func TestValidateRejectsUnsafeForwardingValues(t *testing.T) {
	cases := []Profile{
		{Name: "local", Host: "example.com", LocalForwards: []string{"-oProxyCommand=pwn"}},
		{Name: "remote", Host: "example.com", RemoteForwards: []string{"bad\nvalue"}},
		{Name: "dynamic", Host: "example.com", DynamicForwards: []string{"-----BEGIN OPENSSH PRIVATE KEY-----"}},
	}
	for _, tc := range cases {
		if _, err := Validate(tc); err == nil {
			t.Fatalf("Validate(%#v) nil error, want unsafe forwarding rejected", tc)
		}
	}
}

func TestValidateRejectsMalformedForwardSyntax(t *testing.T) {
	cases := []struct {
		name    string
		profile Profile
	}{
		{name: "local missing host port", profile: Profile{Name: "bad", Host: "example.com", LocalForwards: []string{"15432:db.internal"}}},
		{name: "local empty host", profile: Profile{Name: "bad", Host: "example.com", LocalForwards: []string{"127.0.0.1:15432::5432"}}},
		{name: "local zero listen port", profile: Profile{Name: "bad", Host: "example.com", LocalForwards: []string{"127.0.0.1:0:db.internal:5432"}}},
		{name: "local relative socket", profile: Profile{Name: "bad", Host: "example.com", LocalForwards: []string{"15432:relative.sock"}}},
		{name: "local socket whitespace", profile: Profile{Name: "bad", Host: "example.com", LocalForwards: []string{"15432:/tmp/bad socket"}}},
		{name: "local high host port", profile: Profile{Name: "bad", Host: "example.com", LocalForwards: []string{"127.0.0.1:15432:db.internal:65536"}}},
		{name: "local nonnumeric listen port", profile: Profile{Name: "bad", Host: "example.com", LocalForwards: []string{"127.0.0.1:port:db.internal:5432"}}},
		{name: "remote nonnumeric host port", profile: Profile{Name: "bad", Host: "example.com", RemoteForwards: []string{"18080:localhost:http"}}},
		{name: "remote too many fields", profile: Profile{Name: "bad", Host: "example.com", RemoteForwards: []string{"127.0.0.1:18080:localhost:8080:extra"}}},
		{name: "dynamic nonnumeric port", profile: Profile{Name: "bad", Host: "example.com", DynamicForwards: []string{"socks"}}},
		{name: "dynamic zero port", profile: Profile{Name: "bad", Host: "example.com", DynamicForwards: []string{"127.0.0.1:0"}}},
		{name: "dynamic missing port", profile: Profile{Name: "bad", Host: "example.com", DynamicForwards: []string{"127.0.0.1:"}}},
		{name: "dynamic too many fields", profile: Profile{Name: "bad", Host: "example.com", DynamicForwards: []string{"127.0.0.1:1080:extra"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Validate(tc.profile); err == nil {
				t.Fatalf("Validate(%#v) nil error, want malformed forward rejected", tc.profile)
			}
		})
	}
}
