package etsy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	maxRequestBytes                = 1 << 20
	maxImageBytes            int64 = 25 << 20
	maxMultipartRequestBytes       = 26 << 20
)

func (client *Client) doJSON(
	ctx context.Context,
	operation string,
	method string,
	path string,
	query url.Values,
	input any,
	output any,
	expectedStatus int,
	mutation bool,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	var body io.Reader
	contentType := ""
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return ResponseMeta{}, nil, invalidArgument(operation, "request JSON exceeds 1 MiB")
		}
		body, contentType = bytes.NewReader(encoded), "application/json"
	}
	return client.do(ctx, operation, method, path, query, body, contentType, output, expectedStatus, mutation, options...)
}

func (client *Client) doForm(
	ctx context.Context,
	operation string,
	path string,
	form url.Values,
	output any,
	expectedStatus int,
	mutation bool,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	encoded := form.Encode()
	if len(encoded) > maxRequestBytes {
		return ResponseMeta{}, nil, invalidArgument(operation, "form request exceeds 1 MiB")
	}
	return client.do(
		ctx, operation, http.MethodPost, path, nil, strings.NewReader(encoded),
		"application/x-www-form-urlencoded; charset=utf-8", output, expectedStatus, mutation, options...,
	)
}

func (client *Client) doMultipart(
	ctx context.Context,
	operation string,
	path string,
	input UploadListingImageRequest,
	form url.Values,
	output any,
	expectedStatus int,
	mutation bool,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if input.Image != nil {
		image, err := io.ReadAll(io.LimitReader(input.Image, maxImageBytes+1))
		if err != nil {
			return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if int64(len(image)) > maxImageBytes {
			return ResponseMeta{}, nil, invalidArgument(operation, "image exceeds the SDK 25 MiB safety limit")
		}
		part, err := writer.CreateFormFile("image", input.FileName)
		if err != nil {
			return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if _, err := part.Write(image); err != nil {
			return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
	}
	for key, values := range form {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
			}
		}
	}
	if err := writer.Close(); err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if body.Len() > maxMultipartRequestBytes {
		return ResponseMeta{}, nil, invalidArgument(operation, "multipart request exceeds the SDK 26 MiB safety limit")
	}
	return client.do(
		ctx, operation, http.MethodPost, path, nil, &body, writer.FormDataContentType(),
		output, expectedStatus, mutation, options...,
	)
}

func (client *Client) do(
	ctx context.Context,
	operation string,
	method string,
	path string,
	query url.Values,
	body io.Reader,
	contentType string,
	output any,
	expectedStatus int,
	mutation bool,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return ResponseMeta{}, nil, err
	}
	request, err := client.api.NewRequest(ctx, method, path, query, body, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	responseMeta := ResponseMeta{
		RequestID:           boundedMessage(firstNonEmpty(firstHeader(metadata.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID), 256),
		LimitPerSecond:      headerInteger(metadata.Header, "X-Limit-Per-Second"),
		RemainingThisSecond: headerInteger(metadata.Header, "X-Remaining-This-Second", "X-Remaining-This-Secon"),
		LimitPerDay:         headerInteger(metadata.Header, "X-Limit-Per-Day"),
		RemainingToday:      headerInteger(metadata.Header, "X-Remaining-Today"),
	}
	rawCopy := append(json.RawMessage(nil), raw...)
	if err != nil {
		failure := withOperation(err, operation)
		return responseMeta, rawCopy, withMutationOutcome(operation, mutation, responseMeta.RequestID, failure)
	}
	if metadata.StatusCode != expectedStatus {
		failure := platformContractError(operation, "Etsy returned an unexpected successful HTTP status")
		return responseMeta, rawCopy, withMutationOutcome(operation, mutation, responseMeta.RequestID, failure)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		failure := platformContractError(operation, "Etsy returned an empty or invalid JSON success response")
		return responseMeta, rawCopy, withMutationOutcome(operation, mutation, responseMeta.RequestID, failure)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		failure := platformContractError(operation, "Etsy returned a non-JSON success response")
		return responseMeta, rawCopy, withMutationOutcome(operation, mutation, responseMeta.RequestID, failure)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		failure := platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		return responseMeta, rawCopy, withMutationOutcome(operation, mutation, responseMeta.RequestID, failure)
	}
	return responseMeta, rawCopy, nil
}

func headerInteger(header http.Header, names ...string) *int64 {
	value := firstHeader(header, names...)
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || parsed < 0 {
		return nil
	}
	return &parsed
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}

func unexpectedEmptyID(operation, resource string) error {
	return platformContractError(operation, fmt.Sprintf("Etsy omitted the %s identifier", resource))
}
