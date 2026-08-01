package linkedin

import (
	"encoding/json"
	"fmt"
)

type stringID string

func (id *stringID) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err == nil {
		*id = stringID(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return fmt.Errorf("linkedin: decode ID: %w", err)
	}
	*id = stringID(number.String())
	return nil
}

type userInfo struct {
	Sub           string          `json:"sub"`
	Name          string          `json:"name"`
	GivenName     string          `json:"given_name"`
	FamilyName    string          `json:"family_name"`
	Picture       string          `json:"picture"`
	Email         string          `json:"email"`
	EmailVerified bool            `json:"email_verified"`
	Locale        json.RawMessage `json:"locale"`
}

type linkedInPost struct {
	ID          string `json:"id"`
	Author      string `json:"author"`
	Commentary  string `json:"commentary"`
	Visibility  string `json:"visibility"`
	Lifecycle   string `json:"lifecycleState"`
	CreatedAt   int64  `json:"createdAt"`
	PublishedAt int64  `json:"publishedAt"`
	Content     struct {
		Media *struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"media"`
		MultiImage *struct {
			Images []struct {
				ID      string `json:"id"`
				AltText string `json:"altText"`
			} `json:"images"`
		} `json:"multiImage"`
		Article *struct {
			Source      string `json:"source"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Thumbnail   string `json:"thumbnail"`
		} `json:"article"`
	} `json:"content"`
	ReshareContext *struct {
		Parent string `json:"parent"`
		Root   string `json:"root"`
	} `json:"reshareContext"`
}

type linkedInComment struct {
	ID            stringID `json:"id"`
	CommentURN    string   `json:"commentUrn"`
	Actor         string   `json:"actor"`
	Object        string   `json:"object"`
	ParentComment string   `json:"parentComment"`
	Message       struct {
		Text string `json:"text"`
	} `json:"message"`
	Created struct {
		Actor string `json:"actor"`
		Time  int64  `json:"time"`
	} `json:"created"`
}

type paging struct {
	Start int `json:"start"`
	Count int `json:"count"`
	Total int `json:"total"`
	Links []struct {
		Rel  string `json:"rel"`
		Href string `json:"href"`
	} `json:"links"`
}

type postPage struct {
	Elements []linkedInPost `json:"elements"`
	Paging   paging         `json:"paging"`
}

type commentPage struct {
	Elements []linkedInComment `json:"elements"`
	Paging   paging            `json:"paging"`
}

type imageInitializeResponse struct {
	Value struct {
		UploadURL          string `json:"uploadUrl"`
		UploadURLExpiresAt int64  `json:"uploadUrlExpiresAt"`
		Image              string `json:"image"`
	} `json:"value"`
}

type linkedInImage struct {
	ID              string  `json:"id"`
	Status          string  `json:"status"`
	DownloadURL     string  `json:"downloadUrl"`
	DownloadURLTTL  int64   `json:"downloadUrlExpiresAt"`
	AspectRatioWide float64 `json:"aspectRatioWidth"`
	AspectRatioHigh float64 `json:"aspectRatioHeight"`
}
