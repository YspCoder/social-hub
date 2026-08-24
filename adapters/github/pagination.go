package github

import (
	"net/url"
	"strconv"
	"strings"
)

func pageLinks(value string) PageLinks {
	result := PageLinks{Raw: boundedMessage(value, 16_384)}
	for _, link := range splitLinkHeader(value) {
		start := strings.IndexByte(link, '<')
		end := strings.IndexByte(link, '>')
		if start < 0 || end <= start {
			continue
		}
		target, err := url.Parse(strings.TrimSpace(link[start+1 : end]))
		if err != nil {
			continue
		}
		page, err := strconv.Atoi(target.Query().Get("page"))
		if err != nil || page <= 0 {
			continue
		}
		for _, relation := range linkRelations(link[end+1:]) {
			pageCopy := page
			switch relation {
			case "next":
				result.NextPage = &pageCopy
			case "prev":
				result.PreviousPage = &pageCopy
			case "first":
				result.FirstPage = &pageCopy
			case "last":
				result.LastPage = &pageCopy
			}
		}
	}
	return result
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

func linkRelations(parameters string) []string {
	var relations []string
	for _, parameter := range strings.Split(parameters, ";") {
		name, raw, found := strings.Cut(strings.TrimSpace(parameter), "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "rel") {
			continue
		}
		value := strings.Trim(strings.TrimSpace(raw), "\"")
		for _, relation := range strings.Fields(value) {
			relations = append(relations, strings.ToLower(relation))
		}
	}
	return relations
}
