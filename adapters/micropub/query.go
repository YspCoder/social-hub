package micropub

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) Config(ctx context.Context, options ...socialhub.CallOption) (*Config, error) {
	request, cancel, err := client.endpointRequest(ctx, http.MethodGet, url.Values{"q": {"config"}}, nil, options...)
	if err != nil {
		return nil, err
	}
	result, err := client.do(request, cancel)
	if err != nil {
		var hubError *socialhub.Error
		if result.Status == http.StatusBadRequest && errors.As(err, &hubError) {
			return &Config{}, nil
		}
		return nil, err
	}
	var wire configWire
	if len(result.Body) == 0 || json.Unmarshal(result.Body, &wire) != nil {
		return &Config{}, nil
	}
	if wire.MediaEndpoint != "" && !validEndpoint(wire.MediaEndpoint, true) || !validSyndicationTargets(wire.SyndicateTo) {
		return &Config{}, nil
	}
	return &Config{
		MediaEndpoint: wire.MediaEndpoint, SyndicateTo: wire.SyndicateTo,
		Raw: append(json.RawMessage(nil), result.Body...),
	}, nil
}

func (client *Client) SyndicationTargets(ctx context.Context, options ...socialhub.CallOption) ([]SyndicationTarget, error) {
	var wire syndicationWire
	_, err := client.queryJSON(ctx, url.Values{"q": {"syndicate-to"}}, &wire, options...)
	if err != nil {
		return nil, err
	}
	if !validSyndicationTargets(wire.SyndicateTo) {
		return nil, platformError("syndication_targets", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return wire.SyndicateTo, nil
}

func (client *Client) Source(ctx context.Context, postURL string, properties []string, options ...socialhub.CallOption) (*Entry, error) {
	if !client.supportsUpdate {
		return nil, unsupported("source", "endpoint is not configured with source-query support")
	}
	if err := client.requireScope("source", "update"); err != nil {
		return nil, err
	}
	if !validAbsoluteURL(postURL) {
		return nil, invalidArgument("source", "entry URL must be an absolute HTTP(S) URL")
	}
	query := url.Values{"q": {"source"}, "url": {postURL}}
	seen := make(map[string]struct{}, len(properties))
	for _, property := range properties {
		if !validPropertyName(property) {
			return nil, invalidArgument("source", "requested property name is invalid or reserved")
		}
		if _, exists := seen[property]; exists {
			return nil, invalidArgument("source", "requested property names must be unique")
		}
		seen[property] = struct{}{}
		query.Add("properties[]", property)
	}
	var wire struct {
		Type       []string                     `json:"type"`
		Properties map[string][]json.RawMessage `json:"properties"`
	}
	result, err := client.queryJSON(ctx, query, &wire, options...)
	if err != nil {
		return nil, err
	}
	if wire.Properties == nil || len(properties) == 0 && len(wire.Type) == 0 {
		return nil, platformError("source", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if err := validateSourceProperties(wire.Properties); err != nil {
		return nil, err
	}
	return &Entry{
		Type: append([]string(nil), wire.Type...), Properties: cloneProperties(wire.Properties),
		Raw: append(json.RawMessage(nil), result.Body...),
	}, nil
}

func validSyndicationTargets(values []SyndicationTarget) bool {
	if len(values) > maxValuesPerProperty {
		return false
	}
	seen := make(map[string]struct{}, len(values))
	for _, target := range values {
		if !validText(target.UID, false) || !validText(target.Name, false) {
			return false
		}
		if _, exists := seen[target.UID]; exists {
			return false
		}
		seen[target.UID] = struct{}{}
		for _, identity := range []*SyndicationIdentity{target.Service, target.User} {
			if identity == nil {
				continue
			}
			if !validText(identity.Name, false) || identity.URL != "" && !validAbsoluteURL(identity.URL) || identity.Photo != "" && !validAbsoluteURL(identity.Photo) {
				return false
			}
		}
	}
	return true
}

func validateSourceProperties(properties map[string][]json.RawMessage) error {
	if len(properties) > maxPropertyCount {
		return platformError("source", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	for name, values := range properties {
		if strings.TrimSpace(name) == "" || len(name) > 128 || len(values) > maxValuesPerProperty {
			return platformError("source", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		for _, value := range values {
			if len(value) == 0 || len(value) > maxTextBytes || !json.Valid(value) {
				return platformError("source", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
			}
		}
	}
	return nil
}
