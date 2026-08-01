package matrix

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

type boundedUploadReader struct {
	source    io.Reader
	remaining int64
	read      int64
	sourceErr error
}

func (reader *boundedUploadReader) Read(buffer []byte) (int, error) {
	if reader.remaining == 0 {
		return 0, io.EOF
	}
	if int64(len(buffer)) > reader.remaining {
		buffer = buffer[:reader.remaining]
	}
	count, err := reader.source.Read(buffer)
	reader.read += int64(count)
	reader.remaining -= int64(count)
	if err != nil {
		reader.sourceErr = err
	}
	return count, err
}

func (client *Client) Upload(ctx context.Context, input UploadRequest, content io.Reader, options ...socialhub.CallOption) (*socialhub.Media, error) {
	if content == nil || !validFilename(input.Filename) || !validMIME(input.MIME) || input.Size <= 0 {
		return nil, invalidArgument("upload", "filename, MIME type, positive size, and content reader are required")
	}
	mediaType, _, _ := mime.ParseMediaType(strings.TrimSpace(input.MIME))
	mediaType = strings.ToLower(mediaType)
	query := url.Values{"filename": {input.Filename}}
	reader := &boundedUploadReader{source: content, remaining: input.Size}
	request, err := client.newRequest(ctx, http.MethodPost, "/_matrix/media/v3/upload", query, reader, options...)
	if err != nil {
		return nil, err
	}
	request.ContentLength = input.Size
	request.Header.Set("Content-Type", mediaType)

	var response uploadResponse
	requestErr := client.api.Do(request, &response)
	if reader.read != input.Size && reader.sourceErr != nil {
		return nil, invalidArgument("upload", fmt.Sprintf("content reader supplied %d bytes; expected exactly %d", reader.read, input.Size))
	}
	if requestErr != nil {
		return nil, requestErr
	}
	extra := []byte{0}
	count, readErr := content.Read(extra)
	if count != 0 {
		return nil, invalidArgument("upload", fmt.Sprintf("content reader supplied more than the declared %d bytes", input.Size))
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return nil, invalidArgument("upload", "content reader failed while verifying the declared size")
	}
	if !validMXCURI(response.ContentURI) {
		return nil, platformError("upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	size := input.Size
	return &socialhub.Media{
		ID: response.ContentURI, URL: response.ContentURI, MIME: mediaType,
		Type: mediaTypeForMIME(mediaType), Size: &size, State: socialhub.MediaStateReady,
	}, nil
}

func mediaTypeForMIME(value string) socialhub.MediaType {
	switch {
	case strings.HasPrefix(value, "image/"):
		return socialhub.MediaTypeImage
	case strings.HasPrefix(value, "video/"):
		return socialhub.MediaTypeVideo
	case strings.HasPrefix(value, "audio/"):
		return socialhub.MediaTypeAudio
	default:
		return socialhub.MediaTypeDocument
	}
}

var _ MediaWorkflow = (*Client)(nil)
