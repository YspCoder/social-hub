package tvmaze

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestHTTPErrorClassification(t *testing.T) {
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
		header := http.Header{}
		header.Set("Retry-After", "7")
		header.Set("X-Correlation-ID", "request-1")
		err := decodeHTTPError(test.status, header, []byte(`{"name":"Failure","message":"details","code":12,"status":422}`))
		var platformErr *socialhub.Error
		if !errors.As(err, &platformErr) || platformErr.Code != test.code || platformErr.Class != test.class ||
			platformErr.HTTPStatus != test.status || platformErr.PlatformCode != "12" || platformErr.PlatformMessage != "details" ||
			platformErr.RequestID != "request-1" || platformErr.RetryAfter != 7*time.Second {
			t.Fatalf("status=%d error=%#v", test.status, platformErr)
		}
	}

	header := http.Header{}
	header.Set("X-Request-ID", strings.Repeat("r", 300))
	err := decodeHTTPError(http.StatusBadRequest, header, []byte(`{"name":"Failure"}`))
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.PlatformMessage != "Failure" || len(platformErr.RequestID) != 256 {
		t.Fatalf("bounded error=%#v", platformErr)
	}
	if parseRetryAfter("bad") != 0 || parseRetryAfter("-1") != 0 || parseRetryAfter("86401") != 0 ||
		bounded(strings.Repeat("界", 5), 3) != strings.Repeat("界", 3) || firstNonEmpty("", " value ") != " value " {
		t.Fatal("error helpers accepted invalid input")
	}
}

func TestOperationValidationAndCallOptions(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()
	_, client := newTestClient(t, server)
	ctx := context.Background()
	zeroDate := time.Time{}
	lowerCountry := "us"

	tests := []func() error{
		func() error { _, err := client.SearchShows(ctx, " "); return err },
		func() error { _, err := client.GetShow(ctx, 0); return err },
		func() error { _, err := client.LookupShow(ctx, LookupShowRequest{}); return err },
		func() error { _, err := client.LookupShow(ctx, LookupShowRequest{IMDB: "bad"}); return err },
		func() error { _, err := client.LookupShow(ctx, LookupShowRequest{TVDB: -1}); return err },
		func() error { _, err := client.LookupShow(ctx, LookupShowRequest{TVDB: 1, TVRage: 2}); return err },
		func() error { _, err := client.ListEpisodes(ctx, 0, false); return err },
		func() error { _, err := client.GetEpisode(ctx, 0); return err },
		func() error { _, err := client.GetEpisodeByNumber(ctx, 1, 0, 1); return err },
		func() error { _, err := client.ListEpisodesByDate(ctx, 1, time.Time{}); return err },
		func() error { _, err := client.ListSeasons(ctx, 0); return err },
		func() error { _, err := client.ListSeasonEpisodes(ctx, 0); return err },
		func() error { _, err := client.ListCast(ctx, 0); return err },
		func() error { _, err := client.ListCrew(ctx, 0); return err },
		func() error { _, err := client.ListSchedule(ctx, ScheduleRequest{Country: "us"}); return err },
		func() error { _, err := client.ListSchedule(ctx, ScheduleRequest{Date: &zeroDate}); return err },
		func() error {
			_, err := client.ListWebSchedule(ctx, WebScheduleRequest{Country: &lowerCountry})
			return err
		},
		func() error { _, err := client.ListWebSchedule(ctx, WebScheduleRequest{Date: &zeroDate}); return err },
		func() error { _, err := client.SearchPeople(ctx, strings.Repeat("x", maxQueryLength+1)); return err },
		func() error { _, err := client.GetPerson(ctx, 0); return err },
		func() error { _, err := client.ListCastCredits(ctx, 0); return err },
		func() error { _, err := client.ListCrewCredits(ctx, 0); return err },
		func() error { _, err := client.ListShowUpdates(ctx, "year"); return err },
		func() error { _, err := client.SearchShows(ctx, "valid", socialhub.WithFields("x")); return err },
		func() error {
			_, err := client.SearchShows(ctx, "valid", socialhub.WithIdempotencyKey("key"))
			return err
		},
		func() error {
			_, err := client.SearchShows(ctx, "valid", socialhub.WithCallTimeout(-time.Second))
			return err
		},
	}
	for index, call := range tests {
		if err := call(); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("case %d expected invalid argument, got %v", index, err)
		}
	}
}

func TestTransportAndDecodeErrorsPreserveOperation(t *testing.T) {
	responses := []struct {
		status int
		body   string
	}{
		{http.StatusUnprocessableEntity, `{"name":"Unprocessable entity","message":"Not a valid ISO country code","status":422}`},
		{http.StatusOK, `{`},
	}
	call := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		response := responses[call]
		call++
		writeJSON(writer, response.status, response.body)
	}))
	defer server.Close()
	_, client := newTestClient(t, server)

	_, err := client.SearchShows(context.Background(), "valid")
	var platformErr *socialhub.Error
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodeInvalidArgument || platformErr.Op != "search_shows" {
		t.Fatalf("HTTP error=%#v", platformErr)
	}
	_, err = client.SearchPeople(context.Background(), "valid")
	if !errors.As(err, &platformErr) || platformErr.Code != socialhub.CodePlatformError || platformErr.Op != "search_people" {
		t.Fatalf("decode error=%#v", platformErr)
	}
}
