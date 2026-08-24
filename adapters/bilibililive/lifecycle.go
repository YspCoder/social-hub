package bilibililive

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxBatchHeartbeatIDs = 199

func (client *Client) StartProject(ctx context.Context, identityCode string, options ...socialhub.CallOption) (*ProjectSession, error) {
	if err := client.ensureOpen("start_project"); err != nil {
		return nil, err
	}
	identityCode = strings.TrimSpace(identityCode)
	if !validOpaqueValue(identityCode, 512) {
		return nil, invalidArgument("start_project", "identity code must be a non-empty bounded value without control characters")
	}
	if err := validateCallOptions("start_project", options); err != nil {
		return nil, err
	}
	input := startProjectRequest{Code: identityCode, AppID: client.appID}
	var response responseEnvelope[ProjectSession]
	if err := client.api.JSON(ctx, http.MethodPost, "/v2/app/start", nil, input, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("start_project", http.StatusOK, nil); err != nil {
		return nil, err
	}
	if !validOpaqueValue(response.Data.GameInfo.GameID, 512) {
		return nil, platformError("start_project", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	webSocketInfo, err := normalizeWebSocketInfo(response.Data.WebSocketInfo)
	if err != nil {
		return nil, platformError("start_project", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	response.Data.WebSocketInfo = client.issueWebSocketInfo(webSocketInfo)
	response.Data.client = client
	return &response.Data, nil
}

func (client *Client) EndProject(ctx context.Context, gameID string, options ...socialhub.CallOption) error {
	if err := client.ensureOpen("end_project"); err != nil {
		return err
	}
	gameID = strings.TrimSpace(gameID)
	if !validOpaqueValue(gameID, 512) {
		return invalidArgument("end_project", "game ID must be a non-empty bounded value without control characters")
	}
	if err := validateCallOptions("end_project", options); err != nil {
		return err
	}
	input := endProjectRequest{AppID: client.appID, GameID: gameID}
	var response responseEnvelope[struct{}]
	if err := client.api.JSON(ctx, http.MethodPost, "/v2/app/end", nil, input, &response, options...); err != nil {
		return err
	}
	return response.Err("end_project", http.StatusOK, nil)
}

func (client *Client) Heartbeat(ctx context.Context, gameID string, options ...socialhub.CallOption) error {
	if err := client.ensureOpen("heartbeat"); err != nil {
		return err
	}
	gameID = strings.TrimSpace(gameID)
	if !validOpaqueValue(gameID, 512) {
		return invalidArgument("heartbeat", "game ID must be a non-empty bounded value without control characters")
	}
	if err := validateCallOptions("heartbeat", options); err != nil {
		return err
	}
	var response responseEnvelope[struct{}]
	if err := client.api.JSON(ctx, http.MethodPost, "/v2/app/heartbeat", nil, heartbeatRequest{GameID: gameID}, &response, options...); err != nil {
		return err
	}
	return response.Err("heartbeat", http.StatusOK, nil)
}

func (client *Client) BatchHeartbeat(ctx context.Context, gameIDs []string, options ...socialhub.CallOption) (*BatchHeartbeatResult, error) {
	if err := client.ensureOpen("batch_heartbeat"); err != nil {
		return nil, err
	}
	if len(gameIDs) == 0 || len(gameIDs) > maxBatchHeartbeatIDs {
		return nil, invalidArgument("batch_heartbeat", "game_ids must contain between 1 and 199 values")
	}
	if err := validateCallOptions("batch_heartbeat", options); err != nil {
		return nil, err
	}
	normalized := make([]string, 0, len(gameIDs))
	seen := make(map[string]struct{}, len(gameIDs))
	for _, gameID := range gameIDs {
		gameID = strings.TrimSpace(gameID)
		if !validOpaqueValue(gameID, 512) {
			return nil, invalidArgument("batch_heartbeat", "every game ID must be a non-empty bounded value without control characters")
		}
		if _, exists := seen[gameID]; exists {
			return nil, invalidArgument("batch_heartbeat", "game_ids must not contain duplicates")
		}
		seen[gameID] = struct{}{}
		normalized = append(normalized, gameID)
	}
	var response responseEnvelope[BatchHeartbeatResult]
	if err := client.api.JSON(ctx, http.MethodPost, "/v2/app/batchHeartbeat", nil, batchHeartbeatRequest{GameIDs: normalized}, &response, options...); err != nil {
		return nil, err
	}
	if err := response.Err("batch_heartbeat", http.StatusOK, nil); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (session *ProjectSession) End(ctx context.Context, options ...socialhub.CallOption) error {
	if session == nil || session.client == nil {
		return invalidArgument("end_project", "project session is not attached to a client")
	}
	return session.client.EndProject(ctx, session.GameInfo.GameID, options...)
}

func (session *ProjectSession) Heartbeat(ctx context.Context, options ...socialhub.CallOption) error {
	if session == nil || session.client == nil {
		return invalidArgument("heartbeat", "project session is not attached to a client")
	}
	return session.client.Heartbeat(ctx, session.GameInfo.GameID, options...)
}

func (session *ProjectSession) ConnectMessages(ctx context.Context, options ...StreamOption) (*MessageStream, error) {
	if session == nil || session.client == nil {
		return nil, invalidArgument("connect_messages", "project session is not attached to a client")
	}
	return session.client.ConnectMessages(ctx, session.WebSocketInfo, options...)
}

func validateCallOptions(operation string, options []socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "Bilibili Live Open Platform does not document idempotency keys")
	}
	if len(resolved.Fields) != 0 {
		return invalidArgument(operation, "Bilibili Live Open Platform does not support response field selection")
	}
	return nil
}

func validOpaqueValue(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

var _ ProjectLifecycle = (*Client)(nil)
