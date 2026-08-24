package micropub

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// EntryWorkflow exposes the complete Micropub create/edit lifecycle.
type EntryWorkflow interface {
	CreateEntry(context.Context, EntryCreateRequest, ...socialhub.CallOption) (*EntryResult, error)
	UpdateEntry(context.Context, EntryUpdateRequest, ...socialhub.CallOption) (*EntryResult, error)
	DeleteEntry(context.Context, string, ...socialhub.CallOption) (*EntryResult, error)
	UndeleteEntry(context.Context, string, ...socialhub.CallOption) (*EntryResult, error)
	Source(context.Context, string, []string, ...socialhub.CallOption) (*Entry, error)
}

// QueryWorkflow exposes standardized Micropub endpoint queries.
type QueryWorkflow interface {
	Config(context.Context, ...socialhub.CallOption) (*Config, error)
	SyndicationTargets(context.Context, ...socialhub.CallOption) ([]SyndicationTarget, error)
}

// MediaWorkflow uploads one file to a discovered Micropub Media Endpoint.
type MediaWorkflow interface {
	UploadMedia(context.Context, MediaUploadRequest, io.Reader, ...socialhub.CallOption) (*MediaResult, error)
}

// Content distinguishes plain text from authored HTML.
type Content struct {
	Text string `json:"text,omitempty"`
	HTML string `json:"html,omitempty"`
}

// Photo preserves Micropub's optional image alt text object.
type Photo struct {
	Value string `json:"value"`
	Alt   string `json:"alt,omitempty"`
}

// EntryCreateRequest describes a Microformats2 object. Types defaults to
// h-entry; ExtraProperties supports valid JSON values without weakening typed
// fields or allowing reserved command names to be overwritten.
type EntryCreateRequest struct {
	Types           []string                     `json:"types,omitempty"`
	Name            string                       `json:"name,omitempty"`
	Summary         string                       `json:"summary,omitempty"`
	Content         Content                      `json:"content,omitempty"`
	Published       *time.Time                   `json:"published,omitempty"`
	Categories      []string                     `json:"categories,omitempty"`
	Location        string                       `json:"location,omitempty"`
	InReplyTo       []string                     `json:"in_reply_to,omitempty"`
	LikeOf          []string                     `json:"like_of,omitempty"`
	RepostOf        []string                     `json:"repost_of,omitempty"`
	Photos          []Photo                      `json:"photos,omitempty"`
	Videos          []string                     `json:"videos,omitempty"`
	Audios          []string                     `json:"audios,omitempty"`
	SyndicateTo     []string                     `json:"syndicate_to,omitempty"`
	ExtraProperties map[string][]json.RawMessage `json:"extra_properties,omitempty"`
}

// EntryUpdateRequest represents Micropub's replace/add/delete operations.
// DeleteProperties removes whole properties; DeleteValues removes selected
// values. The protocol cannot encode both delete forms in one request.
type EntryUpdateRequest struct {
	URL              string                       `json:"url"`
	Replace          map[string][]json.RawMessage `json:"replace,omitempty"`
	Add              map[string][]json.RawMessage `json:"add,omitempty"`
	DeleteProperties []string                     `json:"delete_properties,omitempty"`
	DeleteValues     map[string][]json.RawMessage `json:"delete_values,omitempty"`
}

// Entry is a source query result with lossless Microformats2 properties.
type Entry struct {
	Type       []string                     `json:"type,omitempty"`
	Properties map[string][]json.RawMessage `json:"properties"`
	Raw        json.RawMessage              `json:"raw,omitempty"`
}

// EntryResult describes the URL and synchronous/asynchronous outcome returned
// by create, update, delete, or undelete.
type EntryResult struct {
	URL         string                 `json:"url"`
	State       socialhub.PublishState `json:"state"`
	Shortlinks  []string               `json:"shortlinks,omitempty"`
	Syndication []string               `json:"syndication,omitempty"`
}

// SyndicationIdentity is optional service or user display metadata.
type SyndicationIdentity struct {
	Name  string `json:"name"`
	URL   string `json:"url,omitempty"`
	Photo string `json:"photo,omitempty"`
}

// SyndicationTarget is an endpoint accepted by mp-syndicate-to.
type SyndicationTarget struct {
	UID     string                     `json:"uid"`
	Name    string                     `json:"name"`
	Service *SyndicationIdentity       `json:"service,omitempty"`
	User    *SyndicationIdentity       `json:"user,omitempty"`
	Raw     map[string]json.RawMessage `json:"-"`
}

// Config is the standardized q=config response.
type Config struct {
	MediaEndpoint string              `json:"media_endpoint,omitempty"`
	SyndicateTo   []SyndicationTarget `json:"syndicate_to,omitempty"`
	Raw           json.RawMessage     `json:"raw,omitempty"`
}

// MediaUploadRequest supplies the discovered endpoint and exact stream size.
type MediaUploadRequest struct {
	Endpoint string `json:"endpoint"`
	Filename string `json:"filename"`
	MIME     string `json:"mime"`
	Size     int64  `json:"size"`
}

// MediaResult is the URL returned by the Media Endpoint.
type MediaResult struct {
	URL string `json:"url"`
}

type createPayload struct {
	Type       []string                     `json:"type"`
	Properties map[string][]json.RawMessage `json:"properties"`
}

type updatePayload struct {
	Action  string                       `json:"action"`
	URL     string                       `json:"url"`
	Replace map[string][]json.RawMessage `json:"replace,omitempty"`
	Add     map[string][]json.RawMessage `json:"add,omitempty"`
	Delete  any                          `json:"delete,omitempty"`
}

type actionPayload struct {
	Action string `json:"action"`
	URL    string `json:"url"`
}

type configWire struct {
	MediaEndpoint string              `json:"media-endpoint"`
	SyndicateTo   []SyndicationTarget `json:"syndicate-to"`
}

type syndicationWire struct {
	SyndicateTo []SyndicationTarget `json:"syndicate-to"`
}
