package tradedoubler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type matrixPath struct {
	decoded string
	escaped string
}

type matrixParameter struct {
	name   string
	values []string
}

func buildMatrixPath(base string, parameters ...matrixParameter) (matrixPath, error) {
	var decoded strings.Builder
	var escaped strings.Builder
	decoded.WriteString(base)
	escaped.WriteString(base)
	for _, parameter := range parameters {
		if len(parameter.values) == 0 {
			continue
		}
		if parameter.name == "" {
			return matrixPath{}, fmt.Errorf("tradedoubler: matrix parameter name is required")
		}
		decoded.WriteByte(';')
		decoded.WriteString(parameter.name)
		decoded.WriteByte('=')
		escaped.WriteByte(';')
		escaped.WriteString(parameter.name)
		escaped.WriteByte('=')
		for index, value := range parameter.values {
			if index > 0 {
				decoded.WriteByte(',')
				escaped.WriteByte(',')
			}
			decoded.WriteString(value)
			escaped.WriteString(url.PathEscape(value))
		}
	}
	if decoded.Len() > maximumMatrixPathBytes || escaped.Len() > maximumMatrixPathBytes {
		return matrixPath{}, fmt.Errorf("tradedoubler: matrix path exceeds 8 KiB")
	}
	return matrixPath{decoded: decoded.String(), escaped: escaped.String()}, nil
}

func applyEscapedPath(request *http.Request, path matrixPath) error {
	suffix := "/" + path.decoded
	if request == nil || request.URL == nil || !strings.HasSuffix(request.URL.Path, suffix) {
		return fmt.Errorf("tradedoubler: request path does not match matrix path")
	}
	prefix := strings.TrimSuffix(request.URL.Path, suffix)
	prefixEscaped := (&url.URL{Path: prefix}).EscapedPath()
	rawPath := strings.TrimSuffix(prefixEscaped, "/") + "/" + path.escaped
	decoded, err := url.PathUnescape(rawPath)
	if err != nil || decoded != request.URL.Path {
		return fmt.Errorf("tradedoubler: invalid escaped matrix path")
	}
	request.URL.RawPath = rawPath
	return nil
}

func (client *Client) getJSON(
	ctx context.Context,
	operation string,
	path matrixPath,
	output any,
	options ...socialhub.CallOption,
) (ResponseMeta, json.RawMessage, error) {
	if err := prepareCallOptions(operation, options); err != nil {
		return ResponseMeta{}, nil, err
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path.decoded, nil, nil, options...)
	if err != nil {
		return ResponseMeta{}, nil, withOperation(err, operation)
	}
	if err := applyEscapedPath(request, path); err != nil {
		return ResponseMeta{}, nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	var raw json.RawMessage
	metadata, err := client.api.DoWithMetadata(request, &raw)
	responseMeta := client.responseMeta(metadata.Header)
	rawCopy := append(json.RawMessage(nil), raw...)
	if err != nil {
		if providerRaw := rawFromAPIError(err); len(providerRaw) > 0 {
			rawCopy = providerRaw
		}
		return responseMeta, client.errorRaw(rawCopy), withOperation(err, operation)
	}
	if metadata.StatusCode != http.StatusOK {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Tradedoubler returned an unexpected successful HTTP status", metadata.StatusCode,
		)
	}
	if len(raw) == 0 || !json.Valid(raw) {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Tradedoubler returned an empty or invalid successful response", metadata.StatusCode,
		)
	}
	if !validJSONContentType(metadata.Header.Get("Content-Type")) {
		return responseMeta, client.errorRaw(rawCopy), platformContractError(
			operation, "Tradedoubler returned a non-JSON successful response", metadata.StatusCode,
		)
	}
	if err := json.Unmarshal(raw, output); err != nil {
		return responseMeta, client.errorRaw(rawCopy), withHTTPStatus(
			platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err), metadata.StatusCode,
		)
	}
	return responseMeta, rawCopy, nil
}

func (client *Client) responseMeta(header http.Header) ResponseMeta {
	return ResponseMeta{
		RequestID: boundedMessage(client.redactResponseValue(
			firstHeader(header, "X-Request-ID", "X-Correlation-ID"),
		), 256),
		RateLimitLimit: boundedMessage(client.redactResponseValue(
			firstHeader(header, "RateLimit-Limit", "X-RateLimit-Limit"),
		), 64),
		RateLimitRemaining: boundedMessage(client.redactResponseValue(
			firstHeader(header, "RateLimit-Remaining", "X-RateLimit-Remaining"),
		), 64),
		RateLimitReset: boundedMessage(client.redactResponseValue(
			firstHeader(header, "RateLimit-Reset", "X-RateLimit-Reset"),
		), 64),
	}
}

func (client *Client) redactResponseValue(value string) string {
	return redactErrorValue(value, client.currentErrorSecrets()...)
}

func (client *Client) errorRaw(value []byte) json.RawMessage {
	return boundedRedactedRaw(value, client.currentErrorSecrets()...)
}

func (client *Client) currentErrorSecrets() []string {
	if client.errorSecrets == nil {
		return nil
	}
	return client.errorSecrets()
}

func rawFromAPIError(err error) json.RawMessage {
	var apiError *APIError
	if !errors.As(err, &apiError) {
		return nil
	}
	return append(json.RawMessage(nil), apiError.Raw...)
}

func validJSONContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && (strings.EqualFold(mediaType, "application/json") || strings.HasSuffix(strings.ToLower(mediaType), "+json"))
}
