package peertube

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) UploadVideo(ctx context.Context, input UploadVideoRequest, source io.Reader, options ...socialhub.CallOption) (*VideoUploadResult, error) {
	if source == nil {
		return nil, invalidArgument("upload_video", "video reader is required")
	}
	if err := validateUpload(input); err != nil {
		return nil, invalidArgument("upload_video", err.Error())
	}
	if err := c.requireUser("upload_video"); err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	request, err := c.transport.NewRequest(ctx, http.MethodPost, "/videos/upload", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())

	copyDone := make(chan error, 1)
	go func() {
		writeErr := writeUploadFields(writer, input)
		if writeErr == nil {
			mediaType := input.MIME
			if mediaType == "" {
				mediaType = "application/octet-stream"
			}
			disposition := mime.FormatMediaType("form-data", map[string]string{"name": "videofile", "filename": input.Filename})
			part, err := writer.CreatePart(textproto.MIMEHeader{
				"Content-Disposition": {disposition},
				"Content-Type":        {mediaType},
			})
			if err != nil {
				writeErr = err
			} else {
				_, writeErr = io.Copy(part, source)
			}
		}
		if closeErr := writer.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
		copyDone <- writeErr
	}()

	var response struct {
		Video struct {
			ID        int64  `json:"id"`
			UUID      string `json:"uuid"`
			ShortUUID string `json:"shortUUID"`
		} `json:"video"`
	}
	requestErr := c.transport.Do(request, &response)
	_ = pipeReader.Close()
	copyErr := <-copyDone
	if requestErr != nil {
		return nil, requestErr
	}
	if copyErr != nil {
		return nil, platformError("upload_video", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, copyErr)
	}
	if response.Video.ID < 1 || !validResourceID(response.Video.UUID) {
		return nil, platformError("upload_video", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("invalid upload response"))
	}
	return &VideoUploadResult{ID: response.Video.ID, UUID: response.Video.UUID, ShortUUID: response.Video.ShortUUID}, nil
}

func (c *Client) UpdateVideo(ctx context.Context, videoID string, input UpdateVideoRequest, options ...socialhub.CallOption) error {
	if !validResourceID(videoID) {
		return invalidArgument("update_video", "a valid video ID is required")
	}
	if err := validateUpdate(input); err != nil {
		return invalidArgument("update_video", err.Error())
	}
	if err := c.requireUser("update_video"); err != nil {
		return err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writeUpdateFields(writer, input); err != nil {
		return platformError("update_video", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if err := writer.Close(); err != nil {
		return platformError("update_video", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := c.transport.NewRequest(ctx, http.MethodPut, "/videos/"+url.PathEscape(videoID), nil, &body, options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return c.transport.Do(request, nil)
}

func (c *Client) DeleteVideo(ctx context.Context, videoID string, options ...socialhub.CallOption) error {
	if !validResourceID(videoID) {
		return invalidArgument("delete_video", "a valid video ID is required")
	}
	if err := c.requireUser("delete_video"); err != nil {
		return err
	}
	return c.transport.JSON(ctx, http.MethodDelete, "/videos/"+url.PathEscape(videoID), nil, nil, nil, options...)
}

func writeUploadFields(writer *multipart.Writer, input UploadVideoRequest) error {
	if err := writer.WriteField("channelId", strconv.FormatInt(input.ChannelID, 10)); err != nil {
		return err
	}
	if err := writer.WriteField("name", input.Name); err != nil {
		return err
	}
	fields := uploadOptionalFields(input)
	for _, field := range fields {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return err
		}
	}
	for _, tag := range input.Tags {
		if err := writer.WriteField("tags", tag); err != nil {
			return err
		}
	}
	return nil
}

func writeUpdateFields(writer *multipart.Writer, input UpdateVideoRequest) error {
	fields := updateOptionalFields(input)
	for _, field := range fields {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return err
		}
	}
	if input.Tags != nil {
		for _, tag := range *input.Tags {
			if err := writer.WriteField("tags", tag); err != nil {
				return err
			}
		}
	}
	return nil
}

type formField struct{ name, value string }

func uploadOptionalFields(input UploadVideoRequest) []formField {
	fields := make([]formField, 0, 14)
	addInt(&fields, "privacy", input.Privacy)
	addInt(&fields, "category", input.Category)
	addInt(&fields, "licence", input.Licence)
	addString(&fields, "language", input.Language)
	addString(&fields, "description", input.Description)
	addBool(&fields, "waitTranscoding", input.WaitTranscoding)
	addBool(&fields, "generateTranscription", input.GenerateTranscription)
	addString(&fields, "support", input.Support)
	addBool(&fields, "nsfw", input.NSFW)
	addInt(&fields, "commentsPolicy", input.CommentsPolicy)
	addBool(&fields, "downloadEnabled", input.DownloadEnabled)
	if input.OriginallyPublishedAt != nil {
		fields = append(fields, formField{"originallyPublishedAt", input.OriginallyPublishedAt.Format(timeFormat)})
	}
	return fields
}

func updateOptionalFields(input UpdateVideoRequest) []formField {
	fields := make([]formField, 0, 13)
	if input.ChannelID != nil {
		fields = append(fields, formField{"channelId", strconv.FormatInt(*input.ChannelID, 10)})
	}
	addString(&fields, "name", input.Name)
	addInt(&fields, "privacy", input.Privacy)
	addInt(&fields, "category", input.Category)
	addInt(&fields, "licence", input.Licence)
	addString(&fields, "language", input.Language)
	addString(&fields, "description", input.Description)
	addBool(&fields, "waitTranscoding", input.WaitTranscoding)
	addString(&fields, "support", input.Support)
	addBool(&fields, "nsfw", input.NSFW)
	addInt(&fields, "commentsPolicy", input.CommentsPolicy)
	addBool(&fields, "downloadEnabled", input.DownloadEnabled)
	if input.OriginallyPublishedAt != nil {
		fields = append(fields, formField{"originallyPublishedAt", input.OriginallyPublishedAt.Format(timeFormat)})
	}
	return fields
}

func addString(fields *[]formField, name string, value *string) {
	if value != nil {
		*fields = append(*fields, formField{name, *value})
	}
}

func addInt(fields *[]formField, name string, value *int) {
	if value != nil {
		*fields = append(*fields, formField{name, strconv.Itoa(*value)})
	}
}

func addBool(fields *[]formField, name string, value *bool) {
	if value != nil {
		*fields = append(*fields, formField{name, strconv.FormatBool(*value)})
	}
}

const timeFormat = "2006-01-02T15:04:05Z07:00"
