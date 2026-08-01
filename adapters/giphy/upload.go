package giphy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"

	"social-hub/pkg/socialhub"
)

// Upload streams one animated GIF or video to GIPHY.
func (client *Client) Upload(ctx context.Context, input UploadRequest, reader io.Reader, options ...socialhub.CallOption) (*UploadResult, error) {
	if err := validateUpload(input, reader != nil); err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := client.upload.NewRequest(ctx, http.MethodPost, "/gifs", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	type copyResult struct{ err error }
	copyDone := make(chan copyResult, 1)
	go func() {
		copyErr := writeUploadFields(multipartWriter, input)
		var count int64
		if copyErr == nil {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": input.Filename}))
			header.Set("Content-Type", input.MIME)
			var part io.Writer
			part, copyErr = multipartWriter.CreatePart(header)
			if copyErr == nil {
				count, copyErr = io.Copy(part, io.LimitReader(reader, input.Size+1))
			}
		}
		if closeErr := multipartWriter.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr == nil && count != input.Size {
			copyErr = &uploadSizeError{expected: input.Size, actual: count}
		}
		_ = pipeWriter.CloseWithError(copyErr)
		copyDone <- copyResult{err: copyErr}
	}()

	var response singleEnvelope[UploadResult]
	requestErr := client.upload.Do(request, &response)
	if requestErr != nil {
		_ = pipeReader.CloseWithError(requestErr)
	}
	result := <-copyDone
	_ = pipeReader.Close()
	var sizeErr *uploadSizeError
	if errors.As(result.err, &sizeErr) {
		return nil, invalidArgument("upload", sizeErr.Error())
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if result.err != nil {
		return nil, platformError("upload", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, result.err)
	}
	if err := checkMeta("upload", response.Meta); err != nil {
		return nil, err
	}
	if !validPathSegment(response.Data.ID) {
		return nil, platformError("upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response.Data, nil
}

func validateUpload(input UploadRequest, hasReader bool) error {
	if !hasReader || !validFilename(input.Filename) || !validUploadMIME(input.MIME) || input.Size <= 0 || input.Size > maxUploadBytes {
		return invalidArgument("upload", "reader, safe filename, animated GIF/video MIME, and exact size up to 100MB are required")
	}
	if input.Username != "" && !validPathSegment(input.Username) || input.SourcePostURL != "" && !validHTTPURL(input.SourcePostURL) || input.CustomerID != "" && !validOpaque(input.CustomerID, 512) || !validCountry(input.CountryCode) || !validRegion(input.Region) {
		return invalidArgument("upload", "username, source URL, customer ID, country, or region is invalid")
	}
	tagBytes := 0
	for _, tag := range input.Tags {
		if !validText(tag, true, 100) || strings.Contains(tag, ",") {
			return invalidArgument("upload", "upload tags must be non-empty and must not contain commas")
		}
		tagBytes += len(tag) + 1
	}
	if tagBytes > 4096 {
		return invalidArgument("upload", "combined upload tags are too long")
	}
	return nil
}

func writeUploadFields(writer *multipart.Writer, input UploadRequest) error {
	fields := []struct{ key, value string }{
		{"username", input.Username}, {"tags", strings.Join(input.Tags, ",")}, {"source_post_url", input.SourcePostURL},
		{"customer_id", input.CustomerID}, {"country_code", input.CountryCode}, {"region", input.Region},
	}
	for _, field := range fields {
		if field.value != "" {
			if err := writer.WriteField(field.key, field.value); err != nil {
				return err
			}
		}
	}
	return nil
}

type uploadSizeError struct{ expected, actual int64 }

func (err *uploadSizeError) Error() string {
	return fmt.Sprintf("upload reader size is %d bytes; expected exactly %d", err.actual, err.expected)
}

var _ UploadWorkflow = (*Client)(nil)
