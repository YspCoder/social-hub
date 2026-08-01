package dribbble

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

func (client *Client) multipartUpload(ctx context.Context, operation, path, fieldName, filename, contentType string, size int64, fields map[string][]string, reader io.Reader, options ...socialhub.CallOption) (transport.ResponseMetadata, error) {
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := client.api.NewRequest(ctx, http.MethodPost, path, nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return transport.ResponseMetadata{}, err
	}
	request.Header.Set("Accept", textMediaType)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())

	copyDone := make(chan error, 1)
	go func() {
		copyErr := writeFields(multipartWriter, fields)
		var count int64
		if copyErr == nil {
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": fieldName, "filename": filename}))
			header.Set("Content-Type", contentType)
			part, partErr := multipartWriter.CreatePart(header)
			copyErr = partErr
			if copyErr == nil {
				count, copyErr = io.Copy(part, io.LimitReader(reader, size+1))
			}
		}
		if closeErr := multipartWriter.Close(); copyErr == nil {
			copyErr = closeErr
		}
		if copyErr == nil && count != size {
			copyErr = &uploadSizeError{expected: size, actual: count}
		}
		_ = pipeWriter.CloseWithError(copyErr)
		copyDone <- copyErr
	}()

	metadata, requestErr := client.api.DoWithMetadata(request, nil)
	client.recordRateLimit(metadata.Header)
	client.applyResetRetryAfter(requestErr, metadata.Header)
	if requestErr != nil {
		_ = pipeReader.CloseWithError(requestErr)
	}
	copyErr := <-copyDone
	_ = pipeReader.Close()
	var sizeErr *uploadSizeError
	if errors.As(copyErr, &sizeErr) {
		return metadata, invalidArgument(operation, sizeErr.Error())
	}
	if requestErr != nil {
		return metadata, requestErr
	}
	if copyErr != nil {
		return metadata, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, copyErr)
	}
	return metadata, nil
}

func writeFields(writer *multipart.Writer, fields map[string][]string) error {
	for key, values := range fields {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
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
