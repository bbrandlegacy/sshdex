package sshcmd

import (
	"reflect"
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
