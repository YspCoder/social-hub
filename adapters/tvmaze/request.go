package tvmaze

import (
	"bytes"
	"context"
	"net/http"
	"net/url"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func requestJSON(ctx context.Context, client *transport.Client, operation, path string, query url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	if len(resolved.Fields) > 0 {
		return transport.ResponseMetadata{}, invalidArgument(operation, "field selection is fixed by the typed TVmaze operation")
	}
	if resolved.IdempotencyKey != "" {
		return transport.ResponseMetadata{}, invalidArgument(operation, "TVmaze does not document an idempotency-key contract")
	}
	request, err := client.NewRequest(ctx, http.MethodGet, path, query, bytes.NewReader(nil), options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	metadata, err := client.DoWithMetadata(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
	}
	return metadata, err
}
