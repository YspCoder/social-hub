package strava

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"social-hub/pkg/socialhub"
)

func TestManualActivityCreateAndUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/activities":
			if request.Header.Get("Content-Type") != "application/x-www-form-urlencoded" || request.ParseForm() != nil {
				t.Fatalf("create content type=%q", request.Header.Get("Content-Type"))
			}
			if request.Form.Get("name") != "Lunch Ride" || request.Form.Get("sport_type") != "Ride" ||
				request.Form.Get("start_date_local") != "2026-08-03T09:00:00+08:00" || request.Form.Get("elapsed_time") != "3600" ||
				request.Form.Get("description") != "Intervals" || request.Form.Get("distance") != "25000.5" ||
				request.Form.Get("trainer") != "0" || request.Form.Get("commute") != "1" {
				t.Errorf("create form=%v", request.Form)
			}
			writeJSON(writer, http.StatusCreated, activityJSON(testActivityID, testAthleteID))
		case request.Method == http.MethodPut && request.URL.Path == "/api/activities/"+testActivityID:
			var body map[string]any
			if json.NewDecoder(request.Body).Decode(&body) != nil || body["name"] != "Evening Ride" || body["sport_type"] != "GravelRide" ||
				body["description"] != "" || body["trainer"] != true || body["commute"] != false || body["hide_from_home"] != true || body["gear_id"] != "none" {
				t.Errorf("update body=%v", body)
			}
			writeJSON(writer, http.StatusOK, activityJSON(testActivityID, testAthleteID))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, []string{"read", "activity:read", "activity:write"})
	local := time.Date(2026, 8, 3, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	distance, no, yes := 25000.5, false, true
	activity, err := client.CreateManualActivity(context.Background(), ManualActivityRequest{
		Name: "Lunch Ride", SportType: SportRide, StartDateLocal: local, ElapsedTime: time.Hour,
		Description: "Intervals", DistanceMeters: &distance, Trainer: &no, Commute: &yes,
	})
	if err != nil || activity.ID != testActivityID || activity.AthleteID != testAthleteID || activity.SportType != SportRide || len(activity.Raw) == 0 {
		t.Fatalf("activity=%#v err=%v", activity, err)
	}
	name, description, gear, sport := "Evening Ride", "", "none", SportGravelRide
	activity, err = client.UpdateActivity(context.Background(), testActivityID, ActivityUpdateRequest{
		Name: &name, SportType: &sport, Description: &description, Trainer: &yes, Commute: &no, HideFromHome: &yes, GearID: &gear,
	})
	if err != nil || activity.ID != testActivityID {
		t.Fatalf("updated=%#v err=%v", activity, err)
	}
}

func TestActivityUploadAndStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/api/uploads":
			if request.ParseMultipartForm(1<<20) != nil {
				t.Fatal("parse multipart")
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			body, _ := io.ReadAll(file)
			if header.Filename != "ride.fit" || string(body) != "FITDATA" || request.FormValue("data_type") != "fit" ||
				request.FormValue("sport_type") != "Ride" || request.FormValue("name") != "Ride" ||
				request.FormValue("description") != "Uploaded" || request.FormValue("external_id") != "device-activity-1" ||
				request.FormValue("trainer") != "0" || request.FormValue("commute") != "1" {
				t.Errorf("file=%q body=%q form=%v", header.Filename, body, request.MultipartForm.Value)
			}
			writeJSON(writer, http.StatusCreated, `{"id":`+testUploadID+`,"id_str":"`+testUploadID+`","external_id":"device-activity-1.fit","error":null,"status":"Your activity is still being processed.","activity_id":null}`)
		case request.Method == http.MethodGet && request.URL.Path == "/api/uploads/"+testUploadID:
			writeJSON(writer, http.StatusOK, `{"id":`+testUploadID+`,"id_str":"`+testUploadID+`","external_id":"device-activity-1.fit","error":null,"status":"Your activity is ready.","activity_id":`+testActivityID+`}`)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, []string{"read", "activity:read", "activity:write"})
	no, yes := false, true
	upload, err := client.UploadActivity(context.Background(), ActivityUploadRequest{
		Filename: `C:\activities\ride.fit`, Size: 7, DataType: UploadFIT, SportType: SportRide,
		Name: "Ride", Description: "Uploaded", ExternalID: "device-activity-1", Trainer: &no, Commute: &yes,
	}, strings.NewReader("FITDATA"))
	if err != nil || upload.ID != testUploadID || upload.ActivityID != nil || upload.Error != nil || len(upload.Raw) == 0 {
		t.Fatalf("upload=%#v err=%v", upload, err)
	}
	upload, err = client.GetUpload(context.Background(), testUploadID)
	if err != nil || upload.ActivityID == nil || *upload.ActivityID != testActivityID || upload.Status != "Your activity is ready." {
		t.Fatalf("status=%#v err=%v", upload, err)
	}
}

func TestActivityAndUploadValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/api/uploads" {
			_, _ = io.Copy(io.Discard, request.Body)
			writeJSON(writer, http.StatusCreated, `{"id":1,"status":"processing"}`)
			return
		}
		http.NotFound(writer, request)
	}))
	defer server.Close()
	_, client := newTestAdapter(t, server, false, nil)

	invalidManual := []ManualActivityRequest{
		{},
		{Name: "Ride", SportType: "FutureSport", StartDateLocal: testNow, ElapsedTime: time.Hour},
		{Name: "Ride", SportType: SportRide, StartDateLocal: testNow, ElapsedTime: time.Millisecond},
	}
	negative := -1.0
	invalidManual = append(invalidManual, ManualActivityRequest{Name: "Ride", SportType: SportRide, StartDateLocal: testNow, ElapsedTime: time.Hour, DistanceMeters: &negative})
	for index, input := range invalidManual {
		if _, err := client.CreateManualActivity(context.Background(), input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("manual %d error=%v", index, err)
		}
	}
	if _, err := client.UpdateActivity(context.Background(), "bad", ActivityUpdateRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("update ID error=%v", err)
	}
	if _, err := client.UpdateActivity(context.Background(), testActivityID, ActivityUpdateRequest{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("empty update error=%v", err)
	}
	empty, invalidSport, invalidGear := "", SportType("FutureSport"), "\n"
	for index, input := range []ActivityUpdateRequest{{Name: &empty}, {SportType: &invalidSport}, {GearID: &invalidGear}} {
		if _, err := client.UpdateActivity(context.Background(), testActivityID, input); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("update %d error=%v", index, err)
		}
	}

	valid := ActivityUploadRequest{Filename: "activity.gpx", Size: 1, DataType: UploadGPX}
	invalidUploads := []ActivityUploadRequest{
		{},
		{Filename: "activity.gpx", Size: 1, DataType: "zip"},
		{Filename: "activity.gpx", Size: 1, DataType: UploadGPX, SportType: "FutureSport"},
		{Filename: "activity.gpx", Size: 1, DataType: UploadGPX, ExternalID: "\n"},
	}
	for index, input := range invalidUploads {
		if _, err := client.UploadActivity(context.Background(), input, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
			t.Fatalf("upload %d error=%v", index, err)
		}
	}
	if _, err := client.UploadActivity(context.Background(), valid, nil); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("nil reader error=%v", err)
	}
	if _, err := client.UploadActivity(context.Background(), ActivityUploadRequest{Filename: "activity.gpx", Size: 2, DataType: UploadGPX}, strings.NewReader("x")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("short reader error=%v", err)
	}
	if _, err := client.UploadActivity(context.Background(), valid, strings.NewReader("xx")); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("long reader error=%v", err)
	}
	if _, err := client.UploadActivity(context.Background(), valid, failingReader{}); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("reader failure error=%v", err)
	}
	if _, err := client.GetUpload(context.Background(), "bad"); !errors.Is(err, socialhub.ErrInvalidArgument) {
		t.Fatalf("upload ID error=%v", err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("source failed") }
