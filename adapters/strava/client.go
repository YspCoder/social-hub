package strava

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityActivityWorkflow socialhub.Capability = "strava_activity_workflow"
	CapabilityActivityUpload   socialhub.Capability = "strava_activity_upload"
)

// Client exposes activity reads, typed activity writes, uploads, and webhook
// processing for one authorized athlete.
type Client struct {
	accountID      socialhub.AccountID
	athleteID      string
	subscriptionID int64
	api            *transport.Client
	scopes         []string
	verifyToken    string
	clock          socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return "strava" }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	webhookSupported := client.subscriptionID > 0 && client.verifyToken != ""
	return socialhub.Capabilities{
		socialhub.CapFetch: capabilityState(
			socialhub.CapFetch, true, client.scopes, []string{"read", "activity:read"},
			"authenticated athlete, owned activities, and activity comments", documentationURL,
		),
		CapabilityActivityWorkflow: capabilityState(
			CapabilityActivityWorkflow, true, client.scopes, []string{"activity:write"},
			"manual activity creation and owned activity updates", documentationURL+"#api-Activities",
		),
		CapabilityActivityUpload: capabilityState(
			CapabilityActivityUpload, true, client.scopes, []string{"activity:write"},
			"streaming FIT, TCX, GPX, and limited strength-training JSON uploads", "https://developers.strava.com/docs/uploads/",
		),
		socialhub.CapWebhook: capabilityState(
			socialhub.CapWebhook, webhookSupported, client.scopes, []string{"activity:read"},
			"application subscription challenge and account-scoped activity/athlete event decoding; POST events are not signed", "https://developers.strava.com/docs/webhooks/",
		),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "common posts cannot represent sport type, local start time, and elapsed time; use ActivityWorkflow"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "activity files create activities asynchronously; use ActivityUploadWorkflow"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Strava API v3 exposes comments and kudoers as read-only resources"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Strava API v3 does not expose direct messaging"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !scopeGranted(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval, Scopes: append([]string(nil), required...),
		Reason: reason, DocURL: docURL,
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return nil, false }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	if client.subscriptionID <= 0 || client.verifyToken == "" {
		return nil, false
	}
	return client, true
}
func (client *Client) Close() error { return nil }

func (client *Client) ActivityWorkflow() ActivityWorkflow             { return client }
func (client *Client) ActivityUploadWorkflow() ActivityUploadWorkflow { return client }

func (client *Client) requireScopes(operation string, required ...string) error {
	if len(client.scopes) == 0 {
		return nil
	}
	missing := make([]string, 0, len(required))
	for _, scope := range required {
		if !scopeGranted(client.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "strava", Product: productName,
		Op: operation, RequiredScopes: missing, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not include required Strava permissions",
	}
}

func scopeGranted(scopes []string, target string) bool {
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == target || target == "activity:read" && scope == "activity:read_all" {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
