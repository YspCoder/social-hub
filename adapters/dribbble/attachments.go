package dribbble

import (
	"context"
	"io"
	"mime"
	"net/http"

	"social-hub/pkg/socialhub"
)

const maxAttachmentBytes int64 = 10 << 20

func (client *Client) UploadAttachment(ctx context.Context, input AttachmentUploadRequest, reader io.Reader, options ...socialhub.CallOption) (*AttachmentUpload, error) {
	if !validID(input.ShotID) || reader == nil || !validFilename(input.Filename) || !validMIME(input.MIME) || input.Size <= 0 || input.Size > maxAttachmentBytes {
		return nil, invalidArgument("upload_attachment", "Shot ID, safe filename, MIME, exact size up to 10 MiB, and reader are required")
	}
	if err := client.requireScopes("upload_attachment", "upload"); err != nil {
		return nil, err
	}
	metadata, err := client.multipartUpload(ctx, "upload_attachment", "/shots/"+input.ShotID+"/attachments", "file", input.Filename, input.MIME, input.Size, nil, reader, options...)
	if err != nil {
		return nil, err
	}
	if metadata.StatusCode != http.StatusAccepted {
		return nil, platformError("upload_attachment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &AttachmentUpload{ShotID: input.ShotID, State: socialhub.PublishStatePending}, nil
}

func (client *Client) DeleteAttachment(ctx context.Context, shotID, attachmentID string, options ...socialhub.CallOption) error {
	if !validID(shotID) || !validID(attachmentID) {
		return invalidArgument("delete_attachment", "Shot and attachment IDs must be positive integers")
	}
	if err := client.requireScopes("delete_attachment", "upload"); err != nil {
		return err
	}
	_, err := client.requestJSON(ctx, http.MethodDelete, "/shots/"+shotID+"/attachments/"+attachmentID, nil, nil, nil, options...)
	return err
}

func validMIME(value string) bool {
	mediaType, parameters, err := mime.ParseMediaType(value)
	return err == nil && mediaType != "" && len(parameters) == 0
}
