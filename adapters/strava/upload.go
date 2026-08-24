package strava

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"social-hub/pkg/socialhub"
)

var errUploadSizeMismatch = errors.New("activity file byte count does not match declared size")

func (client *Client) UploadActivity(ctx context.Context, input ActivityUploadRequest, reader io.Reader, options ...socialhub.CallOption) (*Upload, error) {
	fields, filename, err := activityUploadFields(input)
	if err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, invalidArgument("activity_upload", "activity file reader is required")
	}
	if err := client.requireScopes("activity_upload", "activity:write"); err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := client.api.NewRequest(ctx, http.MethodPost, "/uploads", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	done := make(chan error, 1)
	go func() {
		writeErr := writeActivityMultipart(multipartWriter, fields, filename, input.Size, reader)
		if writeErr == nil {
			writeErr = multipartWriter.Close()
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	var response uploadWire
	requestErr := client.api.Do(request, &response)
	_ = pipeReader.CloseWithError(requestErr)
	writeErr := <-done
	if errors.Is(writeErr, errUploadSizeMismatch) {
		return nil, platformError("activity_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
		return nil, platformError("activity_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil {
		return nil, platformError("activity_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	upload := mapUpload(response)
	if !validResourceID(upload.ID) {
		return nil, platformError("activity_upload", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return upload, nil
}

func (client *Client) GetUpload(ctx context.Context, uploadID string, options ...socialhub.CallOption) (*Upload, error) {
	if !validResourceID(uploadID) {
		return nil, invalidArgument("upload_get", "upload ID must be a positive decimal Strava ID")
	}
	if err := client.requireScopes("upload_get", "activity:write"); err != nil {
		return nil, err
	}
	var response uploadWire
	if err := client.api.JSON(ctx, http.MethodGet, "/uploads/"+url.PathEscape(uploadID), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	upload := mapUpload(response)
	if upload.ID != uploadID {
		return nil, platformError("upload_get", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return upload, nil
}

func activityUploadFields(input ActivityUploadRequest) (map[string]string, string, error) {
	filename := path.Base(strings.ReplaceAll(input.Filename, "\\", "/"))
	if filename == "." || filename == "/" || !validOpaque(filename, 1024) || input.Size <= 0 || input.Size == int64(^uint64(0)>>1) {
		return nil, "", invalidArgument("activity_upload", "filename and a positive, representable declared size are required")
	}
	if !validUploadDataType(input.DataType) {
		return nil, "", invalidArgument("activity_upload", "data_type must be fit, fit.gz, tcx, tcx.gz, gpx, gpx.gz, or json")
	}
	if input.SportType != "" && !validSportType(input.SportType) {
		return nil, "", invalidArgument("activity_upload", "sport_type is not documented by Strava")
	}
	if !validText(input.Name, 4096, true) || !validText(input.Description, 100_000, true) ||
		input.ExternalID != "" && !validOpaque(input.ExternalID, 4096) {
		return nil, "", invalidArgument("activity_upload", "upload metadata is invalid or too large")
	}
	fields := map[string]string{"data_type": string(input.DataType)}
	for key, value := range map[string]string{
		"sport_type": string(input.SportType), "name": input.Name, "description": input.Description, "external_id": input.ExternalID,
	} {
		if value != "" {
			fields[key] = value
		}
	}
	if input.Trainer != nil {
		fields["trainer"] = boolForm(*input.Trainer)
	}
	if input.Commute != nil {
		fields["commute"] = boolForm(*input.Commute)
	}
	return fields, filename, nil
}

func writeActivityMultipart(writer *multipart.Writer, fields map[string]string, filename string, size int64, reader io.Reader) error {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := writer.WriteField(key, fields[key]); err != nil {
			return err
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return err
	}
	written, err := io.Copy(part, io.LimitReader(reader, size+1))
	if err != nil {
		return err
	}
	if written != size {
		return fmt.Errorf("%w: got %d, want %d", errUploadSizeMismatch, written, size)
	}
	return nil
}

func validUploadDataType(value UploadDataType) bool {
	switch value {
	case UploadFIT, UploadFITGZ, UploadTCX, UploadTCXGZ, UploadGPX, UploadGPXGZ, UploadJSON:
		return true
	default:
		return false
	}
}
