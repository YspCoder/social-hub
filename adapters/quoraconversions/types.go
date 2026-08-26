package quoraconversions

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"social-hub/pkg/socialhub"
)

const (
	MaximumBatchSize         = 1000
	RateLimitEventsPerMinute = 1000
)

// EventName is one of the event names in Quora's public OpenAPI contract.
type EventName string

const (
	EventGeneric              EventName = "Generic"
	EventAppInstall           EventName = "AppInstall"
	EventPurchase             EventName = "Purchase"
	EventGenerateLead         EventName = "GenerateLead"
	EventCompleteRegistration EventName = "CompleteRegistration"
	EventAddPaymentInfo       EventName = "AddPaymentInfo"
	EventAddToCart            EventName = "AddToCart"
	EventAddToWishlist        EventName = "AddToWishlist"
	EventInitiateCheckout     EventName = "InitiateCheckout"
	EventSearch               EventName = "Search"
)

// Decimal preserves an exact JSON number. Construct it from a base-10 string,
// for example "5.99".
type Decimal string

func (value Decimal) String() string { return string(value) }

func (value Decimal) MarshalJSON() ([]byte, error) {
	if !validDecimal(value) {
		return nil, fmt.Errorf("quoraconversions: invalid decimal")
	}
	return json.Marshal(json.Number(value))
}

// Microseconds converts a timestamp to the unit required by Quora.
func Microseconds(value time.Time) int64 { return value.UnixMicro() }

// User contains optional match and location data. Email may be plaintext or a
// precomputed SHA-256 hex digest; use HashEmail to avoid sending plaintext.
type User struct {
	Email       string `json:"email,omitempty"`
	PhoneNumber string `json:"phone_number,omitempty"`
	Name        string `json:"name,omitempty"`
	IP          string `json:"ip,omitempty"`
	Country     string `json:"country,omitempty"`
	Region      string `json:"region,omitempty"`
	City        string `json:"city,omitempty"`
	PostalCode  string `json:"postal_code,omitempty"`
	CompanyName string `json:"company_name,omitempty"`
	JobTitle    string `json:"job_title,omitempty"`
	DateOfBirth string `json:"date_of_birth,omitempty"`
}

// Device contains the optional device context in the public contract.
type Device struct {
	MobileDeviceID string `json:"mobile_device_id,omitempty"`
	Referer        string `json:"referer,omitempty"`
	UserAgent      string `json:"user_agent,omitempty"`
	Language       string `json:"language,omitempty"`
}

// Conversion contains the conversion-specific attributes. Timestamp is Unix
// microseconds. ClickID is the latest qclid value; it is optional for request
// acceptance but required for attribution. EventID enables Pixel deduplication.
type Conversion struct {
	EventName EventName `json:"event_name"`
	Timestamp *int64    `json:"timestamp,omitempty"`
	Value     Decimal   `json:"value,omitempty"`
	EventID   string    `json:"event_id,omitempty"`
	ClickID   string    `json:"click_id,omitempty"`
}

// ConversionEvent is one independently processed event.
type ConversionEvent struct {
	User       User       `json:"user"`
	Device     Device     `json:"device"`
	Conversion Conversion `json:"conversion"`
}

type SubmitEventRequest struct {
	Event ConversionEvent
	Debug bool
}

type SubmitEventsRequest struct {
	Events []ConversionEvent
	Debug  bool
}

type WarningCode string

const (
	WarningClickIDMissing       WarningCode = "CLICK_ID_MISSING"
	WarningClickIDInvalidFormat WarningCode = "CLICK_ID_INVALID_FORMAT"
	WarningEventIDMissing       WarningCode = "EVENT_ID_MISSING"
)

type Warning struct {
	Code WarningCode
}

type EventStatus string

const (
	EventStatusOK    EventStatus = "OK"
	EventStatusError EventStatus = "ERROR"
)

type SubmitEventResult struct {
	StatusCode int
	Status     EventStatus
	Warnings   []Warning
}

type EventResult struct {
	Status          EventStatus
	Index           int
	ErrorCode       string
	HasErrorMessage bool
	Warnings        []Warning
}

type SubmitEventsResult struct {
	StatusCode     int
	EventsReceived int
	EventsErrored  int
	Events         []EventResult
}

type ConversionWorkflow interface {
	SubmitEvent(context.Context, SubmitEventRequest, ...socialhub.CallOption) (SubmitEventResult, error)
	SubmitEvents(context.Context, SubmitEventsRequest, ...socialhub.CallOption) (SubmitEventsResult, error)
}

var _ ConversionWorkflow = (*Client)(nil)
