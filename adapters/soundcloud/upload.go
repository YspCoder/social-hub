package soundcloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxTrackSize int64 = 4_000_000_000

var errUploadSizeMismatch = errors.New("audio byte count does not match declared size")

// TrackUploadRequest contains metadata and the declared size for a new track.
type TrackUploadRequest struct {
	Title       string
	Filename    string
	Size        int64
	Artist      string
	Description string
	Sharing     string
	Genre       string
	TagList     string
	License     string
	Commentable *bool
}

// TrackUpdateRequest contains mutable SoundCloud track metadata.
type TrackUpdateRequest struct {
	Title       *string
	Artist      *string
	Description *string
	Sharing     *string
	Genre       *string
	TagList     *string
	License     *string
	Commentable *bool
}

// TrackUploadWorkflow exposes SoundCloud's track creation and metadata lifecycle.
type TrackUploadWorkflow interface {
	Upload(context.Context, TrackUploadRequest, io.Reader, ...socialhub.CallOption) (*socialhub.Post, error)
	Status(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
	Update(context.Context, string, TrackUpdateRequest, ...socialhub.CallOption) (*socialhub.Post, error)
	Delete(context.Context, string, ...socialhub.CallOption) error
}

// TrackUploadService implements TrackUploadWorkflow.
type TrackUploadService struct{ client *Client }

func (s *TrackUploadService) Upload(ctx context.Context, input TrackUploadRequest, reader io.Reader, options ...socialhub.CallOption) (*socialhub.Post, error) {
	fields, err := uploadFields(input)
	if err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, invalidArgument("track_upload", "audio reader is required")
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := s.client.api.NewRequest(ctx, http.MethodPost, "/tracks", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	done := make(chan error, 1)
	go func() {
		writeErr := writeTrackMultipart(multipartWriter, fields, input.Filename, input.Size, reader)
		if writeErr == nil {
			writeErr = multipartWriter.Close()
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	var response soundCloudTrack
	requestErr := s.client.api.Do(request, &response)
	_ = pipeReader.CloseWithError(requestErr)
	writeErr := <-done
	if errors.Is(writeErr, errUploadSizeMismatch) {
		return nil, platformError("track_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
		return nil, platformError("track_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil {
		return nil, platformError("track_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	post, err := s.client.mapTrack(response)
	if err != nil {
		return nil, err
	}
	post.Status = &socialhub.PublishStatus{ID: post.ID, State: socialhub.PublishStatePending}
	if len(post.Media) > 0 {
		post.Media[0].State = socialhub.MediaStateProcessing
	}
	return post, nil
}

func (s *TrackUploadService) Status(ctx context.Context, trackURN string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	return s.client.GetPost(ctx, trackURN, options...)
}

func (s *TrackUploadService) Update(ctx context.Context, trackURN string, input TrackUpdateRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validURN(trackURN, "tracks") {
		return nil, invalidArgument("track_update", "track ID must be a SoundCloud track URN")
	}
	track, err := updateFields(input)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"track": track}
	var response soundCloudTrack
	if err := s.client.requestJSON(ctx, http.MethodPut, "/tracks/"+escapedURN(trackURN), nil, body, &response, options...); err != nil {
		return nil, err
	}
	post, err := s.client.mapTrack(response)
	if err != nil {
		return nil, err
	}
	if post.ID != trackURN {
		return nil, platformError("track_update", socialhub.CodePlatformError, socialhub.ClassPermanent, errors.New("response track URN mismatch"))
	}
	return post, nil
}

func (s *TrackUploadService) Delete(ctx context.Context, trackURN string, options ...socialhub.CallOption) error {
	if !validURN(trackURN, "tracks") {
		return invalidArgument("track_delete", "track ID must be a SoundCloud track URN")
	}
	return s.client.requestJSON(ctx, http.MethodDelete, "/tracks/"+escapedURN(trackURN), nil, nil, nil, options...)
}

func uploadFields(input TrackUploadRequest) (map[string]string, error) {
	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Filename) == "" || input.Size <= 0 || input.Size > maxTrackSize {
		return nil, invalidArgument("track_upload", "title, filename, and size between 1 byte and 4 GB are required")
	}
	if input.Sharing != "" && input.Sharing != "public" && input.Sharing != "private" {
		return nil, invalidArgument("track_upload", "sharing must be public or private")
	}
	if input.License != "" && !validLicense(input.License) {
		return nil, invalidArgument("track_upload", "license is invalid")
	}
	fields := map[string]string{"track[title]": input.Title}
	for key, value := range map[string]string{
		"track[artist]": input.Artist, "track[description]": input.Description, "track[sharing]": input.Sharing,
		"track[genre]": input.Genre, "track[tag_list]": input.TagList, "track[license]": input.License,
	} {
		if value != "" {
			fields[key] = value
		}
	}
	if input.Commentable != nil {
		fields["track[commentable]"] = fmt.Sprintf("%t", *input.Commentable)
	}
	return fields, nil
}

func updateFields(input TrackUpdateRequest) (map[string]any, error) {
	fields := make(map[string]any)
	for key, value := range map[string]*string{
		"title": input.Title, "metadata_artist": input.Artist, "description": input.Description,
		"sharing": input.Sharing, "genre": input.Genre, "tag_list": input.TagList, "license": input.License,
	} {
		if value != nil {
			fields[key] = *value
		}
	}
	if input.Commentable != nil {
		fields["commentable"] = *input.Commentable
	}
	if len(fields) == 0 {
		return nil, invalidArgument("track_update", "at least one track field is required")
	}
	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return nil, invalidArgument("track_update", "title must not be empty")
	}
	if input.Sharing != nil && *input.Sharing != "public" && *input.Sharing != "private" {
		return nil, invalidArgument("track_update", "sharing must be public or private")
	}
	if input.License != nil && !validLicense(*input.License) {
		return nil, invalidArgument("track_update", "license is invalid")
	}
	return fields, nil
}

func writeTrackMultipart(writer *multipart.Writer, fields map[string]string, filename string, size int64, reader io.Reader) error {
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			return err
		}
	}
	name := filepath.Base(filename)
	if name == "." || name == string(filepath.Separator) || strings.TrimSpace(name) == "" {
		return errors.New("invalid audio filename")
	}
	part, err := writer.CreateFormFile("track[asset_data]", name)
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

func validLicense(value string) bool {
	switch value {
	case "no-rights-reserved", "all-rights-reserved", "cc-by", "cc-by-nc", "cc-by-nd", "cc-by-sa", "cc-by-nc-nd", "cc-by-nc-sa":
		return true
	default:
		return false
	}
}

var _ TrackUploadWorkflow = (*TrackUploadService)(nil)
