package unsplash

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path string,
	query url.Values,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, nil, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, query, nil, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	meta := responseMeta(metadata.StatusCode, metadata.Header, 0, 0, client.accessKey)
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return meta, nil, platformContractError(operation, "Unsplash returned an empty or invalid JSON success response")
	}
	if client.accessKey != "" && bytes.Contains(trimmed, []byte(client.accessKey)) {
		return meta, nil, platformContractError(operation, "Unsplash returned credential material in a success response")
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return meta, append(json.RawMessage(nil), raw...), platformContractError(operation, "Unsplash returned a non-JSON success response")
	}
	if err := json.Unmarshal(trimmed, output); err != nil {
		return meta, append(json.RawMessage(nil), raw...), platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return meta, append(json.RawMessage(nil), raw...), nil
}

func responseMeta(status int, header http.Header, page, perPage int, redactions ...string) ResponseMeta {
	headerValue := func(name string, maximum int) string {
		value := header.Get(name)
		for _, secret := range redactions {
			value = redactExact(value, secret)
		}
		return boundedMessage(value, maximum)
	}
	link := headerValue("Link", 16_384)
	return ResponseMeta{
		StatusCode:         status,
		RequestID:          headerValue("X-Request-ID", 256),
		RateLimitLimit:     headerValue("X-Ratelimit-Limit", 64),
		RateLimitRemaining: headerValue("X-Ratelimit-Remaining", 64),
		Total:              headerValue("X-Total", 64),
		Link:               link,
		Warning:            headerValue("Warning", 1024),
		Page:               page,
		PerPage:            perPage,
		NextPage:           pageFromLink(link, "next"),
		PreviousPage:       pageFromLink(link, "prev"),
	}
}

func pageFromLink(value, expectedRelation string) *int {
	for _, link := range splitLinkHeader(value) {
		start := strings.IndexByte(link, '<')
		end := strings.IndexByte(link, '>')
		if start < 0 || end <= start || !hasRelation(link[end+1:], expectedRelation) {
			continue
		}
		target, err := url.Parse(strings.TrimSpace(link[start+1 : end]))
		if err != nil || target.User != nil {
			continue
		}
		pages, found := target.Query()["page"]
		if !found || len(pages) != 1 {
			continue
		}
		page, err := strconv.Atoi(pages[0])
		if err != nil || page < 1 || page > maxPageNumber {
			continue
		}
		return &page
	}
	return nil
}

func splitLinkHeader(value string) []string {
	var links []string
	start, angleDepth := 0, 0
	inQuote, escaped := false, false
	for index, character := range value {
		if escaped {
			escaped = false
			continue
		}
		if inQuote && character == '\\' {
			escaped = true
			continue
		}
		if character == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch character {
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case ',':
			if angleDepth == 0 {
				links = append(links, strings.TrimSpace(value[start:index]))
				start = index + 1
			}
		}
	}
	if tail := strings.TrimSpace(value[start:]); tail != "" {
		links = append(links, tail)
	}
	return links
}

func hasRelation(parameters, expected string) bool {
	for _, parameter := range strings.Split(parameters, ";") {
		name, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "rel") {
			continue
		}
		for _, relation := range strings.Fields(strings.Trim(strings.TrimSpace(raw), "\"")) {
			if strings.EqualFold(relation, expected) {
				return true
			}
		}
	}
	return false
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
