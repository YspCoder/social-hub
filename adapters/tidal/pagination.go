package tidal

import (
	"net/url"
	"strings"
)

func nextCursor(links Links, endpointPath string) (string, error) {
	fromMeta := ""
	if links.Meta != nil {
		fromMeta = links.Meta.NextCursor
		if !validOptionalOpaque(fromMeta, maxCursorLength) {
			return "", platformContractError("pagination", "links.meta.nextCursor is invalid")
		}
	}
	if links.Next == "" {
		return fromMeta, nil
	}
	if len(links.Next) > 16_384 || links.Next != strings.TrimSpace(links.Next) {
		return "", platformContractError("pagination", "links.next is invalid")
	}
	parsed, err := url.Parse(links.Next)
	if err != nil || parsed.User != nil || parsed.Fragment != "" {
		return "", platformContractError("pagination", "links.next is not a valid trusted URL")
	}
	if parsed.IsAbs() {
		if !strings.EqualFold(parsed.Scheme, "https") || !strings.EqualFold(parsed.Hostname(), "openapi.tidal.com") || parsed.Port() != "" {
			return "", platformContractError("pagination", "links.next points outside the official TIDAL API origin")
		}
	} else if parsed.Scheme != "" || parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") || strings.HasPrefix(parsed.Path, "//") {
		return "", platformContractError("pagination", "links.next must be root-relative or use the official TIDAL API origin")
	}
	wanted := "/" + strings.TrimLeft(endpointPath, "/")
	if parsed.EscapedPath() != wanted && parsed.EscapedPath() != "/v2"+wanted {
		return "", platformContractError("pagination", "links.next path does not match the current endpoint")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", platformContractError("pagination", "links.next contains an invalid query")
	}
	values, exists := query["page[cursor]"]
	if !exists || len(values) != 1 || !validOpaque(values[0], maxCursorLength) {
		return "", platformContractError("pagination", "links.next does not contain one valid page cursor")
	}
	if fromMeta != "" && fromMeta != values[0] {
		return "", platformContractError("pagination", "links.next and links.meta.nextCursor disagree")
	}
	return values[0], nil
}

func validDocumentLink(value string) bool {
	if !validOpaque(value, 16_384) {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.User != nil || parsed.Fragment != "" {
		return false
	}
	if parsed.IsAbs() {
		return strings.EqualFold(parsed.Scheme, "https") && strings.EqualFold(parsed.Hostname(), "openapi.tidal.com") && parsed.Port() == ""
	}
	return parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(parsed.Path, "//")
}
