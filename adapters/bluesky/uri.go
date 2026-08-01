package bluesky

import (
	"fmt"
	"strings"
)

type recordURI struct {
	Repo       string
	Collection string
	RecordKey  string
}

func parseRecordURI(value string) (recordURI, error) {
	if !strings.HasPrefix(value, "at://") || strings.ContainsAny(value, "\\?#%\t\r\n ") {
		return recordURI{}, fmt.Errorf("bluesky: invalid AT URI")
	}
	parts := strings.Split(strings.TrimPrefix(value, "at://"), "/")
	if len(parts) != 3 || !validDID(parts[0]) {
		return recordURI{}, fmt.Errorf("bluesky: AT URI must identify one record")
	}
	collection, recordKey := parts[1], parts[2]
	if !validCollection(collection) || !validRecordKey(recordKey) {
		return recordURI{}, fmt.Errorf("bluesky: invalid AT URI path")
	}
	return recordURI{Repo: parts[0], Collection: collection, RecordKey: recordKey}, nil
}

func validCollection(value string) bool {
	if value == "" || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || strings.Contains(value, "..") {
		return false
	}
	for _, char := range []byte(value) {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '.' || char == '-') {
			return false
		}
	}
	return true
}

func validRecordKey(value string) bool {
	if value == "" || len(value) > 512 || value == "." || value == ".." {
		return false
	}
	for _, char := range []byte(value) {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune(".-_:~", rune(char))) {
			return false
		}
	}
	return true
}
