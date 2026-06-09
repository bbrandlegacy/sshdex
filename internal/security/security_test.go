package security

import (
	"errors"
	"strings"
	"testing"
)

const fakeSentinel = "SSHDX_TEST_SECRET_DO_NOT_PRINT_12345"

func TestRedactRemovesProtectedMaterial(t *testing.T) {
	privateKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nkey-body-without-end-marker"
	input := "token=" + fakeSentinel + " password:let me in, " + privateKey
	got := Redact(input)
	for _, leaked := range []string{fakeSentinel, "let me in", "key-body", "OPENSSH PRIVATE KEY"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("Redact leaked %q in %q", leaked, got)
		}
	}
	if !strings.Contains(got, Redacted) {
		t.Fatalf("Redact did not include redaction marker: %q", got)
	}
}

func TestRedactSanitizesControlCharacters(t *testing.T) {
	got := Redact("safe\n\x1b[31mred")
	if strings.Contains(got, "\n") || strings.Contains(got, "\x1b") {
		t.Fatalf("Redact left terminal control characters: %q", got)
	}
}

func TestValidateMetadataRejectsProtectedMaterialWithoutEchoingValue(t *testing.T) {
	cases := []string{
		fakeSentinel,
		"password=" + fakeSentinel,
		"passphrase: " + fakeSentinel,
		"api_key=" + fakeSentinel,
		"-----BEGIN OPENSSH PRIVATE KEY-----",
	}
	for _, value := range cases {
		err := ValidateMetadata("notes", value)
		if err == nil {
			t.Fatalf("ValidateMetadata(%q) nil error, want rejection", value)
		}
		if strings.Contains(err.Error(), fakeSentinel) || strings.Contains(err.Error(), value) {
			t.Fatalf("ValidateMetadata error leaked value %q: %v", value, err)
		}
	}
}

func TestValidateMetadataAllowsOrdinaryCredentialWordsInProse(t *testing.T) {
	for _, value := range []string{"passwordless login enabled", "secret project notes", "token bucket algorithm"} {
		if err := ValidateMetadata("notes", value); err != nil {
			t.Fatalf("ValidateMetadata(%q) unexpected error: %v", value, err)
		}
	}
}

func TestValidateMetadataRejectsControlCharacters(t *testing.T) {
	err := ValidateMetadata("notes", "one\ntwo")
	if err == nil {
		t.Fatal("ValidateMetadata nil error, want control character rejection")
	}
	if strings.Contains(err.Error(), "one") || strings.Contains(err.Error(), "two") {
		t.Fatalf("control-character error leaked value: %v", err)
	}
}

func TestErrorStringRedactsErrorText(t *testing.T) {
	got := ErrorString(errors.New("failed with " + fakeSentinel))
	if strings.Contains(got, fakeSentinel) {
		t.Fatalf("ErrorString leaked sentinel: %q", got)
	}
	if !strings.Contains(got, Redacted) {
		t.Fatalf("ErrorString missing redaction marker: %q", got)
	}
}

func TestRejectOptionLike(t *testing.T) {
	if err := RejectOptionLike("host", "-oProxyCommand=touch /tmp/pwn"); err == nil {
		t.Fatal("RejectOptionLike nil error, want rejection")
	}
	if err := RejectOptionLike("host", "example.com"); err != nil {
		t.Fatalf("RejectOptionLike valid value error: %v", err)
	}
}

func TestFutureOperationPolicyContract(t *testing.T) {
	allowed := FutureOperationPolicy(OperationMetadataRead)
	if !allowed.Allowed || !allowed.RedactResponse || allowed.RequiresHumanApproval {
		t.Fatalf("metadata policy unexpected: %#v", allowed)
	}
	credential := FutureOperationPolicy(OperationCredentialReveal)
	if credential.Allowed || !credential.RedactResponse || credential.RequiresHumanApproval {
		t.Fatalf("credential reveal policy unexpected: %#v", credential)
	}
	for _, op := range []OperationClass{OperationConnect, OperationDelete, OperationExport, OperationSync} {
		decision := FutureOperationPolicy(op)
		if decision.Allowed || !decision.RequiresHumanApproval || !decision.RedactResponse {
			t.Fatalf("%s policy unexpected: %#v", op, decision)
		}
	}
	unknown := FutureOperationPolicy(OperationClass("mcp_" + fakeSentinel))
	if unknown.Allowed || strings.Contains(Redact(unknown.Reason+string(OperationClass("mcp_"+fakeSentinel))), fakeSentinel) {
		t.Fatalf("unknown policy did not deny/redact sentinel: %#v", unknown)
	}
}
