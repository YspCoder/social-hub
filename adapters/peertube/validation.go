package peertube

import (
	"errors"
	"fmt"
	"mime"
	"strings"
	"unicode"
	"unicode/utf8"
)

func validResourceID(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range []byte(value) {
		if !isASCIIAlphaNumeric(character) && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func validActorHandle(value string) bool {
	if value == "" || len(value) > 255 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range []byte(value) {
		if !isASCIIAlphaNumeric(character) && !strings.ContainsRune("._@:-", rune(character)) {
			return false
		}
	}
	return true
}

func validFilename(value string) bool {
	if value == "" || len(value) > 255 || value == "." || value == ".." || strings.TrimSpace(value) != value || strings.ContainsAny(value, `/\`) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validateMIME(value string) error {
	if value == "" {
		return nil
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || (mediaType != "application/octet-stream" && !strings.HasPrefix(mediaType, "video/")) {
		return errors.New("MIME type must be video/* or application/octet-stream")
	}
	return nil
}

func validateTags(tags []string) error {
	if len(tags) > 5 {
		return errors.New("at most five tags are allowed")
	}
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		length := utf8.RuneCountInString(tag)
		if length < 2 || length > 30 || strings.TrimSpace(tag) != tag {
			return errors.New("tags must contain 2 to 30 non-surrounding-space characters")
		}
		if _, exists := seen[tag]; exists {
			return errors.New("tags must be unique")
		}
		seen[tag] = struct{}{}
	}
	return nil
}

func validateUpload(input UploadVideoRequest) error {
	if !validFilename(input.Filename) || input.ChannelID < 1 {
		return errors.New("a safe filename and positive channel ID are required")
	}
	if length := utf8.RuneCountInString(input.Name); length < 3 || length > 120 || strings.TrimSpace(input.Name) == "" {
		return errors.New("name must contain 3 to 120 characters")
	}
	if err := validateMIME(input.MIME); err != nil {
		return err
	}
	if err := validateTags(input.Tags); err != nil {
		return err
	}
	return validateVideoConstants(input.Privacy, input.Category, input.Licence, input.CommentsPolicy)
}

func validateUpdate(input UpdateVideoRequest) error {
	if input.ChannelID == nil && input.Name == nil && input.Privacy == nil && input.Category == nil && input.Licence == nil &&
		input.Language == nil && input.Description == nil && input.WaitTranscoding == nil && input.Support == nil && input.NSFW == nil &&
		input.Tags == nil && input.CommentsPolicy == nil && input.DownloadEnabled == nil && input.OriginallyPublishedAt == nil {
		return errors.New("at least one update field is required")
	}
	if input.ChannelID != nil && *input.ChannelID < 1 {
		return errors.New("channel ID must be positive")
	}
	if input.Name != nil {
		length := utf8.RuneCountInString(*input.Name)
		if length < 3 || length > 120 || strings.TrimSpace(*input.Name) == "" {
			return errors.New("name must contain 3 to 120 characters")
		}
	}
	if input.Tags != nil {
		if len(*input.Tags) == 0 {
			return errors.New("tags update must contain at least one tag")
		}
		if err := validateTags(*input.Tags); err != nil {
			return err
		}
	}
	return validateVideoConstants(input.Privacy, input.Category, input.Licence, input.CommentsPolicy)
}

func isASCIIAlphaNumeric(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9'
}

func validateVideoConstants(privacy, category, licence, commentsPolicy *int) error {
	if privacy != nil && (*privacy < 1 || *privacy > 5) {
		return errors.New("privacy must be between 1 and 5")
	}
	if category != nil && *category < 1 {
		return errors.New("category must be positive")
	}
	if licence != nil && *licence < 1 {
		return errors.New("licence must be positive")
	}
	if commentsPolicy != nil && (*commentsPolicy < 1 || *commentsPolicy > 3) {
		return errors.New("comments policy must be between 1 and 3")
	}
	return nil
}

func validateSort(value string, allowed ...string) error {
	if value == "" {
		return nil
	}
	for _, candidate := range allowed {
		if value == candidate {
			return nil
		}
	}
	return fmt.Errorf("unsupported sort value %q", value)
}
