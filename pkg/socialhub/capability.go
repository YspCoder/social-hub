package socialhub

import "context"

// Capability identifies a composable client feature.
type Capability string

const (
	CapPublish Capability = "publish"
	CapFetch   Capability = "fetch"
	CapMedia   Capability = "media"
	CapReact   Capability = "react"
	CapMessage Capability = "message"
	CapWebhook Capability = "webhook"
)

// ApprovalState describes whether platform-side approval gates a capability.
type ApprovalState string

const (
	ApprovalUnknown  ApprovalState = "unknown"
	ApprovalRequired ApprovalState = "required"
	ApprovalGranted  ApprovalState = "granted"
)

// CapabilityState explains both technical support and platform-side approval.
type CapabilityState struct {
	Capability Capability
	Supported  bool
	Approval   ApprovalState
	Scopes     []string
	Reason     string
	DocURL     string
}

// Capabilities is a convenience lookup for capability declarations.
type Capabilities map[Capability]CapabilityState

// Has reports whether a capability is both implemented and not known to be
// blocked by approval.
func (c Capabilities) Has(capability Capability) bool {
	state, ok := c[capability]
	return ok && state.Supported && state.Approval != ApprovalRequired
}

// CapabilityProvider exposes the current capability declarations.
type CapabilityProvider interface {
	Capabilities(context.Context) (Capabilities, error)
}
