package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

// SportType is Strava's current activity sport-type enumeration.
type SportType string

const (
	SportAlpineSki                     SportType = "AlpineSki"
	SportBackcountrySki                SportType = "BackcountrySki"
	SportBadminton                     SportType = "Badminton"
	SportBasketball                    SportType = "Basketball"
	SportCanoeing                      SportType = "Canoeing"
	SportCricket                       SportType = "Cricket"
	SportCrossfit                      SportType = "Crossfit"
	SportDance                         SportType = "Dance"
	SportEBikeRide                     SportType = "EBikeRide"
	SportElliptical                    SportType = "Elliptical"
	SportEMountainBikeRide             SportType = "EMountainBikeRide"
	SportGolf                          SportType = "Golf"
	SportGravelRide                    SportType = "GravelRide"
	SportHandcycle                     SportType = "Handcycle"
	SportHighIntensityIntervalTraining SportType = "HighIntensityIntervalTraining"
	SportHike                          SportType = "Hike"
	SportIceSkate                      SportType = "IceSkate"
	SportInlineSkate                   SportType = "InlineSkate"
	SportKayaking                      SportType = "Kayaking"
	SportKitesurf                      SportType = "Kitesurf"
	SportMountainBikeRide              SportType = "MountainBikeRide"
	SportNordicSki                     SportType = "NordicSki"
	SportPadel                         SportType = "Padel"
	SportPhysicalTherapy               SportType = "PhysicalTherapy"
	SportPickleball                    SportType = "Pickleball"
	SportPilates                       SportType = "Pilates"
	SportRacquetball                   SportType = "Racquetball"
	SportRide                          SportType = "Ride"
	SportRockClimbing                  SportType = "RockClimbing"
	SportRollerSki                     SportType = "RollerSki"
	SportRowing                        SportType = "Rowing"
	SportRun                           SportType = "Run"
	SportSail                          SportType = "Sail"
	SportSkateboard                    SportType = "Skateboard"
	SportSnowboard                     SportType = "Snowboard"
	SportSnowshoe                      SportType = "Snowshoe"
	SportSoccer                        SportType = "Soccer"
	SportSquash                        SportType = "Squash"
	SportStairStepper                  SportType = "StairStepper"
	SportStandUpPaddling               SportType = "StandUpPaddling"
	SportSurfing                       SportType = "Surfing"
	SportSwim                          SportType = "Swim"
	SportTableTennis                   SportType = "TableTennis"
	SportTennis                        SportType = "Tennis"
	SportTrailRun                      SportType = "TrailRun"
	SportVelomobile                    SportType = "Velomobile"
	SportVirtualRide                   SportType = "VirtualRide"
	SportVirtualRow                    SportType = "VirtualRow"
	SportVirtualRun                    SportType = "VirtualRun"
	SportVolleyball                    SportType = "Volleyball"
	SportWalk                          SportType = "Walk"
	SportWeightTraining                SportType = "WeightTraining"
	SportWheelchair                    SportType = "Wheelchair"
	SportWindsurf                      SportType = "Windsurf"
	SportWorkout                       SportType = "Workout"
	SportYoga                          SportType = "Yoga"
)

var sportTypes = map[SportType]struct{}{
	SportAlpineSki: {}, SportBackcountrySki: {}, SportBadminton: {}, SportBasketball: {}, SportCanoeing: {}, SportCricket: {},
	SportCrossfit: {}, SportDance: {}, SportEBikeRide: {}, SportElliptical: {}, SportEMountainBikeRide: {}, SportGolf: {},
	SportGravelRide: {}, SportHandcycle: {}, SportHighIntensityIntervalTraining: {}, SportHike: {}, SportIceSkate: {},
	SportInlineSkate: {}, SportKayaking: {}, SportKitesurf: {}, SportMountainBikeRide: {}, SportNordicSki: {}, SportPadel: {},
	SportPhysicalTherapy: {}, SportPickleball: {}, SportPilates: {}, SportRacquetball: {}, SportRide: {}, SportRockClimbing: {},
	SportRollerSki: {}, SportRowing: {}, SportRun: {}, SportSail: {}, SportSkateboard: {}, SportSnowboard: {}, SportSnowshoe: {},
	SportSoccer: {}, SportSquash: {}, SportStairStepper: {}, SportStandUpPaddling: {}, SportSurfing: {}, SportSwim: {},
	SportTableTennis: {}, SportTennis: {}, SportTrailRun: {}, SportVelomobile: {}, SportVirtualRide: {}, SportVirtualRow: {},
	SportVirtualRun: {}, SportVolleyball: {}, SportWalk: {}, SportWeightTraining: {}, SportWheelchair: {}, SportWindsurf: {},
	SportWorkout: {}, SportYoga: {},
}

// Activity preserves the stable activity fields used by common and typed
// workflows. Raw contains the complete API representation.
type Activity struct {
	ID                 string          `json:"id"`
	AthleteID          string          `json:"athlete_id"`
	UploadID           string          `json:"upload_id,omitempty"`
	ExternalID         string          `json:"external_id,omitempty"`
	Name               string          `json:"name"`
	Description        string          `json:"description,omitempty"`
	SportType          SportType       `json:"sport_type"`
	Distance           *float64        `json:"distance,omitempty"`
	MovingTimeSeconds  *int64          `json:"moving_time_seconds,omitempty"`
	ElapsedTimeSeconds *int64          `json:"elapsed_time_seconds,omitempty"`
	ElevationGain      *float64        `json:"elevation_gain,omitempty"`
	KudosCount         *int64          `json:"kudos_count,omitempty"`
	CommentCount       *int64          `json:"comment_count,omitempty"`
	StartDate          *time.Time      `json:"start_date,omitempty"`
	StartDateLocal     *time.Time      `json:"start_date_local,omitempty"`
	Timezone           string          `json:"timezone,omitempty"`
	Private            *bool           `json:"private,omitempty"`
	Trainer            *bool           `json:"trainer,omitempty"`
	Commute            *bool           `json:"commute,omitempty"`
	Manual             *bool           `json:"manual,omitempty"`
	HideFromHome       *bool           `json:"hide_from_home,omitempty"`
	DeviceName         string          `json:"device_name,omitempty"`
	GearID             string          `json:"gear_id,omitempty"`
	Raw                json.RawMessage `json:"raw,omitempty"`
}

// ManualActivityRequest contains the required fields for a Strava manual activity.
type ManualActivityRequest struct {
	Name           string
	SportType      SportType
	StartDateLocal time.Time
	ElapsedTime    time.Duration
	Description    string
	DistanceMeters *float64
	Trainer        *bool
	Commute        *bool
}

// ActivityUpdateRequest contains the mutable Strava activity fields.
type ActivityUpdateRequest struct {
	Name         *string
	SportType    *SportType
	Description  *string
	Trainer      *bool
	Commute      *bool
	HideFromHome *bool
	GearID       *string
}

// UploadDataType identifies one supported Strava activity-file format.
type UploadDataType string

const (
	UploadFIT   UploadDataType = "fit"
	UploadFITGZ UploadDataType = "fit.gz"
	UploadTCX   UploadDataType = "tcx"
	UploadTCXGZ UploadDataType = "tcx.gz"
	UploadGPX   UploadDataType = "gpx"
	UploadGPXGZ UploadDataType = "gpx.gz"
	UploadJSON  UploadDataType = "json"
)

// ActivityUploadRequest describes a streamed activity file and optional metadata.
type ActivityUploadRequest struct {
	Filename    string
	Size        int64
	DataType    UploadDataType
	SportType   SportType
	Name        string
	Description string
	ExternalID  string
	Trainer     *bool
	Commute     *bool
}

// Upload describes Strava's asynchronous activity-file processing state.
type Upload struct {
	ID         string          `json:"id"`
	ExternalID string          `json:"external_id,omitempty"`
	Status     string          `json:"status,omitempty"`
	Error      *string         `json:"error,omitempty"`
	ActivityID *string         `json:"activity_id,omitempty"`
	Raw        json.RawMessage `json:"raw,omitempty"`
}

// WebhookEvent is the account-scoped Strava webhook payload.
type WebhookEvent struct {
	AspectType     string                     `json:"aspect_type"`
	EventTime      int64                      `json:"event_time"`
	ObjectID       int64                      `json:"object_id"`
	ObjectType     string                     `json:"object_type"`
	OwnerID        int64                      `json:"owner_id"`
	SubscriptionID int64                      `json:"subscription_id"`
	Updates        map[string]json.RawMessage `json:"updates,omitempty"`
	Raw            json.RawMessage            `json:"raw,omitempty"`
}

// ActivityWorkflow creates manual activities and updates owned activities.
type ActivityWorkflow interface {
	CreateManualActivity(context.Context, ManualActivityRequest, ...socialhub.CallOption) (*Activity, error)
	UpdateActivity(context.Context, string, ActivityUpdateRequest, ...socialhub.CallOption) (*Activity, error)
}

// ActivityUploadWorkflow streams activity files and polls their processing state.
type ActivityUploadWorkflow interface {
	UploadActivity(context.Context, ActivityUploadRequest, io.Reader, ...socialhub.CallOption) (*Upload, error)
	GetUpload(context.Context, string, ...socialhub.CallOption) (*Upload, error)
}

type wireID string

func (id *wireID) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*id = ""
		return nil
	}
	value := string(data)
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	}
	if value == "" || len(value) > 32 {
		return fmt.Errorf("invalid Strava ID")
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return fmt.Errorf("invalid Strava ID")
		}
	}
	if strings.TrimLeft(value, "0") == "" {
		return fmt.Errorf("invalid Strava ID")
	}
	*id = wireID(value)
	return nil
}

type athleteWire struct {
	ID            wireID          `json:"id"`
	Username      string          `json:"username"`
	FirstName     string          `json:"firstname"`
	LastName      string          `json:"lastname"`
	Profile       string          `json:"profile"`
	ProfileMedium string          `json:"profile_medium"`
	City          string          `json:"city"`
	State         string          `json:"state"`
	Country       string          `json:"country"`
	Sex           string          `json:"sex"`
	FollowerCount *int64          `json:"follower_count"`
	FriendCount   *int64          `json:"friend_count"`
	Weight        *float64        `json:"weight"`
	Raw           json.RawMessage `json:"-"`
}

func (athlete *athleteWire) UnmarshalJSON(data []byte) error {
	type alias athleteWire
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*athlete = athleteWire(decoded)
	athlete.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type activityWire struct {
	ID                 wireID          `json:"id"`
	Athlete            athleteWire     `json:"athlete"`
	UploadID           wireID          `json:"upload_id"`
	UploadIDString     string          `json:"upload_id_str"`
	ExternalID         string          `json:"external_id"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	SportType          SportType       `json:"sport_type"`
	Distance           *float64        `json:"distance"`
	MovingTime         *int64          `json:"moving_time"`
	ElapsedTime        *int64          `json:"elapsed_time"`
	TotalElevationGain *float64        `json:"total_elevation_gain"`
	KudosCount         *int64          `json:"kudos_count"`
	CommentCount       *int64          `json:"comment_count"`
	StartDate          *time.Time      `json:"start_date"`
	StartDateLocal     *time.Time      `json:"start_date_local"`
	Timezone           string          `json:"timezone"`
	Private            *bool           `json:"private"`
	Trainer            *bool           `json:"trainer"`
	Commute            *bool           `json:"commute"`
	Manual             *bool           `json:"manual"`
	HideFromHome       *bool           `json:"hide_from_home"`
	DeviceName         string          `json:"device_name"`
	GearID             string          `json:"gear_id"`
	Raw                json.RawMessage `json:"-"`
}

func (activity *activityWire) UnmarshalJSON(data []byte) error {
	type alias activityWire
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*activity = activityWire(decoded)
	activity.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type commentWire struct {
	ID         wireID          `json:"id"`
	ActivityID wireID          `json:"activity_id"`
	Text       string          `json:"text"`
	Athlete    athleteWire     `json:"athlete"`
	CreatedAt  *time.Time      `json:"created_at"`
	Cursor     string          `json:"cursor"`
	Raw        json.RawMessage `json:"-"`
}

func (comment *commentWire) UnmarshalJSON(data []byte) error {
	type alias commentWire
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*comment = commentWire(decoded)
	comment.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type uploadWire struct {
	ID         wireID          `json:"id"`
	IDString   string          `json:"id_str"`
	ExternalID string          `json:"external_id"`
	Error      *string         `json:"error"`
	Status     string          `json:"status"`
	ActivityID wireID          `json:"activity_id"`
	Raw        json.RawMessage `json:"-"`
}

func (upload *uploadWire) UnmarshalJSON(data []byte) error {
	type alias uploadWire
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*upload = uploadWire(decoded)
	upload.Raw = append(json.RawMessage(nil), data...)
	return nil
}

func validSportType(value SportType) bool {
	_, found := sportTypes[value]
	return found
}

var _ ActivityWorkflow = (*Client)(nil)
var _ ActivityUploadWorkflow = (*Client)(nil)
