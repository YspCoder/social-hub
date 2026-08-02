package mixcloud

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	maxAudioBytes   int64 = 4_294_967_296
	maxPictureBytes int64 = 10_485_760
	maxSections           = 10_000
)

var errUploadSizeMismatch = errors.New("multipart source byte count does not match declared size")

type formField struct{ name, value string }

type fileSource struct {
	field, filename, mime string
	size                  int64
	reader                io.Reader
}

func (client *Client) Upload(ctx context.Context, input UploadRequest, audio, picture io.Reader, options ...socialhub.CallOption) (*ActionResponse, error) {
	fields, err := client.uploadFields(input, audio != nil, picture != nil)
	if err != nil {
		return nil, err
	}
	files := make([]fileSource, 0, 2)
	if picture != nil {
		files = append(files, fileSource{
			field: "picture", filename: input.PictureFilename, mime: input.PictureMIME,
			size: input.PictureSize, reader: picture,
		})
	}
	files = append(files, fileSource{
		field: "mp3", filename: input.AudioFilename, mime: "audio/mpeg", size: input.AudioSize, reader: audio,
	})
	response, err := client.multipart(ctx, http.MethodPost, "/upload/", "upload", fields, files, options...)
	if err != nil {
		return nil, err
	}
	username, _, key, ok := parseCloudcastKey(response.Result.Key)
	if !ok || !strings.EqualFold(username, client.username) {
		return nil, platformError("upload", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("invalid Mixcloud upload result key"))
	}
	response.Result.Key = key
	return response, nil
}

func (client *Client) Edit(ctx context.Context, cloudcastKey string, input EditRequest, picture io.Reader, options ...socialhub.CallOption) (*ActionResponse, error) {
	username, slug, key, ok := parseCloudcastKey(cloudcastKey)
	if !ok {
		return nil, invalidArgument("edit", "Cloudcast key must contain a username and slug")
	}
	if !strings.EqualFold(username, client.username) {
		return nil, invalidArgument("edit", "Cloudcast must belong to the configured Mixcloud user")
	}
	fields, err := client.editFields(input, picture != nil)
	if err != nil {
		return nil, err
	}
	var files []fileSource
	if picture != nil {
		files = []fileSource{{
			field: "picture", filename: input.PictureFilename, mime: input.PictureMIME,
			size: input.PictureSize, reader: picture,
		}}
	}
	path := "/upload/" + username + "/" + slug + "/edit/"
	response, err := client.multipart(ctx, http.MethodPost, path, "edit", fields, files, options...)
	if err != nil {
		return nil, err
	}
	if response.Result.Key != "" {
		_, _, resultKey, valid := parseCloudcastKey(response.Result.Key)
		if !valid || !strings.EqualFold(resultKey, key) {
			return nil, platformError("edit", socialhub.CodePlatformError, socialhub.ClassPermanent, fmt.Errorf("Mixcloud edit result key mismatch"))
		}
		response.Result.Key = resultKey
	}
	return response, nil
}

func (client *Client) multipart(ctx context.Context, method, path, operation string, fields []formField, files []fileSource, options ...socialhub.CallOption) (*ActionResponse, error) {
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := client.api.NewRequest(ctx, method, path, nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	request.Header.Set("User-Agent", client.userAgent)
	done := make(chan error, 1)
	go func() {
		writeErr := writeMultipart(multipartWriter, fields, files)
		if closeErr := multipartWriter.Close(); writeErr == nil {
			writeErr = closeErr
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()

	var response ActionResponse
	requestErr := client.api.Do(request, &response)
	_ = pipeReader.CloseWithError(requestErr)
	writeErr := <-done
	if errors.Is(writeErr, errUploadSizeMismatch) {
		return nil, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if requestErr != nil {
		return nil, requestErr
	}
	if writeErr != nil {
		return nil, platformError(operation, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, writeErr)
	}
	if response.Result == nil || !response.Result.Success {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func writeMultipart(writer *multipart.Writer, fields []formField, files []fileSource) error {
	for _, field := range fields {
		if err := writer.WriteField(field.name, field.value); err != nil {
			return err
		}
	}
	for _, file := range files {
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
			"name": file.field, "filename": file.filename,
		}))
		header.Set("Content-Type", file.mime)
		part, err := writer.CreatePart(header)
		if err != nil {
			return err
		}
		written, err := io.Copy(part, io.LimitReader(file.reader, file.size+1))
		if err != nil {
			return err
		}
		if written != file.size {
			return fmt.Errorf("%w for %s: got %d, want %d", errUploadSizeMismatch, file.field, written, file.size)
		}
	}
	return nil
}

func (client *Client) uploadFields(input UploadRequest, hasAudio, hasPicture bool) ([]formField, error) {
	if !hasAudio || !validText(input.Name, false, 255) || !validFilename(input.AudioFilename) ||
		!strings.EqualFold(filepath.Ext(input.AudioFilename), ".mp3") || input.AudioSize <= 0 || input.AudioSize > maxAudioBytes {
		return nil, invalidArgument("upload", "name, MP3 reader, safe .mp3 filename, and exact size up to 4 GiB are required")
	}
	if err := validatePicture("upload", input.PictureFilename, input.PictureMIME, input.PictureSize, hasPicture); err != nil {
		return nil, err
	}
	if err := validateMetadata("upload", input.Description, input.Tags, input.Hosts, input.Sections); err != nil {
		return nil, err
	}
	if err := client.requireProFields("upload", input.PublishDate != nil, input.DisableComments != nil, input.HideStats != nil, len(input.Hosts) > 0); err != nil {
		return nil, err
	}
	fields := []formField{{"name", input.Name}}
	fields = appendMetadataFields(fields, input.Description, input.Tags, input.Unlisted, input.PublishDate,
		input.DisableComments, input.HideStats, input.Hosts, input.Sections)
	return fields, nil
}

func (client *Client) editFields(input EditRequest, hasPicture bool) ([]formField, error) {
	if err := validatePicture("edit", input.PictureFilename, input.PictureMIME, input.PictureSize, hasPicture); err != nil {
		return nil, err
	}
	if input.Name != nil && !validText(*input.Name, false, 255) || input.Description != nil && !validText(*input.Description, true, 1000) {
		return nil, invalidArgument("edit", "name or description is invalid")
	}
	if input.Publish && input.Unpublish || input.Unlisted != nil && (input.Publish || input.Unpublish) {
		return nil, invalidArgument("edit", "only one of unlisted, publish, or unpublish may be supplied")
	}
	var tags []string
	if input.Tags != nil {
		tags = *input.Tags
		if err := validateTags("edit", tags); err != nil {
			return nil, err
		}
	}
	var hosts []string
	if input.Hosts != nil {
		hosts = *input.Hosts
		if err := validateHosts("edit", hosts); err != nil {
			return nil, err
		}
	}
	var sections []Section
	if input.Sections != nil {
		sections = *input.Sections
		if len(sections) == 0 {
			return nil, invalidArgument("edit", "sections must be non-empty when supplied")
		}
		if err := validateSections("edit", sections); err != nil {
			return nil, err
		}
	}
	if err := client.requireProFields("edit", input.PublishDate != nil, input.DisableComments != nil, input.HideStats != nil, input.Hosts != nil); err != nil {
		return nil, err
	}
	fields := make([]formField, 0)
	if input.Name != nil {
		fields = append(fields, formField{"name", *input.Name})
	}
	if input.Description != nil {
		fields = append(fields, formField{"description", *input.Description})
	}
	if input.Tags != nil {
		fields = appendTags(fields, tags)
	}
	if input.Unlisted != nil {
		fields = append(fields, formField{"unlisted", strconv.FormatBool(*input.Unlisted)})
	}
	if input.Publish {
		fields = append(fields, formField{"publish", "true"})
	}
	if input.Unpublish {
		fields = append(fields, formField{"unpublish", "true"})
	}
	fields = appendProFields(fields, input.PublishDate, input.DisableComments, input.HideStats, hosts, input.Hosts != nil)
	if input.Sections != nil {
		fields = appendSections(fields, sections)
	}
	if len(fields) == 0 && !hasPicture {
		return nil, invalidArgument("edit", "at least one mutable field or picture is required")
	}
	return fields, nil
}

func validatePicture(operation, filename, contentType string, size int64, hasPicture bool) error {
	configured := filename != "" || contentType != "" || size != 0
	if configured != hasPicture {
		return invalidArgument(operation, "picture reader and metadata must be supplied together")
	}
	if !hasPicture {
		return nil
	}
	if !validFilename(filename) || !validPictureMIME(contentType) || size <= 0 || size > maxPictureBytes {
		return invalidArgument(operation, "safe picture filename, image MIME, and exact size up to 10 MiB are required")
	}
	return nil
}

func validateMetadata(operation, description string, tags, hosts []string, sections []Section) error {
	if !validText(description, true, 1000) {
		return invalidArgument(operation, "description exceeds 1000 characters or contains invalid text")
	}
	if err := validateTags(operation, tags); err != nil {
		return err
	}
	if err := validateHosts(operation, hosts); err != nil {
		return err
	}
	return validateSections(operation, sections)
}

func validateTags(operation string, tags []string) error {
	if len(tags) > 5 {
		return invalidArgument(operation, "Mixcloud accepts at most 5 tags")
	}
	for _, tag := range tags {
		if !validText(tag, false, 100) {
			return invalidArgument(operation, "tag text is invalid")
		}
	}
	return nil
}

func validateHosts(operation string, hosts []string) error {
	if len(hosts) > 2 {
		return invalidArgument(operation, "Mixcloud Pro accepts at most 2 hosts")
	}
	for _, host := range hosts {
		if !validSegment(host, maxUsernameLength) {
			return invalidArgument(operation, "host username is invalid")
		}
	}
	return nil
}

func validateSections(operation string, sections []Section) error {
	if len(sections) > maxSections {
		return invalidArgument(operation, "too many track or chapter sections")
	}
	lastStart := -1
	for _, section := range sections {
		chapter := strings.TrimSpace(section.Chapter) != ""
		track := strings.TrimSpace(section.Artist) != "" || strings.TrimSpace(section.Song) != ""
		if chapter == track || track && (!validText(section.Artist, false, 512) || !validText(section.Song, false, 512)) ||
			chapter && !validText(section.Chapter, false, 512) || section.StartTime < 0 || section.StartTime < lastStart {
			return invalidArgument(operation, "each ordered section must be either a chapter or a complete artist/song track")
		}
		lastStart = section.StartTime
	}
	return nil
}

func (client *Client) requireProFields(operation string, flags ...bool) error {
	usesPro := false
	for _, flag := range flags {
		usesPro = usesPro || flag
	}
	if usesPro && client.accountType != "" && !strings.EqualFold(client.accountType, "pro") {
		return approvalRequired(operation, "configured Mixcloud account_type does not permit Pro-only upload fields")
	}
	return nil
}

func appendMetadataFields(fields []formField, description string, tags []string, unlisted *bool, publishDate *time.Time, disableComments, hideStats *bool, hosts []string, sections []Section) []formField {
	if description != "" {
		fields = append(fields, formField{"description", description})
	}
	fields = appendTags(fields, tags)
	if unlisted != nil {
		fields = append(fields, formField{"unlisted", strconv.FormatBool(*unlisted)})
	}
	fields = appendProFields(fields, publishDate, disableComments, hideStats, hosts, len(hosts) > 0)
	return appendSections(fields, sections)
}

func appendTags(fields []formField, tags []string) []formField {
	if len(tags) == 0 {
		return fields
	}
	for index, tag := range tags {
		fields = append(fields, formField{fmt.Sprintf("tags-%d-tag", index), tag})
	}
	return fields
}

func appendProFields(fields []formField, publishDate *time.Time, disableComments, hideStats *bool, hosts []string, includeHosts bool) []formField {
	if publishDate != nil {
		fields = append(fields, formField{"publish_date", publishDate.UTC().Format("2006-01-02T15:04:05Z")})
	}
	if disableComments != nil {
		fields = append(fields, formField{"disable_comments", strconv.FormatBool(*disableComments)})
	}
	if hideStats != nil {
		fields = append(fields, formField{"hide_stats", strconv.FormatBool(*hideStats)})
	}
	if includeHosts {
		if len(hosts) == 0 {
			fields = append(fields, formField{"hosts-0-username", ""})
		} else {
			for index, host := range hosts {
				fields = append(fields, formField{fmt.Sprintf("hosts-%d-username", index), host})
			}
		}
	}
	return fields
}

func appendSections(fields []formField, sections []Section) []formField {
	for index, section := range sections {
		prefix := fmt.Sprintf("sections-%d-", index)
		if section.Chapter != "" {
			fields = append(fields, formField{prefix + "chapter", section.Chapter})
		} else {
			fields = append(fields, formField{prefix + "artist", section.Artist}, formField{prefix + "song", section.Song})
		}
		fields = append(fields, formField{prefix + "start_time", strconv.Itoa(section.StartTime)})
	}
	return fields
}
