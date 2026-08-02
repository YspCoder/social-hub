package listenbrainz

import (
	"encoding/json"
	"time"
)

// User is a ListenBrainz account returned by user search.
type User struct {
	UserName string `json:"user_name"`
}

// AdditionalInfo contains client-submitted listen metadata. Values in this
// object are not canonical MusicBrainz mappings.
type AdditionalInfo struct {
	ArtistMBIDs              []string        `json:"artist_mbids,omitempty"`
	ArtistNames              []string        `json:"artist_names,omitempty"`
	ReleaseGroupMBID         string          `json:"release_group_mbid,omitempty"`
	ReleaseMBID              string          `json:"release_mbid,omitempty"`
	RecordingMBID            string          `json:"recording_mbid,omitempty"`
	RecordingMSID            string          `json:"recording_msid,omitempty"`
	TrackMBID                string          `json:"track_mbid,omitempty"`
	WorkMBIDs                []string        `json:"work_mbids,omitempty"`
	ISRC                     string          `json:"isrc,omitempty"`
	SpotifyID                string          `json:"spotify_id,omitempty"`
	Tags                     []string        `json:"tags,omitempty"`
	DurationMS               int64           `json:"duration_ms,omitempty"`
	Duration                 int64           `json:"duration,omitempty"`
	DurationPlayed           int64           `json:"duration_played,omitempty"`
	TrackNumber              json.RawMessage `json:"tracknumber,omitempty"`
	Label                    string          `json:"label,omitempty"`
	MediaPlayer              string          `json:"media_player,omitempty"`
	MediaPlayerVersion       string          `json:"media_player_version,omitempty"`
	SubmissionClient         string          `json:"submission_client,omitempty"`
	SubmissionClientVersion  string          `json:"submission_client_version,omitempty"`
	OriginalSubmissionClient string          `json:"original_submission_client,omitempty"`
	MusicService             string          `json:"music_service,omitempty"`
	MusicServiceName         string          `json:"music_service_name,omitempty"`
	OriginURL                string          `json:"origin_url,omitempty"`
}

// ArtistCredit is a server-resolved MusicBrainz artist credit.
type ArtistCredit struct {
	ArtistMBID       string `json:"artist_mbid"`
	ArtistCreditName string `json:"artist_credit_name"`
	JoinPhrase       string `json:"join_phrase"`
}

// URLRelation is a MusicBrainz streaming or download relationship.
type URLRelation struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// MBIDMapping is ListenBrainz's read-only canonical MusicBrainz resolution.
// It intentionally does not appear in submission types.
type MBIDMapping struct {
	RecordingMBID  string         `json:"recording_mbid"`
	RecordingName  string         `json:"recording_name,omitempty"`
	ReleaseMBID    string         `json:"release_mbid,omitempty"`
	ArtistMBIDs    []string       `json:"artist_mbids,omitempty"`
	Artists        []ArtistCredit `json:"artists,omitempty"`
	CAAID          int64          `json:"caa_id,omitempty"`
	CAAReleaseMBID string         `json:"caa_release_mbid,omitempty"`
	URLRelations   []URLRelation  `json:"url_rels,omitempty"`
}

// TrackMetadata is the track metadata returned with a listen or feedback item.
type TrackMetadata struct {
	ArtistName     string          `json:"artist_name"`
	TrackName      string          `json:"track_name"`
	ReleaseName    string          `json:"release_name,omitempty"`
	AdditionalInfo *AdditionalInfo `json:"additional_info,omitempty"`
	MBIDMapping    *MBIDMapping    `json:"mbid_mapping,omitempty"`
}

// Listen is one stored or currently playing listen.
type Listen struct {
	InsertedAt    int64         `json:"inserted_at,omitempty"`
	ListenedAt    *int64        `json:"listened_at,omitempty"`
	RecordingMSID string        `json:"recording_msid,omitempty"`
	TrackMetadata TrackMetadata `json:"track_metadata"`
	UserName      string        `json:"user_name,omitempty"`
}

// ListenPage preserves ListenBrainz's timestamp pagination metadata.
type ListenPage struct {
	Count          int      `json:"count"`
	UserID         string   `json:"user_id"`
	Listens        []Listen `json:"listens"`
	LatestListenTS int64    `json:"latest_listen_ts,omitempty"`
	OldestListenTS int64    `json:"oldest_listen_ts,omitempty"`
}

// SubmissionAdditionalInfo is the writable subset of additional_info.
type SubmissionAdditionalInfo struct {
	ArtistMBIDs              []string `json:"artist_mbids,omitempty"`
	ReleaseGroupMBID         string   `json:"release_group_mbid,omitempty"`
	ReleaseMBID              string   `json:"release_mbid,omitempty"`
	RecordingMBID            string   `json:"recording_mbid,omitempty"`
	TrackMBID                string   `json:"track_mbid,omitempty"`
	WorkMBIDs                []string `json:"work_mbids,omitempty"`
	TrackNumber              string   `json:"tracknumber,omitempty"`
	ISRC                     string   `json:"isrc,omitempty"`
	SpotifyID                string   `json:"spotify_id,omitempty"`
	Tags                     []string `json:"tags,omitempty"`
	DurationMS               int64    `json:"duration_ms,omitempty"`
	Duration                 int64    `json:"duration,omitempty"`
	DurationPlayed           int64    `json:"duration_played,omitempty"`
	Label                    string   `json:"label,omitempty"`
	MediaPlayer              string   `json:"media_player,omitempty"`
	MediaPlayerVersion       string   `json:"media_player_version,omitempty"`
	SubmissionClient         string   `json:"submission_client,omitempty"`
	SubmissionClientVersion  string   `json:"submission_client_version,omitempty"`
	OriginalSubmissionClient string   `json:"original_submission_client,omitempty"`
	MusicService             string   `json:"music_service,omitempty"`
	MusicServiceName         string   `json:"music_service_name,omitempty"`
	OriginURL                string   `json:"origin_url,omitempty"`
}

// SubmissionTrackMetadata contains only fields accepted by submit-listens.
type SubmissionTrackMetadata struct {
	ArtistName     string                    `json:"artist_name"`
	TrackName      string                    `json:"track_name"`
	ReleaseName    string                    `json:"release_name,omitempty"`
	AdditionalInfo *SubmissionAdditionalInfo `json:"additional_info,omitempty"`
}

// ListenSubmission is a completed listen submitted as single or import.
type ListenSubmission struct {
	ListenedAt    int64                   `json:"listened_at"`
	TrackMetadata SubmissionTrackMetadata `json:"track_metadata"`
}

// PlayingNowSubmission omits listened_at as required by the API.
type PlayingNowSubmission struct {
	TrackMetadata SubmissionTrackMetadata `json:"track_metadata"`
}

// SubmissionResult is the acknowledgement returned by submit-listens.
type SubmissionResult struct {
	Status  string `json:"status,omitempty"`
	Payload *struct {
		RecordingMSID string `json:"recording_msid,omitempty"`
	} `json:"payload,omitempty"`
}

// TokenValidation reports whether the configured user token is valid.
type TokenValidation struct {
	Code     int    `json:"code"`
	Message  string `json:"message"`
	Valid    bool   `json:"valid"`
	UserName string `json:"user_name,omitempty"`
}

// DeleteListenRequest identifies a listen scheduled for deletion.
type DeleteListenRequest struct {
	ListenedAt    int64  `json:"listened_at"`
	RecordingMSID string `json:"recording_msid"`
}

// FeedbackScore is ListenBrainz's love, hate, or remove-feedback value.
type FeedbackScore int

const (
	FeedbackHate   FeedbackScore = -1
	FeedbackRemove FeedbackScore = 0
	FeedbackLove   FeedbackScore = 1
)

// FeedbackSubmission changes feedback by canonical MBID, MSID, or both.
type FeedbackSubmission struct {
	RecordingMBID string        `json:"recording_mbid,omitempty"`
	RecordingMSID string        `json:"recording_msid,omitempty"`
	Score         FeedbackScore `json:"score"`
}

// Feedback is one recording-feedback item.
type Feedback struct {
	Created       int64          `json:"created,omitempty"`
	RecordingMBID string         `json:"recording_mbid,omitempty"`
	RecordingMSID *string        `json:"recording_msid"`
	Score         FeedbackScore  `json:"score"`
	TrackMetadata *TrackMetadata `json:"track_metadata,omitempty"`
	UserID        string         `json:"user_id"`
}

// Playlist is a JSPF playlist with MusicBrainz extensions preserved verbatim.
type Playlist struct {
	Annotation string                     `json:"annotation,omitempty"`
	Creator    string                     `json:"creator,omitempty"`
	Date       *time.Time                 `json:"date,omitempty"`
	Extension  map[string]json.RawMessage `json:"extension,omitempty"`
	Identifier string                     `json:"identifier,omitempty"`
	Title      string                     `json:"title"`
	Track      []PlaylistTrack            `json:"track"`
}

// PlaylistTrack is a JSPF track; URL-keyed extensions remain lossless.
type PlaylistTrack struct {
	Album      string                     `json:"album,omitempty"`
	Annotation string                     `json:"annotation,omitempty"`
	Creator    string                     `json:"creator,omitempty"`
	Duration   int64                      `json:"duration,omitempty"`
	Extension  map[string]json.RawMessage `json:"extension,omitempty"`
	Identifier []string                   `json:"identifier,omitempty"`
	Image      string                     `json:"image,omitempty"`
	Location   []string                   `json:"location,omitempty"`
	Title      string                     `json:"title,omitempty"`
}
