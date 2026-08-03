package googlebusinessprofile

import (
	"context"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityLocalPostWorkflow socialhub.Capability = "google_business_profile_local_post_workflow"
	CapabilityReviewWorkflow    socialhub.Capability = "google_business_profile_review_workflow"
)

// Client exposes Local Posts and Reviews for one configured business location.
type Client struct {
	accountID       socialhub.AccountID
	googleAccountID string
	locationID      string
	languageCode    string
	api             *transport.Client
	scopes          []string
	clock           socialhub.Clock
}

func (client *Client) Platform() socialhub.Platform { return platformName }
func (client *Client) Account() socialhub.AccountID { return client.accountID }

func (client *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		socialhub.CapPublish: capabilityState(
			socialhub.CapPublish, true, client.scopes,
			"STANDARD Local Posts for the configured location; Google Business Profile API access must be approved",
			documentationURL+"v4/accounts.locations.localPosts/create",
		),
		socialhub.CapFetch: capabilityState(
			socialhub.CapFetch, true, client.scopes,
			"configured business location and its Local Posts; comment listing is not represented by this API",
			documentationURL+"v4/accounts.locations.localPosts/list",
		),
		CapabilityLocalPostWorkflow: capabilityState(
			CapabilityLocalPostWorkflow, true, client.scopes,
			"STANDARD, EVENT, OFFER, ALERT, CTA, remote media, scheduling, and patch workflows",
			documentationURL+"v4/accounts.locations.localPosts",
		),
		CapabilityReviewWorkflow: capabilityState(
			CapabilityReviewWorkflow, true, client.scopes,
			"review reads and owner replies; the configured location must be verified",
			documentationURL+"v4/accounts.locations.reviews",
		),
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Local Posts accept public sourceUrl media, but the common resumable upload contract does not match Google Business Profile media upload"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Google Business Profile v4 does not expose likes or owner-created customer reviews"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Business Profile APIs do not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Notifications use Google Cloud Pub/Sub and do not define an account-scoped signed HTTP callback contract"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if supported && len(granted) > 0 {
		approval = socialhub.ApprovalRequired
		if scopeGranted(granted, businessScope) {
			approval = socialhub.ApprovalGranted
		}
	}
	return socialhub.CapabilityState{
		Capability: capability, Supported: supported, Approval: approval, Scopes: []string{businessScope},
		Reason: reason, DocURL: docURL,
	}
}

func (client *Client) Publisher() (socialhub.Publisher, bool)         { return client, true }
func (client *Client) Fetcher() (socialhub.Fetcher, bool)             { return client, true }
func (client *Client) MediaUploader() (socialhub.MediaUploader, bool) { return nil, false }
func (client *Client) Reactor() (socialhub.Reactor, bool)             { return nil, false }
func (client *Client) Messenger() (socialhub.Messenger, bool)         { return nil, false }
func (client *Client) WebhookHandler() (socialhub.WebhookHandler, bool) {
	return nil, false
}
func (client *Client) Close() error { return nil }

func (client *Client) LocalPostWorkflow() LocalPostWorkflow { return client }
func (client *Client) ReviewWorkflow() ReviewWorkflow       { return client }

func (client *Client) accountResource() string {
	return "accounts/" + client.googleAccountID
}

func (client *Client) locationResource() string {
	return client.accountResource() + "/locations/" + client.locationID
}

func (client *Client) localPostResource(postID string) string {
	return client.locationResource() + "/localPosts/" + postID
}

func (client *Client) reviewResource(reviewID string) string {
	return client.locationResource() + "/reviews/" + reviewID
}

func (client *Client) requireScope(operation string) error {
	if len(client.scopes) == 0 || scopeGranted(client.scopes, businessScope) {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: platformName, Product: productName,
		Op: operation, RequiredScopes: []string{businessScope}, ApprovalURL: defaultAuthURL,
		PlatformMessage: "configured approval scopes do not include Google Business Profile management",
	}
}

func scopeGranted(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Publisher = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
