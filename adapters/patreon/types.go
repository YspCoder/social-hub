package patreon

import (
	"context"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

// Campaign is the stable creator-campaign subset exposed by Patreon API v2.
type Campaign struct {
	ID           string          `json:"id"`
	CreatorID    string          `json:"creator_id,omitempty"`
	Name         *string         `json:"name,omitempty"`
	CreationName *string         `json:"creation_name,omitempty"`
	Summary      *string         `json:"summary,omitempty"`
	URL          *string         `json:"url,omitempty"`
	ImageURL     *string         `json:"image_url,omitempty"`
	Vanity       *string         `json:"vanity,omitempty"`
	Currency     *string         `json:"currency,omitempty"`
	PatronCount  *int64          `json:"patron_count,omitempty"`
	Monthly      *bool           `json:"is_monthly,omitempty"`
	NSFW         *bool           `json:"is_nsfw,omitempty"`
	CreatedAt    *time.Time      `json:"created_at,omitempty"`
	PublishedAt  *time.Time      `json:"published_at,omitempty"`
	Raw          json.RawMessage `json:"raw,omitempty"`
}

// Member preserves campaign membership and entitlement fields that have no
// equivalent in the common social models.
type Member struct {
	ID                      string          `json:"id"`
	CampaignID              string          `json:"campaign_id,omitempty"`
	UserID                  string          `json:"user_id,omitempty"`
	FullName                *string         `json:"full_name,omitempty"`
	Email                   *string         `json:"email,omitempty"`
	PatronStatus            *string         `json:"patron_status,omitempty"`
	LastChargeStatus        *string         `json:"last_charge_status,omitempty"`
	LastChargeDate          *time.Time      `json:"last_charge_date,omitempty"`
	PledgeRelationshipStart *time.Time      `json:"pledge_relationship_start,omitempty"`
	LifetimeSupportCents    *int64          `json:"campaign_lifetime_support_cents,omitempty"`
	EntitledAmountCents     *int64          `json:"currently_entitled_amount_cents,omitempty"`
	WillPayAmountCents      *int64          `json:"will_pay_amount_cents,omitempty"`
	EntitledTierIDs         []string        `json:"currently_entitled_tier_ids,omitempty"`
	Raw                     json.RawMessage `json:"raw,omitempty"`
}

// CampaignWorkflow reads campaigns owned by the authorized creator.
type CampaignWorkflow interface {
	GetCampaign(context.Context, ...socialhub.CallOption) (*Campaign, error)
	ListCampaigns(context.Context, int, string, ...socialhub.CallOption) (socialhub.Page[Campaign], error)
}

// MemberWorkflow reads one campaign's members and entitlement state.
type MemberWorkflow interface {
	GetMember(context.Context, string, ...socialhub.CallOption) (*Member, error)
	ListMembers(context.Context, int, string, ...socialhub.CallOption) (socialhub.Page[Member], error)
}

type resourceIdentifier struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

type toOneRelationship struct {
	Data *resourceIdentifier `json:"data"`
}

type toManyRelationship struct {
	Data []resourceIdentifier `json:"data"`
}

type userResource struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		FullName  *string `json:"full_name"`
		FirstName *string `json:"first_name"`
		LastName  *string `json:"last_name"`
		Vanity    *string `json:"vanity"`
		ImageURL  *string `json:"image_url"`
		ThumbURL  *string `json:"thumb_url"`
		URL       *string `json:"url"`
	} `json:"attributes"`
	Raw json.RawMessage `json:"-"`
}

func (resource *userResource) UnmarshalJSON(data []byte) error {
	type alias userResource
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = userResource(decoded)
	resource.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type postResource struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		AppStatus   *string         `json:"app_status"`
		Content     *string         `json:"content"`
		EmbedData   json.RawMessage `json:"embed_data"`
		EmbedURL    *string         `json:"embed_url"`
		IsPaid      *bool           `json:"is_paid"`
		IsPublic    *bool           `json:"is_public"`
		PublishedAt *time.Time      `json:"published_at"`
		Title       *string         `json:"title"`
		URL         *string         `json:"url"`
	} `json:"attributes"`
	Relationships struct {
		Campaign toOneRelationship `json:"campaign"`
		User     toOneRelationship `json:"user"`
	} `json:"relationships"`
	Raw json.RawMessage `json:"-"`
}

func (resource *postResource) UnmarshalJSON(data []byte) error {
	type alias postResource
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = postResource(decoded)
	resource.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type campaignResource struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name          *string    `json:"name"`
		CreationName  *string    `json:"creation_name"`
		Summary       *string    `json:"summary"`
		URL           *string    `json:"url"`
		ImageURL      *string    `json:"image_url"`
		ImageSmallURL *string    `json:"image_small_url"`
		Vanity        *string    `json:"vanity"`
		Currency      *string    `json:"currency"`
		PatronCount   *int64     `json:"patron_count"`
		Monthly       *bool      `json:"is_monthly"`
		NSFW          *bool      `json:"is_nsfw"`
		CreatedAt     *time.Time `json:"created_at"`
		PublishedAt   *time.Time `json:"published_at"`
	} `json:"attributes"`
	Relationships struct {
		Creator toOneRelationship `json:"creator"`
	} `json:"relationships"`
	Raw json.RawMessage `json:"-"`
}

func (resource *campaignResource) UnmarshalJSON(data []byte) error {
	type alias campaignResource
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = campaignResource(decoded)
	resource.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type memberResource struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		FullName                     *string    `json:"full_name"`
		Email                        *string    `json:"email"`
		PatronStatus                 *string    `json:"patron_status"`
		LastChargeStatus             *string    `json:"last_charge_status"`
		LastChargeDate               *time.Time `json:"last_charge_date"`
		PledgeRelationshipStart      *time.Time `json:"pledge_relationship_start"`
		CampaignLifetimeSupportCents *int64     `json:"campaign_lifetime_support_cents"`
		CurrentlyEntitledAmountCents *int64     `json:"currently_entitled_amount_cents"`
		WillPayAmountCents           *int64     `json:"will_pay_amount_cents"`
	} `json:"attributes"`
	Relationships struct {
		Campaign               toOneRelationship  `json:"campaign"`
		User                   toOneRelationship  `json:"user"`
		CurrentlyEntitledTiers toManyRelationship `json:"currently_entitled_tiers"`
	} `json:"relationships"`
	Raw json.RawMessage `json:"-"`
}

func (resource *memberResource) UnmarshalJSON(data []byte) error {
	type alias memberResource
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*resource = memberResource(decoded)
	resource.Raw = append(json.RawMessage(nil), data...)
	return nil
}

type paginationMeta struct {
	Pagination struct {
		Total   int64 `json:"total"`
		Cursors struct {
			Next string `json:"next"`
		} `json:"cursors"`
	} `json:"pagination"`
}

type userResponse struct {
	Data userResource `json:"data"`
}

type postResponse struct {
	Data postResource `json:"data"`
}

type postListResponse struct {
	Data []postResource `json:"data"`
	Meta paginationMeta `json:"meta"`
}

type campaignResponse struct {
	Data campaignResource `json:"data"`
}

type campaignListResponse struct {
	Data []campaignResource `json:"data"`
	Meta paginationMeta     `json:"meta"`
}

type memberResponse struct {
	Data memberResource `json:"data"`
}

type memberListResponse struct {
	Data []memberResource `json:"data"`
	Meta paginationMeta   `json:"meta"`
}

var _ CampaignWorkflow = (*Client)(nil)
var _ MemberWorkflow = (*Client)(nil)
