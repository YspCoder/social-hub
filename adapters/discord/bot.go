package discord

import (
	"context"

	"social-hub/pkg/socialhub"
)

// GatewayInfo contains the response from Discord's Get Gateway Bot endpoint.
type GatewayInfo struct {
	URL               string            `json:"url"`
	Shards            int               `json:"shards"`
	SessionStartLimit SessionStartLimit `json:"session_start_limit"`
}

// SessionStartLimit describes identify concurrency and remaining sessions.
type SessionStartLimit struct {
	Total          int `json:"total"`
	Remaining      int `json:"remaining"`
	ResetAfterMS   int `json:"reset_after"`
	MaxConcurrency int `json:"max_concurrency"`
}

// BotWorkflow exposes Discord-specific bot operations that do not fit a
// common capability interface.
type BotWorkflow interface {
	CurrentUser(context.Context, ...socialhub.CallOption) (*socialhub.User, error)
	Gateway(context.Context, ...socialhub.CallOption) (*GatewayInfo, error)
}

// BotService implements BotWorkflow.
type BotService struct{ client *Client }

func (s *BotService) CurrentUser(ctx context.Context, options ...socialhub.CallOption) (*socialhub.User, error) {
	var response discordUser
	if err := s.client.get(ctx, "/users/@me", nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, wrapError("current_user", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := s.client.mapUser(response)
	return &result, nil
}

func (s *BotService) Gateway(ctx context.Context, options ...socialhub.CallOption) (*GatewayInfo, error) {
	var response GatewayInfo
	if err := s.client.get(ctx, "/gateway/bot", nil, &response, options...); err != nil {
		return nil, err
	}
	if response.URL == "" || response.Shards <= 0 || response.SessionStartLimit.MaxConcurrency <= 0 {
		return nil, wrapError("gateway", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

var _ BotWorkflow = (*BotService)(nil)
