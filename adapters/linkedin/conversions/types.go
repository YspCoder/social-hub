package conversions

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	MaximumBatchSize    = 5000
	RequestsPerMinute   = 600
	RequestsPerDay      = 500000
	MaximumEventAgeDays = 90
)

// Decimal is an exact non-negative base-10 amount. LinkedIn's wire contract
// encodes it as a JSON string.
type Decimal string

func (value Decimal) String() string { return string(value) }

type Money struct {
	CurrencyCode string
	Amount       Decimal
}

type UserInfo struct {
	FirstName   string
	LastName    string
	CompanyName string
	Title       string
	CountryCode string
}

// User contains the matching identifiers documented by LinkedIn. Emails can
// be plaintext or exact lowercase SHA-256; plaintext is normalized and hashed
// only in a temporary wire payload.
type User struct {
	Emails                          []string
	LinkedInFirstPartyTrackingUUIDs []string
	AcxiomIDs                       []string
	PlaintextIPAddresses            []string
	SHA256IPAddresses               []string
	GoogleAdvertisingIDs            []string
	Info                            *UserInfo
	LeadURN                         string
	ExternalIDs                     []string
}

type ConversionEvent struct {
	ConversionHappenedAt int64
	ConversionValue      *Money
	User                 User
	EventID              string
}

type SubmitEventsRequest struct {
	Events []ConversionEvent
}

type SubmitResult struct {
	StatusCode     int
	EventsAccepted int
	Batch          bool
}

type EventWorkflow interface {
	SubmitEvents(context.Context, SubmitEventsRequest, ...socialhub.CallOption) (SubmitResult, error)
}

var _ EventWorkflow = (*Client)(nil)
