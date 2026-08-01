package imgur

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"

	"social-hub/pkg/socialhub"
)

// Upload streams one image or video in a single Imgur multipart request.
func (client *Client) Upload(ctx context.Context, input UploadRequest, reader io.Reader, options ...socialhub.CallOption) (*Image, error) {
	if reader == nil || !validFilename(input.Filename) || !validUploadMIME(input.MIME) || input.Size <= 0 {
		return nil, invalidArgument("upload", "reader, safe filename, image/video MIME, and positive exact size are required")
	}
	if input.Album != "" && !validIdentifier(input.Album) || !validText(input.Name, false) || !validText(input.Title, false) || !validText(input.Description, false) {
		return nil, invalidArgument("upload", "upload metadata is invalid")
	}

	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := client.active().NewRequest(ctx, http.MethodPost, path("image"), nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	type copyResult struct {
		err error
	}
	copyDone := make(chan copyResult, 1)
	go func() {
		copyErr := writeUploadFields(multipartWriter, input)
		var count int64
		if copyErr == nil {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "image", "filename": input.Filename}))
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

	var envelope apiEnvelope
	requestErr := client.active().Do(request, &envelope)
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
	var image Image
	if err := decodeEnvelope(envelope, &image); err != nil {
		return nil, err
	}
	if !validIdentifier(image.ID) || !validHTTPURL(image.Link) {
		return nil, platformError("upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &image, nil
}

func writeUploadFields(writer *multipart.Writer, input UploadRequest) error {
	fields := []struct{ key, value string }{
		{"album", input.Album}, {"name", input.Name}, {"title", input.Title}, {"description", input.Description},
	}
	if input.DisableAudio != nil {
		fields = append(fields, struct{ key, value string }{"disable_audio", strconv.FormatBool(*input.DisableAudio)})
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
