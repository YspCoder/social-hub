package reddit

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// RateLimit is the latest Reddit quota snapshot observed by this client.
type RateLimit struct {
	Used       float64
	Remaining  float64
	Reset      time.Duration
	ObservedAt time.Time
}

// Client implements Reddit's common read and interaction capabilities.
type Client struct {
	accountID   socialhub.AccountID
	userID      string
	username    string
	transport   *transport.Client
	scopes      []string
	clock       socialhub.Clock
	submissions *SubmissionService
	rateMu      sync.RWMutex
	rateLimit   RateLimit
}

func (c *Client) Platform() socialhub.Platform { return "reddit" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	return socialhub.Capabilities{
		CapabilitySubmissionWorkflow: capabilityState(CapabilitySubmissionWorkflow, true, c.scopes, []string{"submit", "edit"}, "subreddit and title fields require SubmissionWorkflow", docURL+"#POST_api_submit"),
		socialhub.CapPublish:         {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "use SubmissionWorkflow; common posts cannot express subreddit and title"},
		socialhub.CapFetch:           capabilityState(socialhub.CapFetch, true, c.scopes, []string{"identity", "read", "history"}, "authorized profile, submissions, posts, and included comment trees are supported", docURL),
		socialhub.CapMedia:           {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Reddit native media upload is not exposed by this stable Data API adapter"},
		socialhub.CapReact:           capabilityState(socialhub.CapReact, true, c.scopes, []string{"submit", "edit", "vote"}, "comments and human-initiated upvote/unvote are supported", docURL+"#POST_api_vote"),
		socialhub.CapMessage:         {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "private messages use a separate scope and are not included in the first adapter"},
		socialhub.CapWebhook:         {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Reddit Data API does not expose general account webhooks"},
	}, nil
}

func capabilityState(capability socialhub.Capability, supported bool, granted, required []string, reason, docURL string) socialhub.CapabilityState {
	approval := socialhub.ApprovalUnknown
	if len(granted) > 0 {
		approval = socialhub.ApprovalGranted
		for _, scope := range required {
			if !contains(granted, scope) {
				approval = socialhub.ApprovalRequired
				break
			}
		}
	}
	return socialhub.CapabilityState{Capability: capability, Supported: supported, Approval: approval, Scopes: required, Reason: reason, DocURL: docURL}
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return c, true }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

// SubmissionWorkflow returns Reddit's typed subreddit submission workflow.
func (c *Client) SubmissionWorkflow() SubmissionWorkflow { return c.submissions }

// RateLimit returns the most recently observed x-ratelimit header values.
func (c *Client) RateLimit() RateLimit {
	c.rateMu.RLock()
	defer c.rateMu.RUnlock()
	return c.rateLimit
}

func (c *Client) json(ctx context.Context, method, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	request, err := c.transport.NewRequest(ctx, method, path, query, nil, options...)
	if err != nil {
		return err
	}
	metadata, err := c.transport.DoWithMetadata(request, output)
	c.recordRateLimit(metadata.Header)
	return err
}

func (c *Client) form(ctx context.Context, path string, values url.Values, output any, options ...socialhub.CallOption) error {
	request, err := c.transport.NewRequest(ctx, http.MethodPost, path, nil, strings.NewReader(values.Encode()), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	metadata, err := c.transport.DoWithMetadata(request, output)
	c.recordRateLimit(metadata.Header)
	return err
}

func (c *Client) recordRateLimit(header http.Header) {
	if header == nil {
		return
	}
	used, usedErr := strconv.ParseFloat(header.Get("x-ratelimit-used"), 64)
	remaining, remainingErr := strconv.ParseFloat(header.Get("x-ratelimit-remaining"), 64)
	reset, resetErr := strconv.ParseFloat(header.Get("x-ratelimit-reset"), 64)
	if usedErr != nil && remainingErr != nil && resetErr != nil {
		return
	}
	c.rateMu.Lock()
	c.rateLimit = RateLimit{Used: used, Remaining: remaining, Reset: time.Duration(reset * float64(time.Second)), ObservedAt: c.clock.Now()}
	c.rateMu.Unlock()
}

func (c *Client) requireScopes(operation string, scopes ...string) error {
	if len(c.scopes) == 0 {
		return nil
	}
	var missing []string
	for _, scope := range scopes {
		if !contains(c.scopes, scope) {
			missing = append(missing, scope)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &socialhub.Error{
		Code: socialhub.CodeApprovalRequired, Class: socialhub.ClassUserAction, Platform: "reddit", Product: "reddit-data-api", Op: operation,
		RequiredScopes: missing, ApprovalURL: "https://support.reddithelp.com/hc/en-us/requests/new?ticket_form_id=14868593862164", PlatformMessage: "configured approval scopes are incomplete",
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ socialhub.Reactor = (*Client)(nil)
