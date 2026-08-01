package instagram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

// ContainerType is an Instagram publication container media type.
type ContainerType string

const (
	ContainerImage    ContainerType = "IMAGE"
	ContainerReel     ContainerType = "REELS"
	ContainerStory    ContainerType = "STORIES"
	ContainerCarousel ContainerType = "CAROUSEL"
)

// ContainerRequest creates one remote-media publication container.
type ContainerRequest struct {
	Type              ContainerType
	MediaType         socialhub.MediaType
	MediaURL          string
	ThumbnailOffsetMS int
	Caption           string
	Children          []string
	CarouselItem      bool
	ShareToFeed       *bool
	IsAIGenerated     bool
}

// ContainerStatus reports asynchronous media ingestion state.
type ContainerStatus struct {
	ID         string `json:"id"`
	StatusCode string `json:"status_code"`
	Status     string `json:"status"`
}

// ContainerWorkflow exposes Instagram's create, poll, and publish lifecycle.
type ContainerWorkflow interface {
	Create(context.Context, ContainerRequest, ...socialhub.CallOption) (*ContainerStatus, error)
	Status(context.Context, string, ...socialhub.CallOption) (*ContainerStatus, error)
	Publish(context.Context, string, ...socialhub.CallOption) (*socialhub.Post, error)
}

// ContainerService implements ContainerWorkflow.
type ContainerService struct{ client *Client }

func (s *ContainerService) Create(ctx context.Context, input ContainerRequest, options ...socialhub.CallOption) (*ContainerStatus, error) {
	if err := s.client.requireScope("container_create", "instagram_business_content_publish"); err != nil {
		return nil, err
	}
	form, err := containerForm(input)
	if err != nil {
		return nil, err
	}
	var response idResponse
	if err := s.client.form(ctx, http.MethodPost, "/"+url.PathEscape(s.client.userID)+"/media", form, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, wrapError("container_create", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &ContainerStatus{ID: response.ID, StatusCode: "IN_PROGRESS"}, nil
}

func (s *ContainerService) Status(ctx context.Context, containerID string, options ...socialhub.CallOption) (*ContainerStatus, error) {
	if containerID == "" {
		return nil, invalidArgument("container_status", "container ID is required")
	}
	if err := s.client.requireScope("container_status", "instagram_business_content_publish"); err != nil {
		return nil, err
	}
	var response ContainerStatus
	if err := s.client.transport.JSON(ctx, http.MethodGet, "/"+url.PathEscape(containerID), url.Values{"fields": {"id,status_code,status"}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || response.StatusCode == "" {
		return nil, wrapError("container_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (s *ContainerService) Publish(ctx context.Context, containerID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if containerID == "" {
		return nil, invalidArgument("container_publish", "container ID is required")
	}
	if err := s.client.requireScope("container_publish", "instagram_business_content_publish"); err != nil {
		return nil, err
	}
	var response idResponse
	if err := s.client.form(ctx, http.MethodPost, "/"+url.PathEscape(s.client.userID)+"/media_publish", url.Values{"creation_id": {containerID}}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, wrapError("container_publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &socialhub.Post{
		Platform: "instagram", AccountID: s.client.accountID, ID: response.ID, AuthorID: stringPointer(s.client.userID),
		Status:     &socialhub.PublishStatus{ID: response.ID, State: socialhub.PublishStatePublished},
		Extensions: map[string]json.RawMessage{"instagram.container": jsonString(containerID)},
	}, nil
}

func containerForm(input ContainerRequest) (url.Values, error) {
	form := url.Values{}
	if input.Caption != "" {
		form.Set("caption", input.Caption)
	}
	if input.IsAIGenerated {
		form.Set("is_ai_generated", "true")
	}
	if input.CarouselItem && (input.Caption != "" || input.IsAIGenerated) {
		return nil, invalidArgument("container_create", "carousel children cannot carry caption or AI disclosure fields")
	}
	switch input.Type {
	case ContainerImage:
		if !validHTTPSMediaURL(input.MediaURL) || len(input.Children) > 0 {
			return nil, invalidArgument("container_create", "image containers require one absolute HTTPS media URL")
		}
		form.Set("image_url", input.MediaURL)
	case ContainerReel:
		if !validHTTPSMediaURL(input.MediaURL) || len(input.Children) > 0 {
			return nil, invalidArgument("container_create", "reel containers require one absolute HTTPS media URL")
		}
		form.Set("media_type", string(input.Type))
		form.Set("video_url", input.MediaURL)
		if input.ThumbnailOffsetMS < 0 {
			return nil, invalidArgument("container_create", "thumbnail offset must not be negative")
		}
		if input.ThumbnailOffsetMS > 0 {
			form.Set("thumb_offset", strconv.Itoa(input.ThumbnailOffsetMS))
		}
		if input.ShareToFeed != nil {
			form.Set("share_to_feed", strconv.FormatBool(*input.ShareToFeed))
		}
	case ContainerStory:
		if !validHTTPSMediaURL(input.MediaURL) || len(input.Children) > 0 || input.ShareToFeed != nil || input.ThumbnailOffsetMS != 0 {
			return nil, invalidArgument("container_create", "story containers require one HTTPS media URL and do not accept reel-only fields")
		}
		form.Set("media_type", string(input.Type))
		switch input.MediaType {
		case socialhub.MediaTypeImage:
			form.Set("image_url", input.MediaURL)
		case socialhub.MediaTypeVideo:
			form.Set("video_url", input.MediaURL)
		default:
			return nil, invalidArgument("container_create", "story media type must be image or video")
		}
	case ContainerCarousel:
		if input.MediaURL != "" || len(input.Children) < 2 || len(input.Children) > 10 {
			return nil, invalidArgument("container_create", "carousel containers require 2-10 child container IDs and no media URL")
		}
		for _, child := range input.Children {
			if strings.TrimSpace(child) == "" {
				return nil, invalidArgument("container_create", "carousel child IDs must not be empty")
			}
		}
		form.Set("media_type", string(ContainerCarousel))
		form.Set("children", strings.Join(input.Children, ","))
	default:
		return nil, invalidArgument("container_create", "type must be IMAGE, REELS, STORIES, or CAROUSEL")
	}
	if input.CarouselItem {
		if input.Type == ContainerCarousel {
			return nil, invalidArgument("container_create", "a carousel parent cannot be a carousel item")
		}
		form.Set("is_carousel_item", "true")
	}
	return form, nil
}

func validHTTPSMediaURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func jsonString(value string) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func (c *Client) form(ctx context.Context, method, path string, form url.Values, output any, options ...socialhub.CallOption) error {
	encoded := ""
	if form != nil {
		encoded = form.Encode()
	}
	request, err := c.transport.NewRequest(ctx, method, path, nil, strings.NewReader(encoded), options...)
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.transport.Do(request, output)
}

var _ ContainerWorkflow = (*ContainerService)(nil)
