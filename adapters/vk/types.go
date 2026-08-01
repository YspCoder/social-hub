package vk

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// WallPostRequest exposes VK-specific wall publication controls. OwnerID zero
// selects the account's configured default wall.
type WallPostRequest struct {
	OwnerID           int64
	Message           string
	Attachments       []string
	FromGroup         bool
	FriendsOnly       bool
	Signed            bool
	PublishAt         *time.Time
	CloseComments     bool
	MuteNotifications bool
}

// RepostRequest creates an independent VK repost on the selected destination
// wall. Object is a composite owner_id_post_id identifier.
type RepostRequest struct {
	Object             string
	Message            string
	DestinationOwnerID int64
}

// WallWorkflow exposes VK wall controls that do not fit the common publisher.
type WallWorkflow interface {
	CreateWallPost(context.Context, WallPostRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	Repost(context.Context, RepostRequest, ...socialhub.CallOption) (*socialhub.Post, error)
}

// CallbackEvent retains a VK Callback API object and decodes common social
// entities when the event type has a stable schema.
type CallbackEvent struct {
	ID      string
	Type    string
	GroupID int64
	Version string
	Post    *socialhub.Post
	Comment *socialhub.Comment
	Message *socialhub.Message
	Object  json.RawMessage
}

// CallbackWorkflow retrieves the confirmation string VK expects when a
// Callback API endpoint is registered for the configured community.
type CallbackWorkflow interface {
	GetCallbackConfirmationCode(context.Context, ...socialhub.CallOption) (string, error)
}

var _ WallWorkflow = (*Client)(nil)
var _ CallbackWorkflow = (*Client)(nil)
