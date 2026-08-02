package kitsu

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const jsonAPIContentType = "application/vnd.api+json"

type resourceIdentifier struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type relationship struct {
	Data json.RawMessage `json:"data"`
}

type resource struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id"`
	Attributes    json.RawMessage         `json:"attributes"`
	Relationships map[string]relationship `json:"relationships,omitempty"`
}

type links struct {
	First json.RawMessage `json:"first"`
	Prev  json.RawMessage `json:"prev"`
	Next  json.RawMessage `json:"next"`
	Last  json.RawMessage `json:"last"`
}

type collectionDocument struct {
	Data     []resource `json:"data"`
	Included []resource `json:"included,omitempty"`
	Links    links      `json:"links"`
}

type resourceDocument struct {
	Data     resource   `json:"data"`
	Included []resource `json:"included,omitempty"`
}

type mutationDocument struct {
	Data mutationResource `json:"data"`
}

type mutationResource struct {
	Type          string                  `json:"type"`
	ID            string                  `json:"id,omitempty"`
	Attributes    any                     `json:"attributes,omitempty"`
	Relationships map[string]relationship `json:"relationships,omitempty"`
}

func (c *Client) request(ctx context.Context, operation, method, path string, query url.Values, input, output any, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed JSON:API operation")
	}
	var body *bytes.Reader
	if input == nil {
		body = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		body = bytes.NewReader(encoded)
	}
	request, err := c.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", jsonAPIContentType)
	if input != nil {
		request.Header.Set("Content-Type", jsonAPIContentType)
	}
	err = c.api.Do(request, output)
	if platformErr, ok := err.(*socialhub.Error); ok {
		platformErr.Op = operation
		platformErr.Cause = sanitizeTransportError(platformErr.Cause)
	}
	return err
}

func pagination(cursor string, limit int) (int, url.Values, error) {
	offset, valid := validPage(cursor, limit)
	if !valid {
		return 0, nil, invalidArgument("pagination", "cursor or limit is invalid")
	}
	if limit == 0 {
		limit = maxPageSize
	}
	query := url.Values{"page[limit]": {strconv.Itoa(limit)}, "page[offset]": {strconv.Itoa(offset)}}
	return offset, query, nil
}

func toPage[T any](items []T, responseLinks links, offset, limit int) (socialhub.Page[T], error) {
	result := socialhub.Page[T]{Items: items}
	nextOffset, hasNext, err := linkOffset(responseLinks.Next)
	if err != nil {
		return socialhub.Page[T]{}, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if hasNext {
		next := strconv.Itoa(nextOffset)
		result.NextCursor, result.HasMore = &next, true
	}
	if offset > 0 {
		previous := offset - limit
		if previous < 0 {
			previous = 0
		}
		value := strconv.Itoa(previous)
		result.PrevCursor = &value
	}
	return result, nil
}

func linkOffset(raw json.RawMessage) (int, bool, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, false, nil
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0, false, err
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return 0, false, err
	}
	offset := parsed.Query().Get("page[offset]")
	if offset == "" {
		return 0, false, errors.New("next link has no page[offset]")
	}
	number, err := strconv.Atoi(offset)
	if err != nil || number < 0 || number > maxOffset || strconv.Itoa(number) != offset {
		return 0, false, errors.New("next link has invalid page[offset]")
	}
	return number, true, nil
}

func relationshipID(source resource, name string) string {
	return relationshipIdentifier(source, name).ID
}

func relationshipIdentifier(source resource, name string) resourceIdentifier {
	relation, ok := source.Relationships[name]
	if !ok || len(relation.Data) == 0 || string(relation.Data) == "null" {
		return resourceIdentifier{}
	}
	var identifier resourceIdentifier
	if json.Unmarshal(relation.Data, &identifier) != nil {
		return resourceIdentifier{}
	}
	return identifier
}

func includedIndex(resources []resource) map[string]resource {
	index := make(map[string]resource, len(resources))
	for _, item := range resources {
		index[item.Type+":"+item.ID] = item
	}
	return index
}

func sanitizeTransportError(err error) error {
	for err != nil {
		var urlError *url.Error
		if !errors.As(err, &urlError) || urlError.Err == nil {
			return err
		}
		err = urlError.Err
	}
	return nil
}
