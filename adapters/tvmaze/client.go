package tvmaze

import (
	"context"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const (
	CapabilityCatalog  socialhub.Capability = "tvmaze_catalog"
	CapabilitySchedule socialhub.Capability = "tvmaze_schedule"
	CapabilityPeople   socialhub.Capability = "tvmaze_people"
	CapabilityUpdates  socialhub.Capability = "tvmaze_updates"
)

// Client exposes typed TVmaze catalog, schedule, people, and update workflows.
type Client struct {
	accountID socialhub.AccountID
	api       *transport.Client
	baseURL   *url.URL
}

func (c *Client) Platform() socialhub.Platform { return "tvmaze" }
func (c *Client) Account() socialhub.AccountID { return c.accountID }

func (c *Client) Capabilities(context.Context) (socialhub.Capabilities, error) {
	publicRead := func(capability socialhub.Capability, reason string) socialhub.CapabilityState {
		return socialhub.CapabilityState{
			Capability: capability, Supported: true, Approval: socialhub.ApprovalGranted,
			Reason: reason, DocURL: documentationURL,
		}
	}
	return socialhub.Capabilities{
		CapabilityCatalog:    publicRead(CapabilityCatalog, "public show, episode, season, cast, and crew reads; CC BY-SA attribution and ShareAlike apply"),
		CapabilitySchedule:   publicRead(CapabilitySchedule, "public broadcast and web-channel schedules; CC BY-SA attribution and ShareAlike apply"),
		CapabilityPeople:     publicRead(CapabilityPeople, "public people search and embedded show credits; CC BY-SA attribution and ShareAlike apply"),
		CapabilityUpdates:    publicRead(CapabilityUpdates, "public show and people update timestamps for incremental synchronization"),
		socialhub.CapPublish: {Capability: socialhub.CapPublish, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TVmaze does not expose social publishing"},
		socialhub.CapFetch:   {Capability: socialhub.CapFetch, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "TV catalog resources are exposed through typed workflows"},
		socialhub.CapMedia:   {Capability: socialhub.CapMedia, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the public API does not accept media uploads"},
		socialhub.CapReact:   {Capability: socialhub.CapReact, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the public API does not expose reactions"},
		socialhub.CapMessage: {Capability: socialhub.CapMessage, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the public API does not expose direct messaging"},
		socialhub.CapWebhook: {Capability: socialhub.CapWebhook, Supported: false, Approval: socialhub.ApprovalUnknown, Reason: "the public API does not document signed webhooks"},
	}, nil
}

func (c *Client) Publisher() (socialhub.Publisher, bool)           { return nil, false }
func (c *Client) Fetcher() (socialhub.Fetcher, bool)               { return nil, false }
func (c *Client) MediaUploader() (socialhub.MediaUploader, bool)   { return nil, false }
func (c *Client) Reactor() (socialhub.Reactor, bool)               { return nil, false }
func (c *Client) Messenger() (socialhub.Messenger, bool)           { return nil, false }
func (c *Client) WebhookHandler() (socialhub.WebhookHandler, bool) { return nil, false }
func (c *Client) Close() error                                     { return nil }

func (c *Client) CatalogWorkflow() CatalogWorkflow   { return c }
func (c *Client) ScheduleWorkflow() ScheduleWorkflow { return c }
func (c *Client) PeopleWorkflow() PeopleWorkflow     { return c }
func (c *Client) UpdatesWorkflow() UpdatesWorkflow   { return c }

var _ socialhub.Client = (*Client)(nil)
