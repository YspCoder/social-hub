package listenbrainz

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type listenEnvelope struct {
	Payload ListenPage `json:"payload"`
}

type listenCountEnvelope struct {
	Payload struct {
		Count int64 `json:"count"`
	} `json:"payload"`
}

type usersEnvelope struct {
	Users []User `json:"users"`
}

type submitListensRequest struct {
	ListenType string             `json:"listen_type"`
	Payload    []ListenSubmission `json:"payload"`
}

type submitPlayingNowRequest struct {
	ListenType string                 `json:"listen_type"`
	Payload    []PlayingNowSubmission `json:"payload"`
}

func (c *Client) ValidateToken(ctx context.Context, options ...socialhub.CallOption) (*TokenValidation, error) {
	const operation = "validate_token"
	if err := c.requireToken(operation); err != nil {
		return nil, err
	}
	var result TokenValidation
	if err := getOnly(ctx, c, operation, "/1/validate-token", nil, &result, options...); err != nil {
		return nil, err
	}
	if result.Code != http.StatusOK || (result.Valid && !validUsername(result.UserName)) {
		return nil, invalidPlatformResponse(operation, "response contained invalid token metadata")
	}
	return &result, nil
}

func (c *Client) SearchUsers(ctx context.Context, searchTerm string, options ...socialhub.CallOption) ([]User, error) {
	const operation = "search_users"
	if !validText(searchTerm, 255) {
		return nil, invalidArgument(operation, "search term must be a nonempty bounded value")
	}
	var envelope usersEnvelope
	query := url.Values{"search_term": {searchTerm}}
	if err := getOnly(ctx, c, operation, "/1/search/users/", query, &envelope, options...); err != nil {
		return nil, err
	}
	for _, user := range envelope.Users {
		if !validUsername(user.UserName) {
			return nil, invalidPlatformResponse(operation, "response contained an invalid username")
		}
	}
	return envelope.Users, nil
}

func (c *Client) ListListens(ctx context.Context, request ListensRequest, options ...socialhub.CallOption) (*ListenPage, error) {
	const operation = "list_listens"
	username, err := c.resolveUsername(operation, request.Username)
	if err != nil {
		return nil, err
	}
	if request.MinTimestamp < 0 || request.MaxTimestamp < 0 || (request.MinTimestamp > 0 && request.MaxTimestamp > 0) {
		return nil, invalidArgument(operation, "min_timestamp and max_timestamp are exclusive positive Unix timestamps")
	}
	if err := validatePage(request.Count, maxListenPageSize); err != nil {
		return nil, err
	}
	query := make(url.Values)
	if request.MinTimestamp > 0 {
		query.Set("min_ts", strconv.FormatInt(request.MinTimestamp, 10))
	}
	if request.MaxTimestamp > 0 {
		query.Set("max_ts", strconv.FormatInt(request.MaxTimestamp, 10))
	}
	if request.Count > 0 {
		query.Set("count", strconv.Itoa(request.Count))
	}
	var envelope listenEnvelope
	path := "/1/user/" + url.PathEscape(username) + "/listens"
	if err := getOnly(ctx, c, operation, path, query, &envelope, options...); err != nil {
		return nil, err
	}
	if envelope.Payload.Count != len(envelope.Payload.Listens) || envelope.Payload.Count > maxListenPageSize ||
		envelope.Payload.UserID != username {
		return nil, invalidPlatformResponse(operation, "response contained invalid listen metadata")
	}
	for _, listen := range envelope.Payload.Listens {
		if listen.ListenedAt == nil {
			return nil, invalidPlatformResponse(operation, "stored listen omitted listened_at")
		}
	}
	return &envelope.Payload, nil
}

func (c *Client) GetPlayingNow(ctx context.Context, requestedUsername string, options ...socialhub.CallOption) (*Listen, error) {
	const operation = "get_playing_now"
	username, err := c.resolveUsername(operation, requestedUsername)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Payload struct {
			Count      int      `json:"count"`
			Listens    []Listen `json:"listens"`
			PlayingNow bool     `json:"playing_now"`
			UserID     string   `json:"user_id"`
		} `json:"payload"`
	}
	path := "/1/user/" + url.PathEscape(username) + "/playing-now"
	if err := getOnly(ctx, c, operation, path, nil, &envelope, options...); err != nil {
		return nil, err
	}
	payload := envelope.Payload
	if !payload.PlayingNow || payload.UserID != username || payload.Count != len(payload.Listens) || payload.Count < 0 || payload.Count > 1 {
		return nil, invalidPlatformResponse(operation, "response contained invalid playing-now metadata")
	}
	if payload.Count == 0 {
		return nil, nil
	}
	if payload.Listens[0].ListenedAt != nil {
		return nil, invalidPlatformResponse(operation, "playing-now listen unexpectedly included listened_at")
	}
	return &payload.Listens[0], nil
}

func (c *Client) GetListenCount(ctx context.Context, requestedUsername string, options ...socialhub.CallOption) (int64, error) {
	const operation = "get_listen_count"
	username, err := c.resolveUsername(operation, requestedUsername)
	if err != nil {
		return 0, err
	}
	var envelope listenCountEnvelope
	path := "/1/user/" + url.PathEscape(username) + "/listen-count"
	if err := getOnly(ctx, c, operation, path, nil, &envelope, options...); err != nil {
		return 0, err
	}
	if envelope.Payload.Count < 0 {
		return 0, invalidPlatformResponse(operation, "response contained a negative listen count")
	}
	return envelope.Payload.Count, nil
}

func (c *Client) SubmitSingle(ctx context.Context, listen ListenSubmission, options ...socialhub.CallOption) (*SubmissionResult, error) {
	const operation = "submit_single"
	if err := validateListen(listen); err != nil {
		return nil, err
	}
	request := submitListensRequest{ListenType: "single", Payload: []ListenSubmission{listen}}
	return c.submitListens(ctx, operation, request, false, options...)
}

func (c *Client) SubmitImport(ctx context.Context, listens []ListenSubmission, options ...socialhub.CallOption) (*SubmissionResult, error) {
	const operation = "submit_import"
	if len(listens) == 0 || len(listens) > maxListensPerImport {
		return nil, invalidArgument(operation, "import payload must contain between 1 and 1000 listens")
	}
	for _, listen := range listens {
		if err := validateListen(listen); err != nil {
			return nil, err
		}
	}
	request := submitListensRequest{ListenType: "import", Payload: listens}
	return c.submitListens(ctx, operation, request, false, options...)
}

func (c *Client) submitListens(ctx context.Context, operation string, request any, returnMSID bool, options ...socialhub.CallOption) (*SubmissionResult, error) {
	if err := validatePayload(request); err != nil {
		return nil, err
	}
	if err := c.requireToken(operation); err != nil {
		return nil, err
	}
	query := make(url.Values)
	if returnMSID {
		query.Set("return_msid", "true")
	}
	var result SubmissionResult
	if err := c.requestJSON(ctx, operation, http.MethodPost, "/1/submit-listens", query, request, &result, options...); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SubmitPlayingNow(ctx context.Context, listen PlayingNowSubmission, returnMSID bool, options ...socialhub.CallOption) (*SubmissionResult, error) {
	const operation = "submit_playing_now"
	if err := validateTrackMetadata(listen.TrackMetadata); err != nil {
		return nil, err
	}
	request := submitPlayingNowRequest{ListenType: "playing_now", Payload: []PlayingNowSubmission{listen}}
	return c.submitListens(ctx, operation, request, returnMSID, options...)
}

func (c *Client) DeleteListen(ctx context.Context, input DeleteListenRequest, options ...socialhub.CallOption) error {
	const operation = "delete_listen"
	if input.ListenedAt <= 0 || !validMBID(input.RecordingMSID) {
		return invalidArgument(operation, "listened_at and a canonical recording_msid are required")
	}
	if err := c.requireToken(operation); err != nil {
		return err
	}
	return c.requestJSON(ctx, operation, http.MethodPost, "/1/delete-listen", nil, input, nil, options...)
}
