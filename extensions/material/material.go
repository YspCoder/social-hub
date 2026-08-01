// Package material defines optional temporary and permanent asset management.
package material

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

// Kind describes platform-managed asset retention.
type Kind string

const (
	Temporary Kind = "temporary"
	Permanent Kind = "permanent"
)

// Metadata describes an asset upload.
type Metadata struct {
	Filename string
	MIME     string
	Title    string
	Caption  string
}

// Asset is a platform-managed media object.
type Asset struct {
	ID        string
	Kind      Kind
	Type      socialhub.MediaType
	CreatedAt *time.Time
	ExpiresAt *time.Time
	URL       *string
}

// ListRequest selects managed assets.
type ListRequest struct {
	Kind   Kind
	Type   socialhub.MediaType
	Cursor string
	Limit  int
}

// Manager manages temporary and permanent platform assets.
type Manager interface {
	Upload(context.Context, Kind, socialhub.MediaType, io.Reader, Metadata) (*Asset, error)
	Get(context.Context, string) (*Asset, error)
	List(context.Context, ListRequest) (socialhub.Page[Asset], error)
	Delete(context.Context, string) error
}

// Provider exposes optional platform material management.
type Provider interface {
	MaterialManager() Manager
}
