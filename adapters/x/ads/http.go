package ads

import (
	"context"
	"net/http"
	"net/url"

	"github.com/dghubble/oauth1"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) get(ctx context.Context, path string, query url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.api.DoWithMetadata(request, output)
}

func (client *Client) write(ctx context.Context, method, path string, query url.Values, output any, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	request, err := client.api.NewRequest(ctx, method, path, query, nil, options...)
	if err != nil {
		return transport.ResponseMetadata{}, err
	}
	return client.api.DoWithMetadata(request, output)
}

func signedHTTPClient(base *http.Client, config *oauth1.Config, token *oauth1.Token) *http.Client {
	baseTransport := base.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	baseForOAuth := &http.Client{Transport: baseTransport}
	ctx := context.WithValue(context.Background(), oauth1.HTTPClient, baseForOAuth)
	signed := config.Client(ctx, token)
	signed.Timeout = base.Timeout
	signed.Jar = base.Jar
	signed.CheckRedirect = rejectRedirect
	return signed
}

func listQuery(cursor string, count int) url.Values {
	query := make(url.Values)
	if cursor != "" {
		query.Set("cursor", cursor)
	}
	if count > 0 {
		query.Set("count", formatInt(count))
	}
	return query
}
