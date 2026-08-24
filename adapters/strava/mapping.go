package strava

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func typedActivity(input activityWire) *Activity {
	uploadID := firstNonEmpty(input.UploadIDString, string(input.UploadID))
	return &Activity{
		ID: string(input.ID), AthleteID: string(input.Athlete.ID), UploadID: uploadID, ExternalID: input.ExternalID,
		Name: input.Name, Description: input.Description, SportType: input.SportType, Distance: input.Distance,
		MovingTimeSeconds: input.MovingTime, ElapsedTimeSeconds: input.ElapsedTime, ElevationGain: input.TotalElevationGain,
		KudosCount: input.KudosCount, CommentCount: input.CommentCount, StartDate: input.StartDate,
		StartDateLocal: input.StartDateLocal, Timezone: input.Timezone, Private: input.Private, Trainer: input.Trainer,
		Commute: input.Commute, Manual: input.Manual, HideFromHome: input.HideFromHome, DeviceName: input.DeviceName,
		GearID: input.GearID, Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func mapUser(accountID socialhub.AccountID, input athleteWire) *socialhub.User {
	displayName := strings.TrimSpace(strings.Join([]string{input.FirstName, input.LastName}, " "))
	accountType := "athlete"
	extension := append(json.RawMessage(nil), input.Raw...)
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	return &socialhub.User{
		Platform: "strava", AccountID: accountID, ID: string(input.ID), Username: stringPointer(input.Username),
		DisplayName: stringPointer(displayName), AvatarURL: stringPointer(firstNonEmpty(input.Profile, input.ProfileMedium)),
		ProfileURL: stringPointer("https://www.strava.com/athletes/" + string(input.ID)), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"strava.athlete": extension},
	}
}

func mapPost(accountID socialhub.AccountID, input *Activity, observedAt time.Time) *socialhub.Post {
	text := input.Description
	if strings.TrimSpace(text) == "" {
		text = input.Name
	}
	visibility := ""
	if input.Private != nil {
		visibility = "followers_or_everyone"
		if *input.Private {
			visibility = "only_you"
		}
	}
	extension := append(json.RawMessage(nil), input.Raw...)
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	return &socialhub.Post{
		Platform: "strava", AccountID: accountID, ID: input.ID, AuthorID: stringPointer(input.AthleteID),
		Text: stringPointer(text), CreatedAt: input.StartDate, URL: stringPointer("https://www.strava.com/activities/" + input.ID),
		Visibility: stringPointer(visibility), Metrics: activityMetrics(input, observedAt),
		Status:     &socialhub.PublishStatus{ID: input.ID, State: socialhub.PublishStatePublished, UpdatedAt: &observedAt},
		Extensions: map[string]json.RawMessage{"strava.activity": extension},
	}
}

func activityMetrics(input *Activity, observedAt time.Time) []socialhub.Metric {
	metrics := make([]socialhub.Metric, 0, 5)
	appendMetric := func(name string, value float64, definition string) {
		metrics = append(metrics, socialhub.Metric{Name: name, Value: value, AsOf: observedAt, Definition: definition})
	}
	if input.KudosCount != nil {
		appendMetric("kudos", float64(*input.KudosCount), "Strava kudos count")
	}
	if input.CommentCount != nil {
		appendMetric("comments", float64(*input.CommentCount), "Strava activity comment count")
	}
	if input.Distance != nil {
		appendMetric("distance_meters", *input.Distance, "Activity distance in meters")
	}
	if input.MovingTimeSeconds != nil {
		appendMetric("moving_time_seconds", float64(*input.MovingTimeSeconds), "Activity moving time in seconds")
	}
	if input.ElevationGain != nil {
		appendMetric("elevation_gain_meters", *input.ElevationGain, "Activity total elevation gain in meters")
	}
	return metrics
}

func mapComment(accountID socialhub.AccountID, input commentWire) socialhub.Comment {
	extension := append(json.RawMessage(nil), input.Raw...)
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	return socialhub.Comment{
		Platform: "strava", AccountID: accountID, ID: string(input.ID), PostID: string(input.ActivityID),
		AuthorID: stringPointer(string(input.Athlete.ID)), Text: input.Text, CreatedAt: input.CreatedAt,
		Extensions: map[string]json.RawMessage{"strava.comment": extension},
	}
}

func mapUpload(input uploadWire) *Upload {
	id := firstNonEmpty(input.IDString, string(input.ID))
	var activityID *string
	if input.ActivityID != "" {
		value := string(input.ActivityID)
		activityID = &value
	}
	return &Upload{
		ID: id, ExternalID: input.ExternalID, Status: input.Status, Error: input.Error, ActivityID: activityID,
		Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}
