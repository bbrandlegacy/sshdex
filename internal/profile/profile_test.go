package profile

import (
	"testing"
	"time"
)

func TestValidateNormalizesValidProfile(t *testing.T) {
	now := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	p := Profile{
		Name:         "  prod-web-01  ",
		Host:         "  192.0.2.10  ",
		User:         " deploy ",
		Port:         0,
		IdentityFile: " ~/.ssh/id_ed25519 ",
		Tags:         []string{" prod ", " web "},
		Notes:        " production host ",
		ProxyJump:    " bastion ",
		CreatedAt:    now,
		UpdatedAt:    now,
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
		{Name: "badhost", Host: "-oProxyCommand=touch/tmp/pwn"},
		{Name: "baduser", Host: "example.com", User: "-lroot"},
	}
	for _, tc := range cases {
		if _, err := Validate(tc); err == nil {
			t.Fatalf("Validate(%#v) nil error, want option-like value rejected", tc)
		}
	}
}

func TestValidateRejectsPrivateKeyLikeOrMultilineMetadata(t *testing.T) {
	cases := []Profile{
		{Name: "key", Host: "example.com", IdentityFile: "-----BEGIN OPENSSH PRIVATE KEY-----"},
		{Name: "notes", Host: "example.com", Notes: "line one\nline two"},
	}
	for _, tc := range cases {
		if _, err := Validate(tc); err == nil {
			t.Fatalf("Validate(%#v) nil error, want unsafe metadata rejected", tc)
		}
	}
}
