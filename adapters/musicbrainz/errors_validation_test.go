package musicbrainz

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

func TestHTTPErrorClassification(t *testing.T) {
	tests := []struct {
		status int
		code   socialhub.ErrorCode
		class  socialhub.ErrorClass
		retry  time.Duration
	}{
		{http.StatusBadRequest, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, 0},
		{http.StatusMethodNotAllowed, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, 0},
		{http.StatusUnprocessableEntity, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, 0},
		{http.StatusUnauthorized, socialhub.CodeUnauthenticated, socialhub.ClassUserAction, 0},
		{http.StatusForbidden, socialhub.CodePermissionDenied, socialhub.ClassUserAction, 0},
		{http.StatusNotFound, socialhub.CodeNotFound, socialhub.ClassPermanent, 0},
		{http.StatusGone, socialhub.CodeNotFound, socialhub.ClassPermanent, 0},
		{http.StatusConflict, socialhub.CodeConflict, socialhub.ClassPermanent, 0},
		{http.StatusTooManyRequests, socialhub.CodeRateLimited, socialhub.ClassRetryable, time.Second},
		{http.StatusServiceUnavailable, socialhub.CodeRateLimited, socialhub.ClassRetryable, time.Second},
		{http.StatusBadGateway, socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, 0},
		{http.StatusTeapot, socialhub.CodePlatformError, socialhub.ClassPermanent, 0},
	}
	for _, test := range tests {
		header := http.Header{}
		header.Set("X-Correlation-ID", "request-1")
		err := decodeHTTPError(test.status, header, []byte(`{"error":"details","help":"https://musicbrainz.org/doc"}`))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class ||
			platformErr.HTTPStatus != test.status || platformErr.PlatformMessage != "details" || platformErr.RequestID != "request-1" ||
			platformErr.RetryAfter != test.retry {
			t.Fatalf("status=%d error=%#v", test.status, platformErr)
		}
	}
	header := http.Header{}
	header.Set("Retry-After", "7")
	header.Set("X-Request-ID", strings.Repeat("r", 300))
	err := decodeHTTPError(http.StatusServiceUnavailable, header, []byte(`{"error":"slow"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.RetryAfter != 7*time.Second || len(platformErr.RequestID) != 256 {
		t.Fatalf("bounded error=%#v", platformErr)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("86401") != 0 ||
		bounded(strings.Repeat("界", 5), 3) != strings.Repeat("界", 3) || firstNonEmpty("", " value ") != " value " {
		t.Fatal("error helpers accepted invalid input")
	}
}

func TestOperationValidationPaginationAndCallOptions(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	tests := []func() error{
		func() error { _, err := client.SearchArtists(ctx, SearchRequest{}); return err },
		func() error { _, err := client.SearchArtists(ctx, SearchRequest{Query: "x", Limit: -1}); return err },
		func() error { _, err := client.SearchArtists(ctx, SearchRequest{Query: "x", Limit: 101}); return err },
		func() error { _, err := client.SearchArtists(ctx, SearchRequest{Query: "x", Cursor: "01"}); return err },
		func() error {
			_, err := client.SearchArtists(ctx, SearchRequest{Query: "x", Cursor: strings.Repeat("9", 100)})
			return err
		},
		func() error { _, err := client.GetArtist(ctx, "bad"); return err },
		func() error { _, err := client.GetReleaseGroup(ctx, "bad"); return err },
		func() error { _, err := client.GetRelease(ctx, "bad"); return err },
		func() error { _, err := client.GetRecording(ctx, "bad"); return err },
		func() error { _, err := client.GetWork(ctx, "bad"); return err },
		func() error { _, err := client.ListArtistReleaseGroups(ctx, "bad", BrowseRequest{}); return err },
		func() error {
			_, err := client.ListArtistRecordings(ctx, artistMBID, BrowseRequest{Limit: 101})
			return err
		},
		func() error {
			_, err := client.SearchArtists(ctx, SearchRequest{Query: "x"}, socialhub.WithFields("x"))
			return err
		},
		func() error {
			_, err := client.SearchArtists(ctx, SearchRequest{Query: "x"}, socialhub.WithIdempotencyKey("key"))
			return err
		},
		func() error {
			_, err := client.SearchArtists(ctx, SearchRequest{Query: "x"}, socialhub.WithCallTimeout(-time.Second))
			return err
		},
	}
	for index, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d expected invalid argument, got %v", index, err)
		}
	}
	if !validMBID(artistMBID) || validMBID(strings.ToUpper(artistMBID)) || validMBID(artistMBID[:35]+"g") ||
		validQuery(" surrounded ") || validQuery("line\nbreak") || validUserAgent("") {
		t.Fatal("validation helper mismatch")
	}

	page, err := pageFromEnvelope("page", []int{1}, 3, 1, 1)
	if err != nil || !page.HasMore || page.NextCursor == nil || *page.NextCursor != "2" || page.PrevCursor == nil || *page.PrevCursor != "0" {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	last, err := pageFromEnvelope("page", []int{1}, 2, 1, 0)
	if err != nil || last.HasMore || last.NextCursor != nil || last.PrevCursor == nil || *last.PrevCursor != "0" {
		t.Fatalf("last page=%#v err=%v", last, err)
	}
	invalidPages := []struct {
		items  []int
		count  int
		offset int
	}{
		{nil, -1, 0}, {nil, 1, -1}, {nil, 1, 2}, {make([]int, 101), 101, 0}, {[]int{1, 2}, 1, 0}, {nil, 1, 0},
	}
	for _, invalid := range invalidPages {
		if _, err := pageFromEnvelope("page", invalid.items, invalid.count, invalid.offset, 1); err == nil {
			t.Fatalf("accepted invalid page %#v", invalid)
		}
	}
}

func TestTransportErrorsMalformedJSONAndRedirectRefusal(t *testing.T) {
	var targetCalls atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		targetCalls.Add(1)
		writeJSON(writer, http.StatusOK, `{"count":0,"offset":0,"artists":[]}`)
	}))
	defer target.Close()
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call++
		switch call {
		case 1:
			writeJSON(writer, http.StatusServiceUnavailable, `{"error":"Rate limit exceeded"}`)
		case 2:
			writeJSON(writer, http.StatusOK, `{`)
		default:
			http.Redirect(writer, request, target.URL, http.StatusFound)
		}
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	_, err := client.SearchArtists(context.Background(), SearchRequest{Query: "x"})
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || !errors.Is(err, socialhub.ErrRateLimited) || platformErr.Op != "search_artists" {
		t.Fatalf("rate error=%#v", platformErr)
	}
	_, err = client.SearchArtists(context.Background(), SearchRequest{Query: "x"})
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError || platformErr.Op != "search_artists" {
		t.Fatalf("decode error=%#v", platformErr)
	}
	if _, err := client.SearchArtists(context.Background(), SearchRequest{Query: "x"}); err == nil {
		t.Fatal("expected redirect error")
	}
	if targetCalls.Load() != 0 {
		t.Fatal("redirect target was followed")
	}
}
