package sshcmd

import (
	"reflect"
	"strings"
	"testing"

	"github.com/bbrandlegacy/sshdex/internal/profile"
)

func TestBuildArgsForDirectProfile(t *testing.T) {
	got, err := BuildArgs(profile.Profile{Name: "prod", Host: "example.com", User: "deploy", Port: 2200, IdentityFile: "~/.ssh/prod key"})
	if err != nil {
		t.Fatalf("BuildArgs() error: %v", err)
	}
	want := []string{"-p", "2200", "-i", "~/.ssh/prod key", "deploy@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs() = %#v, want %#v", got, want)
	}
}

func TestBuildArgsUsesHostOnlyWhenUserOmitted(t *testing.T) {
	got, err := BuildArgs(profile.Profile{Name: "prod", Host: "example.com"})
	if err != nil {
		t.Fatalf("BuildArgs() error: %v", err)
	}
	want := []string{"example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs() = %#v, want %#v", got, want)
	}
}

func TestBuildArgsIncludesProxyJump(t *testing.T) {
	got, err := BuildArgs(profile.Profile{Name: "prod", Host: "target", User: "deploy", ProxyJump: "bastion"})
	if err != nil {
		t.Fatalf("BuildArgs() error: %v", err)
	}
	want := []string{"-J", "bastion", "deploy@target"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs() = %#v, want %#v", got, want)
	}
}

func TestBuildArgsIncludesForwardsAndRemoteCommand(t *testing.T) {
	got, err := BuildArgs(profile.Profile{
		Name:            "prod",
		Host:            "target",
		User:            "deploy",
		LocalForwards:   []string{"127.0.0.1:15432:db.internal:5432"},
		RemoteForwards:  []string{"0.0.0.0:18080:localhost:8080"},
		DynamicForwards: []string{"127.0.0.1:1080"},
		RemoteCommand:   "uptime -p",
	})
	if err != nil {
		t.Fatalf("BuildArgs() error: %v", err)
	}
	want := []string{
		"-L", "127.0.0.1:15432:db.internal:5432",
		"-R", "0.0.0.0:18080:localhost:8080",
		"-D", "127.0.0.1:1080",
		"deploy@target",
		"uptime -p",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildArgs() = %#v, want %#v", got, want)
	}
}

func TestPreviewShellQuotesSpecialArguments(t *testing.T) {
	got, err := Preview(profile.Profile{Name: "prod", Host: "host.example", User: "deploy", IdentityFile: "~/.ssh/prod key's file"})
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	want := `ssh -i '~/.ssh/prod key'\''s file' deploy@host.example`
	if got != want {
		t.Fatalf("Preview() = %q, want %q", got, want)
	}
}

func TestBuildArgsRejectsOptionLikeInjectionValues(t *testing.T) {
	cases := []profile.Profile{
		{Name: "host", Host: "-oProxyCommand=touch/tmp/pwn"},
		{Name: "identity", Host: "example.com", IdentityFile: "-oProxyCommand=touch/tmp/pwn"},
		{Name: "jump", Host: "example.com", ProxyJump: "-W target:22"},
		{Name: "forward", Host: "example.com", LocalForwards: []string{"-oProxyCommand=touch/tmp/pwn"}},
	}
	for _, tc := range cases {
		if _, err := BuildArgs(tc); err == nil {
			t.Fatalf("BuildArgs(%#v) nil error, want injection boundary rejection", tc)
		}
	}
}

func TestPreviewRejectsProtectedSentinelWithoutLeaking(t *testing.T) {
	const sentinel = "SSHDX_TEST_SECRET_DO_NOT_PRINT_12345"
	_, err := Preview(profile.Profile{Name: "prod", Host: "example.com", Notes: sentinel})
	if err == nil {
		t.Fatal("Preview nil error, want protected metadata rejection")
	}
	if strings.Contains(err.Error(), sentinel) {
		t.Fatalf("Preview error leaked sentinel: %v", err)
	}
}
