package bitbucket

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

var errUnsafeContinuation = errors.New("bitbucket: unsafe continuation query")

type pageEnvelope struct {
	Size       *int            `json:"size"`
	Page       *int            `json:"page"`
	PageLength *int            `json:"pagelen"`
	Next       string          `json:"next"`
	Previous   string          `json:"previous"`
	Values     json.RawMessage `json:"values"`
}

func decodePage[T any](
	operation string,
	path string,
	meta ResponseMeta,
	raw json.RawMessage,
	valid func(T) bool,
	entityName string,
) (*Page[T], error) {
	var envelope pageEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, platformContractError(operation, "Bitbucket returned an invalid pagination envelope")
	}
	values := bytes.TrimSpace(envelope.Values)
	if len(values) == 0 || values[0] != '[' || !json.Valid(values) {
		return nil, platformContractError(operation, "Bitbucket pagination omitted the values array")
	}
	if envelope.Size != nil && *envelope.Size < 0 || envelope.Page != nil && *envelope.Page < 1 ||
		envelope.PageLength != nil && (*envelope.PageLength < MinimumPageLength || *envelope.PageLength > MaximumPageLength) {
		return nil, platformContractError(operation, "Bitbucket returned invalid pagination counters")
	}
	var items []T
	if err := json.Unmarshal(values, &items); err != nil || len(items) > MaximumPageLength {
		return nil, platformContractError(operation, "Bitbucket returned invalid "+entityName+" values")
	}
	for _, item := range items {
		if !valid(item) {
			return nil, platformContractError(operation, "Bitbucket returned a "+entityName+" without its required identifier")
		}
	}
	nextQuery, err := continuationQuery(envelope.Next, path)
	if err != nil {
		return nil, platformContractError(operation, "Bitbucket returned an unsafe next-page URL")
	}
	previousQuery, err := continuationQuery(envelope.Previous, path)
	if err != nil {
		return nil, platformContractError(operation, "Bitbucket returned an unsafe previous-page URL")
	}
	return &Page[T]{
		Items: items, Size: envelope.Size, PageNumber: envelope.Page, PageLength: envelope.PageLength,
		NextURL: boundedMessage(envelope.Next, 16_384), PreviousURL: boundedMessage(envelope.Previous, 16_384),
		NextQuery: nextQuery, PreviousQuery: previousQuery, Meta: meta,
		Raw: append(json.RawMessage(nil), raw...),
	}, nil
}

func continuationQuery(value, requestPath string) (string, error) {
	if value == "" {
		return "", nil
	}
	if len(value) > 16_384 {
		return "", errUnsafeContinuation
	}
	target, err := url.Parse(value)
	if err != nil || target.Scheme != "https" || !strings.EqualFold(target.Hostname(), "api.bitbucket.org") ||
		target.Port() != "" || target.User != nil || target.Fragment != "" || target.Path != "/2.0"+requestPath {
		return "", errUnsafeContinuation
	}
	if !validContinuationQuery(target.RawQuery) {
		return "", errUnsafeContinuation
	}
	return target.RawQuery, nil
}

func pageQuery(options PageOptions) (url.Values, error) {
	if !validPageOptions(options) {
		return nil, errUnsafeContinuation
	}
	if options.NextQuery != "" {
		return url.ParseQuery(options.NextQuery)
	}
	query := make(url.Values)
	if options.PageLength > 0 {
		query.Set("pagelen", strconv.Itoa(options.PageLength))
	}
	return query, nil
}
