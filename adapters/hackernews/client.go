package hackernews

import (
	"context"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityItems   socialhub.Capability = "hackernews_items"
	CapabilityFeeds   socialhub.Capability = "hackernews_feeds"
	CapabilityUsers   socialhub.Capability = "hackernews_users"
	CapabilityUpdates socialhub.Capability = "hackernews_updates"
)

// Client exposes common fetching plus typed Hacker News workflows.
type Client struct {
	accountID   socialhub.AccountID
	api         *transport.Client
	clock       socialhub.Clock
	defaultFeed Feed
}

func (c *Client) Platform() socialhub.Platform { return "hackernews" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publicRead := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the official Hacker News API v0 is read-only"},
		socialhub.CapFetch:   publicRead(socialhub.CapFetch, "public users, stories, jobs, polls, ranked feeds, and direct comments"),
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the official API does not accept media uploads"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the official API does not expose voting operations"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "Hacker News does not expose direct messaging through API v0"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "updates are polled; API v0 does not document signed webhooks"},
		CapabilityItems:      publicRead(CapabilityItems, "raw item reads, direct-child pagination, and max-item discovery"),
		CapabilityFeeds:      publicRead(CapabilityFeeds, "top, new, best, Ask HN, Show HN, and job feeds"),
		CapabilityUsers:      publicRead(CapabilityUsers, "public profiles and submitted item IDs"),
		CapabilityUpdates:    publicRead(CapabilityUpdates, "recently changed item IDs and profile names"),
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return c, true }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) ItemWorkflow() ItemWorkflow       { return c }
func (c *Client) FeedWorkflow() FeedWorkflow       { return c }
func (c *Client) UserWorkflow() UserWorkflow       { return c }
func (c *Client) UpdatesWorkflow() UpdatesWorkflow { return c }

var _ socialhub.Client = (*Client)(nil)
var _ socialhub.Fetcher = (*Client)(nil)
var _ ItemWorkflow = (*Client)(nil)
var _ FeedWorkflow = (*Client)(nil)
var _ UserWorkflow = (*Client)(nil)
var _ UpdatesWorkflow = (*Client)(nil)
