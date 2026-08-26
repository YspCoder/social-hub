package foursquare

import (
	"net/url"
	"strings"
)

func nextCursorFromLink(value string) *string {
	for _, link := range splitLinkHeader(value) {
		start := strings.IndexByte(link, '<')
		end := strings.IndexByte(link, '>')
		if start < 0 || end <= start || !hasNextRelation(link[end+1:]) {
			continue
		}
		target, err := url.Parse(strings.TrimSpace(link[start+1 : end]))
		if err != nil {
			continue
		}
		cursor := target.Query().Get("cursor")
		if !validOpaque(cursor, 4096) {
			continue
		}
		copy := cursor
		return &copy
	}
	return nil
}

func splitLinkHeader(value string) []string {
	var links []string
	start, angleDepth := 0, 0
	inQuote, escaped := false, false
	for index, character := range value {
		if escaped {
			escaped = false
			continue
		}
		if inQuote && character == '\\' {
			escaped = true
			continue
		}
		if character == '"' {
			inQuote = !inQuote
			continue
		}
		if inQuote {
			continue
		}
		switch character {
		case '<':
			angleDepth++
		case '>':
			if angleDepth > 0 {
				angleDepth--
			}
		case ',':
			if angleDepth == 0 {
				links = append(links, strings.TrimSpace(value[start:index]))
				start = index + 1
			}
		}
	}
	if tail := strings.TrimSpace(value[start:]); tail != "" {
		links = append(links, tail)
	}
	return links
}

func hasNextRelation(parameters string) bool {
	for _, parameter := range strings.Split(parameters, ";") {
		name, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "rel") {
			continue
		}
		value := strings.Trim(strings.TrimSpace(raw), "\"")
		for _, relation := range strings.Fields(value) {
			if strings.EqualFold(relation, "next") {
				return true
			}
		}
	}
	return false
}
