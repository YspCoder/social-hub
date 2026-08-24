package micropub

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

var errUploadSizeMismatch = errors.New("Micropub media reader size does not match declared size")

func (client *Client) UploadMedia(ctx context.Context, input MediaUploadRequest, source io.Reader, options ...socialhub.CallOption) (*MediaResult, error) {
	if err := client.requireScope("upload_media", "create"); err != nil {
		return nil, err
	}
	if source == nil || !validEndpoint(input.Endpoint, true) || !validFilename(input.Filename) || !validMIME(input.MIME) || input.Size <= 0 || input.Size > maxUploadBytes {
		return nil, invalidArgument("upload_media", "absolute Media Endpoint, reader, safe filename, MIME, and exact size up to 8 GiB are required")
	}
	endpoint, _ := url.Parse(input.Endpoint)
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, cancel, err := client.newRequest(ctx, http.MethodPost, endpoint, nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.CloseWithError(err)
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	done := make(chan error, 1)
	go func() {
		writeErr := writeMediaPart(multipartWriter, input, source)
		if closeErr := multipartWriter.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()

	result, requestErr := client.do(request, cancel)
	if requestErr != nil {
		_ = pipeReader.CloseWithError(requestErr)
	}
	writeErr := <-done
	_ = pipeReader.Close()
	if errors.Is(writeErr, errUploadSizeMismatch) {
		return nil, invalidArgument("upload_media", writeErr.Error())
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil {
		return nil, platformError("upload_media", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, writeErr)
	}
	if result.Status != http.StatusCreated {
		return nil, &socialhub.Error{
			Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
			Platform: platformName, Product: productName, Op: "upload_media", HTTPStatus: result.Status,
			PlatformMessage: "Media Endpoint must return HTTP 201 Created",
		}
	}
	location := strings.TrimSpace(result.Header.Get("Location"))
	if !validAbsoluteURL(location) {
		return nil, &socialhub.Error{
			Code: socialhub.CodePlatformError, Class: socialhub.ClassPermanent,
			Platform: platformName, Product: productName, Op: "upload_media", HTTPStatus: result.Status,
			PlatformMessage: "Media Endpoint response requires an absolute Location URL",
		}
	}
	return &MediaResult{URL: location}, nil
}

func writeMediaPart(writer *multipart.Writer, input MediaUploadRequest, source io.Reader) error {
	disposition := mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": input.Filename})
	header := textproto.MIMEHeader{"Content-Disposition": {disposition}, "Content-Type": {input.MIME}}
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	written, err := io.Copy(part, io.LimitReader(source, input.Size+1))
	if err != nil {
		return err
	}
	if written != input.Size {
		return fmt.Errorf("%w: got %d, want %d", errUploadSizeMismatch, written, input.Size)
	}
	return nil
}
