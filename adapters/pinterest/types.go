package pinterest

import (
	"encoding/json"
	"time"
)

type pinterestAccount struct {
	ID             string `json:"id"`
	Username       string `json:"username"`
	BusinessName   string `json:"business_name"`
	About          string `json:"about"`
	AccountType    string `json:"account_type"`
	ProfileImage   string `json:"profile_image"`
	WebsiteURL     string `json:"website_url"`
	FollowerCount  *int64 `json:"follower_count"`
	FollowingCount *int64 `json:"following_count"`
	MonthlyViews   *int64 `json:"monthly_views"`
	PinCount       *int64 `json:"pin_count"`
	BoardCount     *int64 `json:"board_count"`
}

type pinterestPin struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	AltText         string     `json:"alt_text"`
	Link            string     `json:"link"`
	BoardID         string     `json:"board_id"`
	BoardSectionID  string     `json:"board_section_id"`
	ParentPinID     string     `json:"parent_pin_id"`
	CreatedAt       *time.Time `json:"created_at"`
	CreativeType    string     `json:"creative_type"`
	DominantColor   string     `json:"dominant_color"`
	HasBeenPromoted bool       `json:"has_been_promoted"`
	IsOwner         bool       `json:"is_owner"`
	IsProduct       bool       `json:"is_product"`
	IsStandard      bool       `json:"is_standard"`
	BoardOwner      struct {
		Username string `json:"username"`
	} `json:"board_owner"`
	Media      pinterestPinMedia             `json:"media"`
	PinMetrics map[string]map[string]float64 `json:"pin_metrics"`
}

type pinterestPinMedia struct {
	MediaType     string                    `json:"media_type"`
	Images        map[string]pinterestAsset `json:"images"`
	Items         []json.RawMessage         `json:"items"`
	CoverImageURL string                    `json:"cover_image_url"`
	VideoURL      string                    `json:"video_url"`
	VideoURLHLS   string                    `json:"video_url_hls"`
	Width         *int                      `json:"width"`
	Height        *int                      `json:"height"`
	DurationMS    *float64                  `json:"duration"`
}

type pinterestAsset struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type pinterestMediaItem struct {
	ItemType   string                    `json:"item_type"`
	URL        string                    `json:"url"`
	VideoURL   string                    `json:"video_url"`
	Images     map[string]pinterestAsset `json:"images"`
	Width      int                       `json:"width"`
	Height     int                       `json:"height"`
	DurationMS float64                   `json:"duration"`
}

type pinList struct {
	Items    []pinterestPin `json:"items"`
	Bookmark *string        `json:"bookmark"`
}

type mediaRegistration struct {
	MediaID          string            `json:"media_id"`
	MediaType        string            `json:"media_type"`
	UploadURL        string            `json:"upload_url"`
	UploadParameters map[string]string `json:"upload_parameters"`
}

type mediaStatusResponse struct {
	MediaID   string `json:"media_id"`
	MediaType string `json:"media_type"`
	Status    string `json:"status"`
}

type videoUpload struct {
	URL        string
	Parameters map[string]string
	Uploading  bool
	Used       bool
}
