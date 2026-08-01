package threads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) CreateContainer(ctx context.Context, input ContainerRequest, options ...socialhub.CallOption) (*ContainerStatus, error) {
	required := []string{"threads_content_publish"}
	if input.LocationID != "" {
		required = append(required, "threads_location_tagging")
	}
	if err := c.requireScope("container_create", required...); err != nil {
		return nil, err
	}
	form, err := containerForm(input)
	if err != nil {
		return nil, err
	}
	var response idResponse
	if err := c.form(ctx, http.MethodPost, "/me/threads", form, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("container_create", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &ContainerStatus{ID: response.ID, Status: "IN_PROGRESS"}, nil
}

func (c *Client) ContainerStatus(ctx context.Context, containerID string, options ...socialhub.CallOption) (*ContainerStatus, error) {
	if strings.TrimSpace(containerID) == "" {
		return nil, invalidArgument("container_status", "container ID is required")
	}
	if err := c.requireScope("container_status", "threads_content_publish"); err != nil {
		return nil, err
	}
	var response containerStatusResponse
	if err := c.get(ctx, "/"+url.PathEscape(containerID), url.Values{"fields": {"id,status,error_message"}}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" || response.Status == "" {
		return nil, platformError("container_status", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &ContainerStatus{ID: response.ID, Status: response.Status, ErrorMessage: response.ErrorMessage}, nil
}

func (c *Client) PublishContainer(ctx context.Context, containerID string, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if strings.TrimSpace(containerID) == "" {
		return nil, invalidArgument("container_publish", "container ID is required")
	}
	if err := c.requireScope("container_publish", "threads_content_publish"); err != nil {
		return nil, err
	}
	var response idResponse
	if err := c.form(ctx, http.MethodPost, "/me/threads_publish", url.Values{"creation_id": {containerID}}, &response, options...); err != nil {
		return nil, err
	}
	if response.ID == "" {
		return nil, platformError("container_publish", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := c.clock.Now()
	extension, _ := json.Marshal(map[string]string{"container_id": containerID})
	return &socialhub.Post{
		Platform: "threads", AccountID: c.accountID, ID: response.ID, AuthorID: stringPointer(c.userID),
		CreatedAt: &now, Visibility: stringPointer("public"),
		Status:     &socialhub.PublishStatus{ID: response.ID, State: socialhub.PublishStatePublished, UpdatedAt: &now},
		Extensions: map[string]json.RawMessage{"threads.container": extension},
	}, nil
}

func containerForm(input ContainerRequest) (url.Values, error) {
	form := url.Values{"media_type": {string(input.Type)}}
	if input.Text != "" {
		form.Set("text", input.Text)
	}
	if input.AltText != "" {
		form.Set("alt_text", input.AltText)
	}
	switch input.Type {
	case ContainerText:
		if input.ImageURL != "" || input.VideoURL != "" || len(input.Children) > 0 || input.CarouselItem || input.SpoilerMedia || input.AltText != "" {
			return nil, invalidArgument("container_create", "text containers cannot include media URLs, children, or media-only flags")
		}
		if strings.TrimSpace(input.Text) == "" && input.LinkAttachmentURL == "" {
			return nil, invalidArgument("container_create", "text containers require text or a link attachment")
		}
	case ContainerImage:
		if !validHTTPSURL(input.ImageURL) || input.VideoURL != "" || len(input.Children) > 0 {
			return nil, invalidArgument("container_create", "image containers require one public HTTPS image URL")
		}
		form.Set("image_url", input.ImageURL)
	case ContainerVideo:
		if !validHTTPSURL(input.VideoURL) || input.ImageURL != "" || len(input.Children) > 0 {
			return nil, invalidArgument("container_create", "video containers require one public HTTPS video URL")
		}
		form.Set("video_url", input.VideoURL)
	case ContainerCarousel:
		if input.ImageURL != "" || input.VideoURL != "" || input.AltText != "" || input.CarouselItem || len(input.Children) < 2 || len(input.Children) > 20 {
			return nil, invalidArgument("container_create", "carousel parents require 2-20 child IDs and no media URL")
		}
		for _, child := range input.Children {
			if strings.TrimSpace(child) == "" {
				return nil, invalidArgument("container_create", "carousel child IDs must not be empty")
			}
		}
		form.Set("children", strings.Join(input.Children, ","))
	default:
		return nil, invalidArgument("container_create", "type must be TEXT, IMAGE, VIDEO, or CAROUSEL")
	}
	if input.CarouselItem {
		if input.Type != ContainerImage && input.Type != ContainerVideo {
			return nil, invalidArgument("container_create", "only image or video containers can be carousel items")
		}
		if input.Text != "" || input.ReplyToID != "" || input.QuotePostID != "" || input.ReplyControl != "" ||
			input.TopicTag != "" || input.LocationID != "" || input.LinkAttachmentURL != "" || input.Poll != nil ||
			input.GhostPost || input.EnableReplyApprovals {
			return nil, invalidArgument("container_create", "carousel items can only carry their media URL, alt text, and spoiler flag")
		}
		form.Set("is_carousel_item", "true")
	}
	if input.ReplyToID != "" {
		form.Set("reply_to_id", input.ReplyToID)
	}
	if input.QuotePostID != "" {
		form.Set("quote_post_id", input.QuotePostID)
	}
	if input.ReplyControl != "" {
		if !validReplyControl(input.ReplyControl) {
			return nil, invalidArgument("container_create", "reply control is invalid")
		}
		form.Set("reply_control", string(input.ReplyControl))
	}
	if input.TopicTag != "" {
		form.Set("topic_tag", input.TopicTag)
	}
	if input.LocationID != "" {
		form.Set("location_id", input.LocationID)
	}
	if input.LinkAttachmentURL != "" {
		if input.Type != ContainerText || !validHTTPSURL(input.LinkAttachmentURL) || input.Poll != nil {
			return nil, invalidArgument("container_create", "link attachments require a text container, HTTPS URL, and no poll")
		}
		form.Set("link_attachment", input.LinkAttachmentURL)
	}
	if input.Poll != nil {
		if input.Type != ContainerText || input.LinkAttachmentURL != "" || strings.TrimSpace(input.Poll.OptionA) == "" || strings.TrimSpace(input.Poll.OptionB) == "" {
			return nil, invalidArgument("container_create", "polls require a text container, two choices, and no link attachment")
		}
		if input.Poll.OptionD != "" && strings.TrimSpace(input.Poll.OptionC) == "" {
			return nil, invalidArgument("container_create", "poll option D requires option C")
		}
		encoded, err := json.Marshal(input.Poll)
		if err != nil {
			return nil, platformError("container_create", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		form.Set("poll_attachment", string(encoded))
	}
	if input.SpoilerMedia {
		form.Set("is_spoiler_media", "true")
	}
	if input.GhostPost {
		form.Set("is_ghost_post", "true")
	}
	if input.EnableReplyApprovals {
		form.Set("enable_reply_approvals", "true")
	}
	return form, nil
}

func validReplyControl(value ReplyControl) bool {
	switch value {
	case ReplyEveryone, ReplyAccountsYouFollow, ReplyMentionedOnly, ReplyParentPostAuthorOnly, ReplyFollowersOnly:
		return true
	default:
		return false
	}
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}
