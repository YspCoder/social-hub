package matrix

import (
	"crypto/rand"
	"encoding/base64"
	"mime"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxOpaqueLength = 4096

func validUserID(value string) bool { return validMatrixID(value, '@') }
func validRoomID(value string) bool { return validMatrixID(value, '!') }
func validEventID(value string) bool {
	return strings.HasPrefix(value, "$") && validOpaque(value, maxOpaqueLength)
}

func validMatrixID(value string, sigil byte) bool {
	if len(value) < 4 || value[0] != sigil || !validOpaque(value, maxOpaqueLength) {
		return false
	}
	separator := strings.LastIndexByte(value, ':')
	return separator > 1 && separator < len(value)-1
}

func validOpaque(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !strings.ContainsFunc(value, unicode.IsControl)
}

func validText(value string) bool {
	return strings.TrimSpace(value) != "" && len(value) <= 1<<20 && utf8.ValidString(value) && !strings.ContainsFunc(value, unsafeControl)
}

func validFilename(value string) bool {
	return validOpaque(value, 1024) && !strings.ContainsAny(value, `/\`) && value != "." && value != ".."
}

func validMIME(value string) bool {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	parts := strings.Split(mediaType, "/")
	return err == nil && len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.Contains(mediaType, "*")
}

func validMXCURI(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "mxc" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	mediaID := strings.TrimPrefix(parsed.Path, "/")
	return strings.Count(parsed.Path, "/") == 1 && validOpaque(mediaID, maxOpaqueLength)
}

func unsafeControl(character rune) bool {
	return unicode.IsControl(character) && character != '\n' && character != '\r' && character != '\t'
}

func randomTransactionID() (string, error) {
	data := make([]byte, 18)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return "socialhub_" + base64.RawURLEncoding.EncodeToString(data), nil
}
