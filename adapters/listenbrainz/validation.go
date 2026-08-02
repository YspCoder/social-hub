package listenbrainz

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const (
	maxTextLength       = 2048
	maxCredentialLength = 1024
	maxListensPerImport = 1000
	maxListenPageSize   = 1000
	maxPlaylistPageSize = 100
	maxTagsPerListen    = 50
	maxTagLength        = 64
	maxListenBytes      = 10_240
	maxRequestBytes     = 10_240_000
)

func validEndpoint(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" &&
		parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.RawPath == "" &&
		!strings.Contains(parsed.Path, "..")
}

func compatibleAccount(account socialhub.AccountConfig) bool {
	return account.ClientID == "" && account.AppID == "" && account.SecretRef == "" && account.TokenStore == "" &&
		account.Webhook.SecretRef == "" && account.Webhook.TokenRef == "" && account.Webhook.AESKeyRef == "" &&
		account.Approval.AccountType == "" && len(account.Approval.Scopes) == 0
}

func validCredential(value string) bool {
	return value != "" && len(value) <= maxCredentialLength && strings.TrimSpace(value) == value && !containsControl(value)
}

func validUsername(value string) bool {
	return validText(value, 255) && !strings.ContainsAny(value, "/?#\\")
}

func validText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && strings.TrimSpace(value) == value && !containsControl(value)
}

func optionalText(value string, maximum int) bool {
	return value == "" || validText(value, maximum)
}

func containsControl(value string) bool {
	for _, character := range value {
		if unicode.IsControl(character) {
			return true
		}
	}
	return false
}

func validMBID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
				return false
			}
		}
	}
	return true
}

func validHTTPURL(value string) bool {
	if value == "" {
		return true
	}
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func validateOffset(cursor string, maximumPageSize int) (int, error) {
	if cursor == "" {
		return 0, nil
	}
	offset, err := strconv.Atoi(cursor)
	maxInt := int(^uint(0) >> 1)
	if err != nil || offset < 0 || offset > maxInt-maximumPageSize || strconv.Itoa(offset) != cursor {
		return 0, invalidArgument("pagination", "cursor must be a canonical nonnegative integer offset")
	}
	return offset, nil
}

func validatePage(maxResults, maximum int) error {
	if maxResults < 0 || maxResults > maximum {
		return invalidArgument("pagination", "max_results is outside the platform limit")
	}
	return nil
}

func validateTrackMetadata(metadata SubmissionTrackMetadata) error {
	if !validText(metadata.ArtistName, maxTextLength) || !validText(metadata.TrackName, maxTextLength) ||
		!optionalText(metadata.ReleaseName, maxTextLength) {
		return invalidArgument("submit_listens", "artist_name and track_name are required bounded strings")
	}
	additional := metadata.AdditionalInfo
	if additional == nil {
		return nil
	}
	if additional.DurationMS < 0 || additional.Duration < 0 || additional.DurationPlayed < 0 ||
		(additional.DurationMS > 0 && additional.Duration > 0) || len(additional.Tags) > maxTagsPerListen ||
		!validMBIDs(additional.ArtistMBIDs) || !validMBIDs(additional.WorkMBIDs) ||
		!validOptionalMBID(additional.ReleaseGroupMBID) || !validOptionalMBID(additional.ReleaseMBID) ||
		!validOptionalMBID(additional.RecordingMBID) || !validOptionalMBID(additional.TrackMBID) ||
		!validHTTPURL(additional.SpotifyID) || !validHTTPURL(additional.OriginURL) {
		return invalidArgument("submit_listens", "additional_info contains invalid identifiers, sizes, or URLs")
	}
	for _, tag := range additional.Tags {
		if !validText(tag, maxTextLength) || utf8.RuneCountInString(tag) > maxTagLength {
			return invalidArgument("submit_listens", "tags must be nonempty and at most 64 characters")
		}
	}
	texts := []string{
		additional.TrackNumber, additional.ISRC, additional.Label, additional.MediaPlayer, additional.MediaPlayerVersion,
		additional.SubmissionClient, additional.SubmissionClientVersion, additional.OriginalSubmissionClient,
		additional.MusicService, additional.MusicServiceName,
	}
	for _, value := range texts {
		if !optionalText(value, maxTextLength) {
			return invalidArgument("submit_listens", "additional_info contains an invalid text value")
		}
	}
	return nil
}

func validOptionalMBID(value string) bool { return value == "" || validMBID(value) }

func validMBIDs(values []string) bool {
	for _, value := range values {
		if !validMBID(value) {
			return false
		}
	}
	return true
}

func validateListen(listen ListenSubmission) error {
	if listen.ListenedAt <= 0 {
		return invalidArgument("submit_listens", "listened_at must be a positive Unix timestamp")
	}
	if err := validateTrackMetadata(listen.TrackMetadata); err != nil {
		return err
	}
	encoded, err := json.Marshal(listen)
	if err != nil || len(encoded) > maxListenBytes {
		return invalidArgument("submit_listens", "listen exceeds the 10,240-byte platform limit")
	}
	return nil
}

func validatePayload(payload any) error {
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) > maxRequestBytes {
		return invalidArgument("submit_listens", "request exceeds the 10,240,000-byte platform limit")
	}
	return nil
}

func validateCallOptions(operation string, options ...socialhub.CallOption) error {
	resolved, err := socialhub.ResolveCallOptions(options...)
	if err != nil {
		return err
	}
	if len(resolved.Fields) > 0 {
		return invalidArgument(operation, "field selection is fixed by the typed operation")
	}
	if resolved.IdempotencyKey != "" {
		return invalidArgument(operation, "ListenBrainz does not document idempotency-key support")
	}
	return nil
}
