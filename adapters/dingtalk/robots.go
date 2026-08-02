package dingtalk

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"

	"social-hub/pkg/socialhub"
)

type robotSendResponse struct {
	apiError
	RobotSendResult
}

type robotSendPayload struct {
	MessageKey         string   `json:"msgKey"`
	MessageParam       string   `json:"msgParam"`
	RobotCode          string   `json:"robotCode"`
	UserIDs            []string `json:"userIds,omitempty"`
	OpenConversationID string   `json:"openConversationId,omitempty"`
}

func (c *Client) SendGroupMessage(ctx context.Context, request GroupMessageRequest, options ...socialhub.CallOption) (*RobotSendResult, error) {
	if c.robotCode == "" {
		return nil, unsupported("send_group_message", "account.settings.robot_code is required")
	}
	if !validOpaque(request.OpenConversationID, 512) {
		return nil, invalidArgument("send_group_message", "open conversation ID is invalid")
	}
	messageParam, err := validateRobotMessage("send_group_message", request.Message)
	if err != nil {
		return nil, err
	}
	payload := robotSendPayload{
		MessageKey: request.Message.Key, MessageParam: messageParam,
		RobotCode: c.robotCode, OpenConversationID: request.OpenConversationID,
	}
	return c.sendRobot(ctx, "send_group_message", "/v1.0/robot/groupMessages/send", payload, options...)
}

func (c *Client) BatchSendOTO(ctx context.Context, request BatchOTORequest, options ...socialhub.CallOption) (*RobotSendResult, error) {
	if c.robotCode == "" {
		return nil, unsupported("batch_send_oto", "account.settings.robot_code is required")
	}
	if len(request.UserIDs) == 0 || len(request.UserIDs) > 100 {
		return nil, invalidArgument("batch_send_oto", "user IDs must contain 1 to 100 entries")
	}
	seen := make(map[string]struct{}, len(request.UserIDs))
	for _, userID := range request.UserIDs {
		if !validOpaque(userID, 256) {
			return nil, invalidArgument("batch_send_oto", "user ID is invalid")
		}
		if _, exists := seen[userID]; exists {
			return nil, invalidArgument("batch_send_oto", "user IDs must be unique")
		}
		seen[userID] = struct{}{}
	}
	messageParam, err := validateRobotMessage("batch_send_oto", request.Message)
	if err != nil {
		return nil, err
	}
	payload := robotSendPayload{
		MessageKey: request.Message.Key, MessageParam: messageParam,
		RobotCode: c.robotCode, UserIDs: slices.Clone(request.UserIDs),
	}
	return c.sendRobot(ctx, "batch_send_oto", "/v1.0/robot/oToMessages/batchSend", payload, options...)
}

func (c *Client) sendRobot(ctx context.Context, operation, path string, payload robotSendPayload, options ...socialhub.CallOption) (*RobotSendResult, error) {
	var response robotSendResponse
	if err := c.call(ctx, operation, http.MethodPost, path, payload, &response, options...); err != nil {
		return nil, err
	}
	if err := c.responseError(ctx, operation, response.apiError); err != nil {
		return nil, err
	}
	if !validOpaque(response.ProcessQueryKey, 1024) {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	result := response.RobotSendResult
	result.FilteredStaffIDList = slices.Clone(result.FilteredStaffIDList)
	result.FlowControlledStaffIDList = slices.Clone(result.FlowControlledStaffIDList)
	result.InvalidStaffIDList = slices.Clone(result.InvalidStaffIDList)
	return &result, nil
}

func validateRobotMessage(operation string, message RobotMessage) (string, error) {
	if !validOpaque(message.Key, 128) {
		return "", invalidArgument(operation, "message key is invalid")
	}
	if len(message.Param) == 0 || len(message.Param) > 64<<10 || !json.Valid(message.Param) {
		return "", invalidArgument(operation, "message param must be a JSON object of at most 64 KiB")
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(message.Param, &object); err != nil || object == nil {
		return "", invalidArgument(operation, "message param must be a JSON object of at most 64 KiB")
	}
	compact, err := json.Marshal(object)
	if err != nil {
		return "", invalidArgument(operation, "message param is invalid")
	}
	return string(compact), nil
}

func (c *Client) RefreshAppToken(ctx context.Context) (socialhub.Token, error) {
	if c.tokenManager == nil {
		return socialhub.Token{}, unsupported("refresh_app_token", "access_token_ref is caller-managed")
	}
	c.tokenManager.Invalidate(ctx)
	return c.tokenManager.Token(ctx)
}
