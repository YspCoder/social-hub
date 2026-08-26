package awinpublisher

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	feedReaderBufferBytes = 64 << 10
	maxFeedErrorBytes     = int64(1 << 20)
)

var errFeedLineTooLarge = errors.New("Awin Enhanced Feed line exceeded the configured size limit")
var errFeedOutputTooLarge = errors.New("Awin Enhanced Feed exceeded the configured output size limit")

func (client *Client) DownloadEnhancedFeed(
	ctx context.Context,
	input DownloadEnhancedFeedRequest,
	output io.Writer,
	download FeedDownloadOptions,
	options ...socialhub.CallOption,
) (FeedDownloadResult, error) {
	const operation = "download_enhanced_feed"
	callOptions, err := prepareCallOptions(operation, options)
	if err != nil {
		return FeedDownloadResult{}, err
	}
	if !validDownloadEnhancedFeed(input) || output == nil || download.MaxBytes < 0 || download.MaxLineBytes < 0 {
		return FeedDownloadResult{}, invalidArgument(operation, "advertiser ID, locale, output, and nonnegative size limits are required")
	}
	maximum := download.MaxBytes
	if maximum == 0 {
		maximum = DefaultMaxFeedBytes
	}
	maximumLine := download.MaxLineBytes
	if maximumLine == 0 {
		maximumLine = DefaultMaxFeedLineBytes
	}
	if maximumLine > MaximumFeedLineBytes {
		return FeedDownloadResult{}, invalidArgument(operation, "max line bytes exceeds the 64 MiB provider-object limit")
	}
	feedKey := strconv.FormatInt(client.publisherID, 10) + "/" + strconv.FormatInt(input.AdvertiserID, 10) + "/retail/" + input.Locale
	if !client.acquireFeed(feedKey) {
		return FeedDownloadResult{}, &socialhub.Error{
			Code: socialhub.CodeConflict, Class: socialhub.ClassUserAction,
			Platform: platformName, Product: productName, Op: operation,
			PlatformMessage: "the same Awin advertiser feed is already downloading",
		}
	}
	defer client.releaseFeed(feedKey)

	path := client.publisherPath("/awinfeeds/download/" + strconv.FormatInt(input.AdvertiserID, 10) + "-retail-" + input.Locale + ".jsonl")
	if callOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, callOptions.Timeout)
		defer cancel()
	}
	request, err := client.api.NewRequest(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return FeedDownloadResult{}, withOperation(err, operation)
	}
	if callOptions.RequestID != "" {
		request.Header.Set("X-Request-ID", callOptions.RequestID)
	}
	request.Header.Set("Accept", "application/x-ndjson, application/json")
	request.Header.Set("Accept-Encoding", "identity")
	response, err := client.httpClient.Do(request)
	if err != nil {
		return FeedDownloadResult{}, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()

	result := FeedDownloadResult{
		ContentType: response.Header.Get("Content-Type"), ContentLength: response.ContentLength,
		RequestID: boundedMessage(firstNonEmpty(
			firstHeader(response.Header, "X-Request-ID", "X-Correlation-ID"), callOptions.RequestID,
		), 256),
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxFeedErrorBytes+1))
		if readErr != nil {
			return result, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxFeedErrorBytes {
			return result, platformContractError(operation, "Awin feed error response exceeded 1 MiB")
		}
		return result, withOperation(decodeHTTPError(response.StatusCode, response.Header, body, client.clock.Now()), operation)
	}
	if response.StatusCode != http.StatusOK {
		return result, platformContractError(operation, "Awin returned an unexpected successful feed status")
	}
	if encoding := strings.ToLower(strings.TrimSpace(response.Header.Get("Content-Encoding"))); encoding != "" && encoding != "identity" {
		return result, platformContractError(operation, "Awin returned an unexpected Enhanced Feed content encoding")
	}
	if !validFeedContentType(result.ContentType) {
		return result, platformContractError(operation, "Awin returned an unexpected Enhanced Feed content type")
	}

	reader := bufio.NewReaderSize(response.Body, feedReaderBufferBytes)
	for {
		line, readErr := readBoundedJSONLine(reader, maximumLine)
		if errors.Is(readErr, io.EOF) {
			return result, nil
		}
		if readErr != nil {
			if errors.Is(readErr, errFeedLineTooLarge) {
				return result, platformContractError(operation, readErr.Error())
			}
			return result, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 || !json.Valid(line) || line[0] != '{' {
			return result, platformContractError(operation, "Awin returned an invalid JSONL product record")
		}
		provider, terminal, err := enhancedFeedError(line)
		if err != nil {
			return result, platformContractError(operation, "Awin returned an invalid JSONL error sentinel")
		}
		if terminal {
			return result, enhancedFeedTerminalError(provider, line)
		}
		var product EnhancedFeedProduct
		if err := json.Unmarshal(line, &product); err != nil {
			return result, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
		}
		advertiserID, valid := positiveExactID(product.Meta.AdvertiserID)
		if !valid || advertiserID != input.AdvertiserID || !validExactIdentifier(product.ProductBasic.ID) ||
			!validRequiredWebURL(product.ProductBasic.Link) {
			return result, platformContractError(operation, "Awin returned an invalid or mismatched Enhanced Feed product")
		}
		recordSize := int64(len(line)) + 1
		if recordSize > maximum-result.BytesWritten {
			return result, platformContractError(operation, errFeedOutputTooLarge.Error())
		}
		written, writeErr := writeAll(output, line)
		result.BytesWritten += written
		if writeErr == nil {
			written, writeErr = writeAll(output, []byte{'\n'})
			result.BytesWritten += written
		}
		if writeErr != nil {
			return result, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, writeErr)
		}
		result.Products++
	}
}

func validFeedContentType(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	switch strings.ToLower(mediaType) {
	case "application/json", "application/jsonl", "application/x-jsonlines", "application/x-ndjson", "application/octet-stream", "text/plain":
		return true
	default:
		return false
	}
}

func readBoundedJSONLine(reader *bufio.Reader, maximum int64) ([]byte, error) {
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if int64(len(fragment)) > maximum-int64(len(line)) {
			return nil, errFeedLineTooLarge
		}
		line = append(line, fragment...)
		switch {
		case err == nil:
			return bytes.TrimSuffix(bytes.TrimSuffix(line, []byte{'\n'}), []byte{'\r'}), nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(line) > 0:
			return line, nil
		default:
			return nil, err
		}
	}
}

func enhancedFeedError(data []byte) (ProviderError, bool, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return ProviderError{}, false, err
	}
	raw, found := fields["error"]
	if !found || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return ProviderError{}, false, nil
	}
	var provider ProviderError
	if err := json.Unmarshal(data, &provider); err != nil || !provider.Error.IsSet() {
		return ProviderError{}, false, err
	}
	return provider, true, nil
}

func writeAll(output io.Writer, data []byte) (int64, error) {
	var total int64
	for len(data) > 0 {
		written, err := output.Write(data)
		if written < 0 || written > len(data) {
			return total, io.ErrShortWrite
		}
		total += int64(written)
		data = data[written:]
		if err != nil {
			return total, err
		}
		if written == 0 {
			return total, io.ErrNoProgress
		}
	}
	return total, nil
}

func (client *Client) acquireFeed(key string) bool {
	client.feedMu.Lock()
	defer client.feedMu.Unlock()
	if client.activeFeeds == nil {
		client.activeFeeds = make(map[string]struct{})
	}
	if _, found := client.activeFeeds[key]; found {
		return false
	}
	client.activeFeeds[key] = struct{}{}
	return true
}

func (client *Client) releaseFeed(key string) {
	client.feedMu.Lock()
	defer client.feedMu.Unlock()
	delete(client.activeFeeds, key)
}
