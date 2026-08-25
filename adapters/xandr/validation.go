package xandr

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

func validOpaque(value string, maximum int) bool {
	if strings.TrimSpace(value) == "" || value != strings.TrimSpace(value) || len(value) > maximum || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validState(value State) bool {
	return value == "" || value == StateActive || value == StateInactive
}

func validResponseState(value State) bool {
	return value == StateActive || value == StateInactive
}

func validResponseText(value string, maximum int) bool {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > maximum {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func validSearch(value string) bool {
	if value == "" {
		return true
	}
	if value != strings.TrimSpace(value) || !utf8.ValidString(value) || utf8.RuneCountInString(value) > 256 {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validListOptions(value ListOptions) bool {
	return validState(value.State) && validSearch(value.Search) && value.StartElement >= 0 &&
		value.NumElements >= 0 && value.NumElements <= 100
}

func normalizedNumElements(value int) int {
	if value == 0 {
		return 100
	}
	return value
}
