package yahoodisplayads

import (
	"bytes"
	"context"
	"encoding/json"
	"mime"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 1 << 20

type responseEnvelope struct {
	Errors []ErrorItem     `json:"errors"`
	RID    string          `json:"rid"`
	RVal   json.RawMessage `json:"rval"`
}

type rawValue struct {
	OperationSucceeded *bool           `json:"operationSucceeded"`
	Errors             []ErrorItem     `json:"errors"`
	Campaign           json.RawMessage `json:"campaign"`
	AdGroup            json.RawMessage `json:"adGroup"`
	AdGroupAd          json.RawMessage `json:"adGroupAd"`
	ReportDefinition   json.RawMessage `json:"reportDefinition"`
}

type pageWire struct {
	TotalNumEntries int32      `json:"totalNumEntries"`
	Values          []rawValue `json:"values"`
}

type mutationWire struct {
	Values []rawValue `json:"values"`
}

func (client *Client) postJSON(ctx context.Context, operation, path string, input, output any, mutation bool, options ...socialhub.CallOption) (string, error) {
	if err := client.requireAccess(operation); err != nil {
		return "", err
	}
	prepared, err := prepareCallOptions(operation, options)
	if err != nil {
		return "", err
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return "", platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if len(encoded) > maxRequestBytes {
		return "", invalidArgument(operation, "request JSON exceeds 1 MiB")
	}
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, bytes.NewReader(encoded), prepared...)
	if err != nil {
		return "", withOperation(err, operation)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Accept-Encoding", "identity")
	request.Header.Set("Content-Type", "application/json; charset=utf-8")
	var envelope responseEnvelope
	metadata, err := client.api.DoWithMetadata(request, &envelope)
	if err != nil {
		err = withOperation(err, operation)
		if mutation && ambiguousMutationError(err) {
			return "", outcomeUnknownError(operation, err, "")
		}
		return "", err
	}
	if metadata.StatusCode != http.StatusOK {
		contractErr := platformContractError(operation, "LINE Yahoo returned an unexpected successful HTTP status")
		if mutation {
			return "", outcomeUnknownError(operation, contractErr, client.requestIDs.safe(envelope.RID))
		}
		return "", contractErr
	}
	if !validJSONMediaType(metadata.Header.Get("Content-Type")) {
		contractErr := platformContractError(operation, "LINE Yahoo returned an unexpected successful content type")
		if mutation {
			return "", outcomeUnknownError(operation, contractErr, client.requestIDs.safe(envelope.RID))
		}
		return "", contractErr
	}
	if !validOpaque(envelope.RID, 256) {
		contractErr := platformContractError(operation, "LINE Yahoo response omitted a valid rid")
		if mutation {
			return "", outcomeUnknownError(operation, contractErr, "")
		}
		return "", contractErr
	}
	rid := client.requestIDs.safe(envelope.RID)
	if rid == "" {
		contractErr := platformContractError(operation, "LINE Yahoo response omitted a safe rid")
		if mutation {
			return "", outcomeUnknownError(operation, contractErr, "")
		}
		return "", contractErr
	}
	if !validErrorItems(envelope.Errors) {
		contractErr := platformContractError(operation, "LINE Yahoo returned malformed top-level errors")
		if mutation {
			return rid, outcomeUnknownError(operation, contractErr, rid)
		}
		return rid, contractErr
	}
	if len(envelope.Errors) > 0 {
		return rid, client.apiErrorValue(operation, metadata.StatusCode, metadata.Header, rid, envelope.Errors)
	}
	if !rawPresent(envelope.RVal) {
		contractErr := platformContractError(operation, "LINE Yahoo response omitted rval")
		if mutation {
			return rid, outcomeUnknownError(operation, contractErr, rid)
		}
		return rid, contractErr
	}
	if output != nil {
		if err := json.Unmarshal(envelope.RVal, output); err != nil {
			contractErr := platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
			if mutation {
				return rid, outcomeUnknownError(operation, contractErr, rid)
			}
			return rid, contractErr
		}
	}
	return rid, nil
}

func postPage[T any](ctx context.Context, client *Client, operation, path string, input any, pageRequest PageRequest, maximum int32, entity func(rawValue) json.RawMessage, validate func(*T) error, options ...socialhub.CallOption) (Page[T], error) {
	var wire pageWire
	rid, err := client.postJSON(ctx, operation, path, input, &wire, false, options...)
	if err != nil {
		return Page[T]{}, err
	}
	page := normalizedPage(pageRequest)
	if page.NumberResults > maximum || wire.TotalNumEntries < 0 || len(wire.Values) > int(page.NumberResults) {
		return Page[T]{}, platformContractError(operation, "LINE Yahoo returned invalid pagination metadata")
	}
	items := make([]T, 0, len(wire.Values))
	for _, value := range wire.Values {
		if value.OperationSucceeded == nil || !validErrorItems(value.Errors) {
			return Page[T]{}, platformContractError(operation, "LINE Yahoo returned malformed per-item read status")
		}
		if !*value.OperationSucceeded {
			if len(value.Errors) == 0 {
				return Page[T]{}, platformContractError(operation, "LINE Yahoo returned a failed read without errors")
			}
			return Page[T]{}, client.apiErrorValue(operation, http.StatusOK, nil, rid, value.Errors)
		}
		if len(value.Errors) != 0 || !rawPresent(entity(value)) {
			return Page[T]{}, platformContractError(operation, "LINE Yahoo returned an incomplete successful read item")
		}
		var item T
		if err := json.Unmarshal(entity(value), &item); err != nil {
			return Page[T]{}, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		if err := validate(&item); err != nil {
			return Page[T]{}, err
		}
		items = append(items, item)
	}
	if len(items) > 0 && int64(page.StartIndex)+int64(len(items))-1 > int64(wire.TotalNumEntries) {
		return Page[T]{}, platformContractError(operation, "LINE Yahoo page extends beyond totalNumEntries")
	}
	return Page[T]{Items: items, TotalNumEntries: wire.TotalNumEntries, StartIndex: page.StartIndex, NumberResults: page.NumberResults, RID: rid}, nil
}

func postMutation[T any](ctx context.Context, client *Client, operation, path string, input any, expected int, entity func(rawValue) json.RawMessage, validate func(*T) error, options ...socialhub.CallOption) (MutationResult[T], error) {
	var wire mutationWire
	rid, err := client.postJSON(ctx, operation, path, input, &wire, true, options...)
	if err != nil {
		return MutationResult[T]{RID: rid}, err
	}
	result := MutationResult[T]{Items: make([]MutationItem[T], 0, len(wire.Values)), RID: rid}
	if len(wire.Values) != expected {
		return result, outcomeUnknownError(operation, platformContractError(operation, "LINE Yahoo returned an unexpected number of mutation results"), rid)
	}
	failures := 0
	var firstFailure error
	for _, value := range wire.Values {
		if value.OperationSucceeded == nil || !validErrorItems(value.Errors) {
			return result, outcomeUnknownError(operation, platformContractError(operation, "LINE Yahoo returned malformed mutation status"), rid)
		}
		item := MutationItem[T]{Succeeded: *value.OperationSucceeded, Errors: sanitizeErrorItems(value.Errors)}
		if *value.OperationSucceeded {
			if len(value.Errors) != 0 || !rawPresent(entity(value)) {
				return result, outcomeUnknownError(operation, platformContractError(operation, "LINE Yahoo returned an incomplete successful mutation item"), rid)
			}
			var decoded T
			if err := json.Unmarshal(entity(value), &decoded); err != nil {
				return result, outcomeUnknownError(operation, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err), rid)
			}
			if err := validate(&decoded); err != nil {
				return result, outcomeUnknownError(operation, err, rid)
			}
			item.Value = &decoded
		} else {
			failures++
			if len(value.Errors) == 0 {
				return result, outcomeUnknownError(operation, platformContractError(operation, "LINE Yahoo returned a failed mutation without errors"), rid)
			}
			if firstFailure == nil {
				firstFailure = client.apiErrorValue(operation, http.StatusOK, nil, rid, value.Errors)
			}
		}
		result.Items = append(result.Items, item)
	}
	if failures == 0 {
		return result, nil
	}
	if failures < len(result.Items) {
		return result, partialMutationError(operation, rid, firstFailure)
	}
	return result, firstFailure
}

func rawPresent(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func validJSONMediaType(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.EqualFold(mediaType, "application/json")
}

func campaignEntity(value rawValue) json.RawMessage { return value.Campaign }
func adGroupEntity(value rawValue) json.RawMessage  { return value.AdGroup }
func adEntity(value rawValue) json.RawMessage       { return value.AdGroupAd }
func reportEntity(value rawValue) json.RawMessage   { return value.ReportDefinition }
