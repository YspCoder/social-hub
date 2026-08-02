package listenbrainz

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorClassificationAndRateHeaders(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusMethodNotAllowed, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnprocessableEntity, socialhub.CodeInvalidArgument, socialhub.ClassPermanent},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusGone, socialhub.CodeNotFound, socialhub.ClassPermanent},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable},
		{http.StatusBadGateway, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent},
	}
	for _, test := range tests {
		header := make(http.Header)
		header.Set("X-RateLimit-Reset-In", "9")
		header.Set("X-Correlation-ID", "request-1")
		err := decodeHTTPError(test.status, header, []byte(`{"code":`+platformCode(test.status)+`,"error":"details"}`))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class ||
			platformErr.HTTPStatus != test.status || platformErr.PlatformMessage != "details" ||
			platformErr.PlatformCode != platformCode(test.status) || platformErr.RequestID != "request-1" || platformErr.RetryAfter != 9*time.Second {
			t.Fatalf("status=%d error=%#v", test.status, platformErr)
		}
	}
	header := make(http.Header)
	header.Set("Retry-After", "7")
	header.Set("X-RateLimit-Reset-In", "9")
	header.Set("X-Request-ID", strings.Repeat("r", 300))
	err := decodeHTTPError(http.StatusTooManyRequests, header, []byte(`{"message":"slow"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.RetryAfter != 7*time.Second || len(platformErr.RequestID) != 256 || platformErr.PlatformMessage != "slow" {
		t.Fatalf("bounded rate error=%#v", platformErr)
	}
	if parseRetryDuration("bad") != 0 || parseRetryDuration("-1") != 0 || parseRetryDuration("86401") != 0 ||
		bounded(strings.Repeat("界", 5), 3) != strings.Repeat("界", 3) || firstNonEmpty("", " value ") != " value " || platformCode(0) != "" {
		t.Fatal("error helper mismatch")
	}
}

func TestOperationValidationAndApprovalGates(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, public, _ := newTestClients(t, server)
	ctx := context.Background()
	validListen := ListenSubmission{ListenedAt: 1, TrackMetadata: testTrackMetadata()}
	longTag := strings.Repeat("x", maxTagLength+1)
	tooManyTags := make([]string, maxTagsPerListen+1)
	for index := range tooManyTags {
		tooManyTags[index] = "tag"
	}
	badMetadata := func(change func(*SubmissionAdditionalInfo)) ListenSubmission {
		input := validListen
		additional := *input.TrackMetadata.AdditionalInfo
		additional.Tags = append([]string(nil), additional.Tags...)
		change(&additional)
		input.TrackMetadata.AdditionalInfo = &additional
		return input
	}

	tests := []func() error{
		func() error { _, err := public.SearchUsers(ctx, ""); return err },
		func() error {
			_, err := public.ListListens(ctx, ListensRequest{MinTimestamp: 1, MaxTimestamp: 2})
			return err
		},
		func() error {
			_, err := public.ListListens(ctx, ListensRequest{Count: maxListenPageSize + 1})
			return err
		},
		func() error { _, err := public.GetPlayingNow(ctx, "bad/name"); return err },
		func() error { _, err := public.GetListenCount(ctx, "bad/name"); return err },
		func() error { _, err := public.SubmitSingle(ctx, ListenSubmission{}); return err },
		func() error {
			input := validListen
			input.TrackMetadata.ArtistName = ""
			_, err := public.SubmitSingle(ctx, input)
			return err
		},
		func() error {
			_, err := public.SubmitSingle(ctx, badMetadata(func(info *SubmissionAdditionalInfo) { info.RecordingMBID = "bad" }))
			return err
		},
		func() error {
			_, err := public.SubmitSingle(ctx, badMetadata(func(info *SubmissionAdditionalInfo) { info.DurationMS = -1 }))
			return err
		},
		func() error {
			_, err := public.SubmitSingle(ctx, badMetadata(func(info *SubmissionAdditionalInfo) { info.Duration = 1 }))
			return err
		},
		func() error {
			_, err := public.SubmitSingle(ctx, badMetadata(func(info *SubmissionAdditionalInfo) { info.OriginURL = "file:///tmp/a" }))
			return err
		},
		func() error {
			_, err := public.SubmitSingle(ctx, badMetadata(func(info *SubmissionAdditionalInfo) { info.Tags = tooManyTags }))
			return err
		},
		func() error {
			_, err := public.SubmitSingle(ctx, badMetadata(func(info *SubmissionAdditionalInfo) { info.Tags = []string{longTag} }))
			return err
		},
		func() error { _, err := public.SubmitImport(ctx, nil); return err },
		func() error {
			_, err := public.SubmitImport(ctx, make([]ListenSubmission, maxListensPerImport+1))
			return err
		},
		func() error { _, err := public.SubmitPlayingNow(ctx, PlayingNowSubmission{}, false); return err },
		func() error { return public.DeleteListen(ctx, DeleteListenRequest{}) },
		func() error { return public.SubmitFeedback(ctx, FeedbackSubmission{}) },
		func() error {
			return public.SubmitFeedback(ctx, FeedbackSubmission{RecordingMBID: recordingMBID, Score: 2})
		},
		func() error {
			score := FeedbackRemove
			_, err := public.ListFeedback(ctx, FeedbackListRequest{Score: &score})
			return err
		},
		func() error { _, err := public.ListFeedback(ctx, FeedbackListRequest{Cursor: "01"}); return err },
		func() error { _, err := public.SearchPlaylists(ctx, PlaylistSearchRequest{Query: "ab"}); return err },
		func() error {
			_, err := public.SearchPlaylists(ctx, PlaylistSearchRequest{Query: "valid", MaxResults: maxPlaylistPageSize + 1})
			return err
		},
		func() error {
			_, err := public.ListUserPlaylists(ctx, "rob", PlaylistPageRequest{Cursor: "-1"})
			return err
		},
		func() error { _, err := public.GetPlaylist(ctx, "bad", false); return err },
		func() error { _, err := public.SearchUsers(ctx, "rob", socialhub.WithFields("name")); return err },
		func() error {
			_, err := public.SearchUsers(ctx, "rob", socialhub.WithIdempotencyKey("key"))
			return err
		},
		func() error {
			_, err := public.SearchUsers(ctx, "rob", socialhub.WithCallTimeout(-time.Second))
			return err
		},
	}
	for index, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d expected invalid argument, got %v", index, err)
		}
	}

	if _, err := public.ValidateToken(ctx); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected token approval error, got %v", err)
	}
	if _, err := public.SubmitSingle(ctx, validListen); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected submission approval error, got %v", err)
	}
	if err := public.SubmitFeedback(ctx, FeedbackSubmission{RecordingMSID: recordingMSID, Score: FeedbackRemove}); !errors.Is(err, socialhub.ErrApprovalRequired) {
		t.Fatalf("expected feedback approval error, got %v", err)
	}
	if err := validatePayload(strings.Repeat("x", maxRequestBytes)); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("expected request-size error, got %v", err)
	}
	oversized := validListen
	oversized.TrackMetadata.ReleaseName = strings.Repeat("x", 1800)
	info := *oversized.TrackMetadata.AdditionalInfo
	info.MediaPlayer = strings.Repeat("m", 1800)
	info.MediaPlayerVersion = strings.Repeat("v", 1800)
	info.SubmissionClient = strings.Repeat("s", 1800)
	info.SubmissionClientVersion = strings.Repeat("c", 1800)
	info.MusicServiceName = strings.Repeat("n", 1800)
	oversized.TrackMetadata.AdditionalInfo = &info
	if err := validateListen(oversized); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("expected listen-size error, got %v", err)
	}

	if !validEndpoint("https://api.listenbrainz.org") || validEndpoint("https://user@example.test") ||
		!validUsername("user+name@example.test") || validUsername(" bad ") || validCredential(" bad ") ||
		!validMBID(recordingMBID) || validMBID(strings.ToUpper(recordingMBID)) || validHTTPURL("file:///tmp/a") {
		t.Fatal("validation helper mismatch")
	}
}

func TestPaginationHelpersAndInvalidResponses(t *testing.T) {
	page, err := offsetPage("page", []int{1}, 1, 1, 3, 1, 1)
	if err != nil || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "2" || page.PrevCursor == nil || *page.PrevCursor != "0" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	last, err := offsetPage("page", []int{1}, 1, 2, 3, 2, 1)
	if err != nil || last.HasMore || last.NextCursor != nil || last.PrevCursor == nil || *last.PrevCursor != "1" {
		t.Fatalf("last=%#v err=%v", last, err)
	}
	empty, err := offsetPage("page", []int{}, 0, 50, 50, 50, 0)
	if err != nil || empty.PrevCursor == nil || *empty.PrevCursor != "25" {
		t.Fatalf("empty=%#v err=%v", empty, err)
	}
	invalid := []struct{ count, offset, total, expected int }{
		{0, 0, 1, 0}, {1, 1, 1, 1}, {1, 1, 2, 0},
	}
	for _, item := range invalid {
		if _, err := offsetPage("page", []int{1}, item.count, item.offset, item.total, item.expected, 1); err == nil {
			t.Fatalf("accepted invalid page %#v", item)
		}
	}
}

func TestTransportErrorsMalformedJSONAndRedirectRefusal(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalls.Add(1)
		writeJSON(writer, http.StatusOK, `{"users":[]}`)
	}))
	defer target.Close()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch calls.Add(1) {
		case 1:
			writer.Header().Set("X-RateLimit-Reset-In", "5")
			writeJSON(writer, http.StatusTooManyRequests, `{"code":429,"error":"too many requests"}`)
		case 2:
			writeJSON(writer, http.StatusOK, `{`)
		default:
			http.Redirect(writer, request, target.URL, http.StatusFound)
		}
	}))
	defer server.Close()
	_, public, _ := newTestClients(t, server)

	_, err := public.SearchUsers(context.Background(), "rob")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrRateLimited) || platformErr.Op != "search_users" || platformErr.RetryAfter != 5*time.Second {
		t.Fatalf("rate error=%#v", platformErr)
	}
	if _, err := public.SearchUsers(context.Background(), "rob"); !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError || platformErr.Op != "search_users" {
		t.Fatalf("decode error=%#v", platformErr)
	}
	if _, err := public.SearchUsers(context.Background(), "rob"); err == nil {
		t.Fatal("expected redirect refusal")
	}
	if targetCalls.Load() != 0 {
		t.Fatal("redirect target was followed")
	}
}
