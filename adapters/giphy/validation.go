package giphy

import (
	"encoding/json"
	"mime"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxUploadBytes int64 = 100_000_000

func (number *Number) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		*number = 0
		return nil
	}
	var raw json.Number
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		raw = json.Number(text)
	} else {
		raw = json.Number(string(data))
	}
	value, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || value < 0 {
		return invalidArgument("decode_number", "GIPHY numeric field is invalid")
	}
	*number = Number(value)
	return nil
}

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value &&
		utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validText(value string, required bool, maximum int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum || strings.ContainsFunc(value, unsafeControl) {
		return false
	}
	return !required || strings.TrimSpace(value) != ""
}

func unsafeControl(character rune) bool {
	return unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t'
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validOrigin(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && (parsed.Path == "" || parsed.Path == "/") && parsed.RawQuery == "" && parsed.Fragment == ""
}

func normalizedOrigin(value string) string {
	parsed, _ := url.Parse(value)
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

func validPathSegment(value string) bool {
	return validOpaque(value, 255) && !strings.ContainsAny(value, `/\\?#`)
}

func validContent(value ContentType) bool {
	return value == ContentGIF || value == ContentSticker
}

func validRating(value Rating) bool {
	return value == "" || value == RatingG || value == RatingPG || value == RatingPG13 || value == RatingR
}

func validCountry(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func validRegion(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 16 {
		return false
	}
	for _, character := range value {
		if character != '-' && (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validLanguage(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && value[0] >= 'a' && value[0] <= 'z' && value[1] >= 'a' && value[1] <= 'z'
}

func validBundle(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 128 {
		return false
	}
	for _, character := range value {
		if character != '_' && character != '-' && !unicode.IsLetter(character) && !unicode.IsDigit(character) {
			return false
		}
	}
	return true
}

func validFilename(value string) bool {
	return validOpaque(value, 1024) && !strings.ContainsAny(value, `/\\`) && value != "." && value != ".."
}

func validUploadMIME(value string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	return err == nil && (mediaType == "image/gif" || strings.HasPrefix(mediaType, "video/")) && !strings.Contains(mediaType, "*")
}

func validHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil
}
