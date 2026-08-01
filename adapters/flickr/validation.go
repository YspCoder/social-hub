package flickr

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxPhotoSize int64 = 64 << 30

func pageValues(operation, cursor string, maximum int) (url.Values, error) {
	if maximum < 0 {
		return nil, invalidArgument(operation, "max results must not be negative")
	}
	values := url.Values{}
	if cursor != "" {
		page, err := strconv.Atoi(cursor)
		if err != nil || page < 1 || page > 1_000_000 {
			return nil, invalidArgument(operation, "cursor must be a positive decimal page number")
		}
		values.Set("page", strconv.Itoa(page))
	}
	if maximum > 0 {
		values.Set("per_page", strconv.Itoa(min(maximum, 500)))
	}
	return values, nil
}

func validatePhotoList(input PhotoListRequest) error {
	if input.UserID != "" && !validResourceID(input.UserID) || input.SafeSearch < 0 || input.SafeSearch > 3 || input.Privacy < 0 || input.Privacy > 5 || input.StartTime != nil && input.EndTime != nil && input.StartTime.After(*input.EndTime) {
		return invalidArgument("list_photos", "user ID, safe search, privacy, or time range is invalid")
	}
	return nil
}

func validateUpdatePhoto(input UpdatePhotoRequest) error {
	if input.Title == nil && input.Description == nil {
		return invalidArgument("update_photo", "title or description is required")
	}
	if input.Title != nil && (utf8.RuneCountInString(*input.Title) > 1024 || strings.ContainsFunc(*input.Title, unicode.IsControl)) || input.Description != nil && utf8.RuneCountInString(*input.Description) > 65_536 {
		return invalidArgument("update_photo", "title or description is invalid")
	}
	return nil
}

func validateUpload(input UploadPhotoRequest, readerPresent bool) error {
	if !readerPresent || !validFilename(input.Filename) || input.Size <= 0 || input.Size > maxPhotoSize || !validMediaMIME(input.MIME) {
		return invalidArgument("upload_photo", "reader, safe filename, image/video MIME, and size between 1 byte and 64 GiB are required")
	}
	if utf8.RuneCountInString(input.Title) > 1024 || strings.ContainsFunc(input.Title, unicode.IsControl) || utf8.RuneCountInString(input.Description) > 65_536 || len(input.Tags) > 500 {
		return invalidArgument("upload_photo", "title, description, or tags are invalid")
	}
	for _, tag := range input.Tags {
		if strings.TrimSpace(tag) == "" || utf8.RuneCountInString(tag) > 255 || strings.ContainsFunc(tag, unicode.IsControl) {
			return invalidArgument("upload_photo", "tags must be non-empty and contain no control characters")
		}
	}
	if input.SafetyLevel < 0 || input.SafetyLevel > 3 || input.ContentType < 0 || input.ContentType > 3 || input.Hidden < 0 || input.Hidden > 2 {
		return invalidArgument("upload_photo", "safety level, content type, or hidden value is invalid")
	}
	return nil
}

func validFilename(value string) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= 255 && !strings.ContainsAny(value, `/\\`) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validMediaMIME(value string) bool {
	return len(value) <= 255 && (strings.HasPrefix(value, "image/") || strings.HasPrefix(value, "video/")) && !strings.ContainsAny(value, "\r\n;")
}

func validAlbumMedia(value string) bool {
	return value == "" || value == "all" || value == "photos" || value == "videos"
}

func validText(value string, maximum int) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= maximum && !strings.ContainsFunc(value, unsafeControl)
}

func unsafeControl(character rune) bool {
	return unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t'
}
