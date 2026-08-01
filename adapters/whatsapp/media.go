package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

const (
	megabyte         int64 = 1024 * 1024
	maxDocumentBytes       = 100 * megabyte
)

var mediaLimits = map[string]int64{
	"audio/aac": 16 * megabyte, "audio/mp4": 16 * megabyte, "audio/mpeg": 16 * megabyte,
	"audio/amr": 16 * megabyte, "audio/ogg": 16 * megabyte,
	"text/plain": 100 * megabyte, "application/pdf": 100 * megabyte,
	"application/vnd.ms-powerpoint": 100 * megabyte, "application/msword": 100 * megabyte,
	"application/vnd.ms-excel": 100 * megabyte,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document":   100 * megabyte,
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": 100 * megabyte,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         100 * megabyte,
	"image/jpeg": 5 * megabyte, "image/png": 5 * megabyte, "image/webp": 100 * 1024,
	"video/mp4": 16 * megabyte, "video/3gpp": 16 * megabyte,
}

type uploadWriteResult struct {
	size int64
	err  error
}

func (c *Client) UploadMedia(ctx context.Context, input MediaUploadRequest, options ...socialhub.CallOption) (*MediaInfo, error) {
	mimeType := strings.ToLower(strings.TrimSpace(input.MIME))
	limit, supported := mediaLimits[mimeType]
	if input.Reader == nil || strings.TrimSpace(input.Filename) == "" || input.Size <= 0 || !supported {
		return nil, invalidArgument("upload_media", "filename, supported MIME type, positive size, and reader are required")
	}
	if input.Size > limit || input.Size > maxDocumentBytes {
		return nil, invalidArgument("upload_media", "declared media size exceeds the MIME-specific Cloud API limit")
	}
	if err := c.requireScope("upload_media", "whatsapp_business_messaging"); err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	done := make(chan uploadWriteResult, 1)
	go func() {
		result := uploadWriteResult{}
		if err := writer.WriteField("messaging_product", "whatsapp"); err != nil {
			result.err = err
		} else if err := writer.WriteField("type", mimeType); err != nil {
			result.err = err
		} else {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, escapeQuotes(input.Filename)))
			header.Set("Content-Type", mimeType)
			part, err := writer.CreatePart(header)
			if err != nil {
				result.err = err
			} else {
				result.size, result.err = io.Copy(part, io.LimitReader(input.Reader, input.Size+1))
			}
		}
		if closeErr := writer.Close(); result.err == nil {
			result.err = closeErr
		}
		_ = pipeWriter.CloseWithError(result.err)
		done <- result
	}()
	request, err := c.transport.NewRequest(ctx, http.MethodPost, c.phonePath("media"), nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = <-done
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	var response struct {
		ID string `json:"id"`
	}
	httpErr := c.transport.Do(request, &response)
	_ = pipeReader.CloseWithError(httpErr)
	writeResult := <-done
	if httpErr != nil {
		return nil, httpErr
	}
	if writeResult.err != nil {
		return nil, platformError("upload_media", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeResult.err)
	}
	if writeResult.size != input.Size {
		return nil, invalidArgument("upload_media", "uploaded byte count does not match declared size")
	}
	if strings.TrimSpace(response.ID) == "" {
		return nil, platformError("upload_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &MediaInfo{ID: response.ID, MIME: mimeType, Size: input.Size, MessagingProduct: "whatsapp"}, nil
}

func (c *Client) GetMedia(ctx context.Context, mediaID string, options ...socialhub.CallOption) (*MediaInfo, error) {
	if strings.TrimSpace(mediaID) == "" {
		return nil, invalidArgument("get_media", "media ID is required")
	}
	if err := c.requireScope("get_media", "whatsapp_business_messaging"); err != nil {
		return nil, err
	}
	var response struct {
		ID               string      `json:"id"`
		URL              string      `json:"url"`
		MIME             string      `json:"mime_type"`
		SHA256           string      `json:"sha256"`
		Size             stringInt64 `json:"file_size"`
		MessagingProduct string      `json:"messaging_product"`
	}
	query := url.Values{"phone_number_id": {c.phoneNumberID}}
	if err := c.request(ctx, http.MethodGet, "/"+url.PathEscape(mediaID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || response.URL == "" || response.MessagingProduct != "whatsapp" {
		return nil, platformError("get_media", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &MediaInfo{
		ID: response.ID, URL: response.URL, MIME: response.MIME, SHA256: response.SHA256,
		Size: int64(response.Size), MessagingProduct: response.MessagingProduct,
	}, nil
}

func (c *Client) DeleteMedia(ctx context.Context, mediaID string, options ...socialhub.CallOption) error {
	if strings.TrimSpace(mediaID) == "" {
		return invalidArgument("delete_media", "media ID is required")
	}
	if err := c.requireScope("delete_media", "whatsapp_business_messaging"); err != nil {
		return err
	}
	var response successPayload
	query := url.Values{"phone_number_id": {c.phoneNumberID}}
	if err := c.request(ctx, http.MethodDelete, "/"+url.PathEscape(mediaID), query, nil, &response, options...); err != nil {
		return err
	}
	return requireSuccess(response, "delete_media")
}

type stringInt64 int64

func (value *stringInt64) UnmarshalJSON(data []byte) error {
	var number int64
	if err := json.Unmarshal(data, &number); err == nil {
		*value = stringInt64(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	parsed, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*value = stringInt64(parsed)
	return nil
}

func escapeQuotes(value string) string {
	value = strings.ReplaceAll(value, "\r", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "\"", "\\\"")
}
