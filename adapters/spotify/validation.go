package spotify

import (
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

func validScopes(scopes []string) bool {
	seen := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope == "" || len(scope) > 128 || strings.TrimSpace(scope) != scope {
			return false
		}
		for _, character := range scope {
			if (character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') || character == '-' || character == ':' {
				continue
			}
			return false
		}
		if _, exists := seen[scope]; exists {
			return false
		}
		seen[scope] = struct{}{}
	}
	return true
}

func validSpotifyID(value string) bool {
	if value == "" || len(value) > 128 || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9') {
			continue
		}
		return false
	}
	return true
}

func spotifyURIType(value string) (string, bool) {
	parts := strings.Split(value, ":")
	if len(parts) != 3 || parts[0] != "spotify" || !validSpotifyID(parts[2]) {
		return "", false
	}
	switch parts[1] {
	case "track", "album", "episode", "show", "audiobook", "artist", "user", "playlist":
		return parts[1], true
	default:
		return "", false
	}
}

func validPlayableURI(value string) bool {
	typeName, ok := spotifyURIType(value)
	return ok && (typeName == "track" || typeName == "episode")
}

func validContextURI(value string) bool {
	typeName, ok := spotifyURIType(value)
	return ok && (typeName == "album" || typeName == "artist" || typeName == "playlist")
}

func validMarket(value string) bool {
	if value == "" {
		return true
	}
	return len(value) == 2 && value[0] >= 'A' && value[0] <= 'Z' && value[1] >= 'A' && value[1] <= 'Z'
}

func pageQuery(operation, cursor string, maximum, limitCap int) (url.Values, error) {
	if maximum < 0 {
		return nil, invalidArgument(operation, "max results must not be negative")
	}
	query := url.Values{}
	if cursor != "" {
		offset, err := strconv.Atoi(cursor)
		if err != nil || offset < 0 || offset > 100_000 {
			return nil, invalidArgument(operation, "cursor must be a non-negative decimal offset")
		}
		query.Set("offset", strconv.Itoa(offset))
	}
	if maximum > 0 {
		query.Set("limit", strconv.Itoa(min(maximum, limitCap)))
	}
	return query, nil
}

func pageCursor(value string, base *url.URL) (*string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nil, platformError("pagination", "invalid Spotify pagination URL")
	}
	if (parsed.Scheme != "" || parsed.Host != "") &&
		(base == nil || !strings.EqualFold(parsed.Scheme, base.Scheme) || !strings.EqualFold(parsed.Host, base.Host)) {
		return nil, platformError("pagination", "Spotify pagination URL changed origin")
	}
	offset := parsed.Query().Get("offset")
	number, err := strconv.Atoi(offset)
	if err != nil || number < 0 {
		return nil, platformError("pagination", "invalid Spotify pagination offset")
	}
	return &offset, nil
}

func validDeviceID(value string) bool {
	return value == "" || (len(value) <= 256 && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl))
}
