package sshconfig

import "testing"

func TestParseConfigImportsConcreteHosts(t *testing.T) {
	input := `
# comment
Host prod prod-alias
    HostName prod.example.com
    User deploy
    Port 2200
    IdentityFile ~/.ssh/prod key
    ProxyJump bastion
    ForwardAgent yes

Host *
    User ignored

Host dev
    HostName dev.example.com
`
	profiles, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString() error: %v", err)
	}
	if len(profiles) != 3 {
		t.Fatalf("len(profiles) = %d, want 3: %#v", len(profiles), profiles)
	}
	prod := profiles[0]
	if prod.Name != "prod" || prod.Host != "prod.example.com" || prod.User != "deploy" || prod.Port != 2200 || prod.IdentityFile != "~/.ssh/prod key" || prod.ProxyJump != "bastion" {
		t.Fatalf("prod profile unexpected: %#v", prod)
	}
	alias := profiles[1]
	if alias.Name != "prod-alias" || alias.Host != "prod.example.com" {
		t.Fatalf("alias profile unexpected: %#v", alias)
	}
	dev := profiles[2]
	if dev.Name != "dev" || dev.Host != "dev.example.com" || dev.Port != 22 {
		t.Fatalf("dev profile unexpected: %#v", dev)
	}
}

func TestParseConfigHandlesInlineCommentsAndAliasDefaultHosts(t *testing.T) {
	input := `
Host prod prod-alias # production aliases
  User deploy # inline user comment
  Port 2222 # inline port comment
  IdentityFile "~/.ssh/prod key" # quoted path
`
	profiles, err := ParseString(input)
	if err != nil {
		t.Fatalf("ParseString() error: %v", err)
	}
	if len(profiles) != 2 {
		t.Fatalf("len(profiles) = %d, want 2: %#v", len(profiles), profiles)
	}
	if profiles[0].Name != "prod" || profiles[0].Host != "prod" || profiles[0].Port != 2222 || profiles[0].IdentityFile != "~/.ssh/prod key" {
		t.Fatalf("first profile unexpected: %#v", profiles[0])
	}
	if profiles[1].Name != "prod-alias" || profiles[1].Host != "prod-alias" {
		t.Fatalf("alias host should default to alias itself: %#v", profiles[1])
	}
}
