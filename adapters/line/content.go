package line

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxContentErrorBytes int64 = 1 << 20

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body *cancelReadCloser) Close() error {
	err := body.ReadCloser.Close()
	body.cancel()
	return err
}

func (c *Client) DownloadContent(ctx context.Context, messageID string, options ...socialhub.CallOption) (*Content, error) {
	return c.download(ctx, messageID, "", "download_content", options...)
}

func (c *Client) DownloadPreview(ctx context.Context, messageID string, options ...socialhub.CallOption) (*Content, error) {
	return c.download(ctx, messageID, "/preview", "download_preview", options...)
}

func (c *Client) GetTranscodingStatus(ctx context.Context, messageID string, options ...socialhub.CallOption) (TranscodingStatus, error) {
	messageID = strings.TrimSpace(messageID)
	if !validOpaque(messageID, 256) {
		return "", invalidArgument("get_transcoding_status", "message ID is required")
	}
	var response struct {
		Status TranscodingStatus `json:"status"`
	}
	path := "/v2/bot/message/" + url.PathEscape(messageID) + "/content/transcoding"
	if err := c.request(ctx, c.data, http.MethodGet, path, nil, nil, &response, false, options...); err != nil {
		return "", err
	}
	switch response.Status {
	case TranscodingProcessing, TranscodingSucceeded, TranscodingFailed:
		return response.Status, nil
	default:
		return "", platformError("get_transcoding_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
}

func (c *Client) download(ctx context.Context, messageID, suffix, operation string, options ...socialhub.CallOption) (*Content, error) {
	messageID = strings.TrimSpace(messageID)
	if !validOpaque(messageID, 256) {
		return nil, invalidArgument(operation, "message ID is required")
	}
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return nil, err
	}
	cancel := context.CancelFunc(func() {})
	if resolved.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, resolved.Timeout)
	}
	requestOptions := make([]socialhub.CallOption, 0, 1)
	if resolved.RequestID != "" {
		requestOptions = append(requestOptions, socialhub.WithRequestID(resolved.RequestID))
	}
	path := "/v2/bot/message/" + url.PathEscape(messageID) + "/content" + suffix
	request, err := c.data.NewRequest(ctx, http.MethodGet, path, nil, nil, requestOptions...)
	if err != nil {
		cancel()
		return nil, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		cancel()
		return nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		defer response.Body.Close()
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxContentErrorBytes+1))
		cancel()
		if readErr != nil {
			return nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, readErr)
		}
		if int64(len(body)) > maxContentErrorBytes {
			return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	return &Content{
		Body: &cancelReadCloser{ReadCloser: response.Body, cancel: cancel}, ContentType: response.Header.Get("Content-Type"),
		ContentLength: response.ContentLength, ContentDisposition: response.Header.Get("Content-Disposition"),
	}, nil
}
