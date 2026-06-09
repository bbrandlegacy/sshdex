package security

type OperationClass string

const (
	OperationMetadataRead     OperationClass = "metadata_read"
	OperationDryRun           OperationClass = "dry_run"
	OperationCredentialReveal OperationClass = "credential_reveal"
	OperationConnect          OperationClass = "connect"
	OperationDelete           OperationClass = "delete"
	OperationExport           OperationClass = "export"
	OperationSync             OperationClass = "sync"
)

type PolicyDecision struct {
	Allowed               bool
	RequiresHumanApproval bool
	RedactResponse        bool
	Reason                string
}

// FutureOperationPolicy is a non-executing contract for future vault/sync/MCP
// stories. It does not implement those features; it records the safety gate
// they must preserve before any agent-facing capability is added.
func FutureOperationPolicy(op OperationClass) PolicyDecision {
	switch op {
	case OperationMetadataRead, OperationDryRun:
		return PolicyDecision{Allowed: true, RedactResponse: true, Reason: "metadata-only operation may run with redacted response"}
	case OperationCredentialReveal:
		return PolicyDecision{Allowed: false, RedactResponse: true, Reason: "credential reveal is denied by default"}
	case OperationConnect, OperationDelete, OperationExport, OperationSync:
		return PolicyDecision{Allowed: false, RequiresHumanApproval: true, RedactResponse: true, Reason: "side-effectful operation requires explicit human approval"}
	default:
		return PolicyDecision{Allowed: false, RedactResponse: true, Reason: "unknown operation denied by default"}
	}
}
