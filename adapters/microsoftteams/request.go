package microsoftteams

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) call(ctx context.Context, operation, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return unsupported(operation, "field selection is not exposed for this Graph operation")
	}
	if resolved.IdempotencyKey != "" {
		return unsupported(operation, "Microsoft Graph does not document an idempotency-key contract for this operation")
	}
	err = c.api.JSON(ctx, method, path, query, input, output, cleanCallOptions(resolved)...)
	if err != nil {
		return operationError(err, operation)
	}
	return nil
}

func cleanCallOptions(resolved socialhub.CallOptions) []socialhub.CallOption {
	options := make([]socialhub.CallOption, 0, 2)
	if resolved.RequestID != "" {
		options = append(options, socialhub.WithRequestID(resolved.RequestID))
	}
	if resolved.Timeout > 0 {
		options = append(options, socialhub.WithCallTimeout(resolved.Timeout))
	}
	return options
}

func (c *Client) get(ctx context.Context, operation, path string, query url.Values, output any, options ...socialhub.CallOption) error {
	return c.call(ctx, operation, http.MethodGet, path, query, nil, output, options...)
}

func (c *Client) post(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return c.call(ctx, operation, http.MethodPost, path, nil, input, output, options...)
}

func (c *Client) patch(ctx context.Context, operation, path string, input, output any, options ...socialhub.CallOption) error {
	return c.call(ctx, operation, http.MethodPatch, path, nil, input, output, options...)
}

func (c *Client) delete(ctx context.Context, operation, path string, options ...socialhub.CallOption) error {
	return c.call(ctx, operation, http.MethodDelete, path, nil, nil, nil, options...)
}

func targetCollectionPath(target Target) string {
	if target.Kind == TargetChat {
		return "chats/" + target.ChatID + "/messages"
	}
	return "teams/" + target.TeamID + "/channels/" + target.ChannelID + "/messages"
}

func messagePath(ref MessageRef) string {
	path := targetCollectionPath(ref.Target) + "/" + ref.RootID
	if ref.ReplyID != "" {
		path += "/replies/" + ref.ReplyID
	}
	return path
}

func repliesPath(ref MessageRef) string {
	return targetCollectionPath(ref.Target) + "/" + ref.RootID + "/replies"
}

func (c *Client) pageRequest(cursor, defaultPath string, defaultQuery url.Values) (string, url.Values, error) {
	if cursor == "" {
		return defaultPath, defaultQuery, nil
	}
	if len(cursor) > 16<<10 {
		return "", nil, invalidArgument("list_messages", "cursor exceeds the supported size")
	}
	next, err := url.Parse(cursor)
	if err != nil || !next.IsAbs() || next.User != nil || next.Fragment != "" {
		return "", nil, invalidArgument("list_messages", "cursor must be a Graph @odata.nextLink")
	}
	if c.baseURL == nil || !strings.EqualFold(next.Scheme, c.baseURL.Scheme) || !strings.EqualFold(next.Host, c.baseURL.Host) {
		return "", nil, invalidArgument("list_messages", "cursor origin does not match the configured Graph cloud")
	}
	basePath := strings.TrimRight(c.baseURL.Path, "/")
	if next.Path != basePath && !strings.HasPrefix(next.Path, basePath+"/") {
		return "", nil, invalidArgument("list_messages", "cursor path is outside the configured Graph API root")
	}
	path := strings.TrimPrefix(next.Path, basePath)
	return strings.TrimLeft(path, "/"), next.Query(), nil
}
