package deviantart

import (
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

var oauthScopes = map[string]struct{}{
	"basic": {}, "browse": {}, "collection": {}, "comment.manage": {}, "comment.post": {},
	"feed": {}, "gallery": {}, "message": {}, "note": {}, "publish": {}, "stash": {},
	"user": {}, "user.manage": {},
}

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validCallbackURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" &&
		parsed.User == nil && parsed.Fragment == ""
}

func validOpaque(value string, maximum int) bool {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func validResourceID(value string) bool {
	return validPathSegment(value, 256)
}

func validUsername(value string) bool {
	return validPathSegment(value, 100)
}

func validPathSegment(value string, maximum int) bool {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || value == "" || len(value) > maximum || strings.ContainsAny(value, "/\\?#%") {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	return true
}

func validUserAgent(value string) bool {
	trimmed := strings.TrimSpace(value)
	if value != trimmed || value == "" || len(value) > 512 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validScopes(scopes []string) bool {
	seen := make(map[string]struct{}, len(scopes))
	for _, raw := range scopes {
		scope := strings.TrimSpace(raw)
		if raw != scope {
			return false
		}
		if _, valid := oauthScopes[scope]; !valid {
			return false
		}
		if _, duplicate := seen[scope]; duplicate {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func scopeGranted(scopes []string, target string) bool {
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == target {
			return true
		}
	}
	return false
}

func validPKCEValue(value string) bool {
	if len(value) < 43 || len(value) > 128 {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || strings.ContainsRune("-._~", character) {
			continue
		}
		return false
	}
	return true
}

func validCursor(value string) bool {
	if value == "" || len(value) > 4096 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func offsetQuery(operation, cursor string, maximum, upperLimit, minimumOffset, maximumOffset int) (url.Values, int, error) {
	if maximum < 0 || maximum > upperLimit {
		return nil, 0, invalidArgument(operation, "max_results is outside the endpoint limit")
	}
	if maximum == 0 {
		maximum = 10
	}
	offset := 0
	if cursor != "" {
		parsed, err := strconv.Atoi(cursor)
		if err != nil || parsed < minimumOffset || parsed > maximumOffset {
			return nil, 0, invalidArgument(operation, "cursor must be a valid DeviantArt offset")
		}
		offset = parsed
	}
	return url.Values{"offset": {strconv.Itoa(offset)}, "limit": {strconv.Itoa(maximum)}}, offset, nil
}

func pageCursors(next *int, hasMore bool, current, pageSize, minimumOffset, maximumOffset int) (nextCursor, previousCursor *string, err error) {
	if next != nil {
		if *next < minimumOffset || *next > maximumOffset {
			return nil, nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
		}
		value := strconv.Itoa(*next)
		nextCursor = &value
	} else if hasMore {
		return nil, nil, platformError("pagination", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if current > minimumOffset && pageSize > 0 {
		value := strconv.Itoa(max(minimumOffset, current-pageSize))
		previousCursor = &value
	}
	return nextCursor, previousCursor, nil
}
