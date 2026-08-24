package googlebusinessprofile

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if input.Text == nil || !validRequiredText(*input.Text, 100_000) {
		return nil, invalidArgument("publish", "common Google Business Profile publishing requires bounded text")
	}
	if len(input.MediaIDs) > 0 {
		return nil, unsupported("publish", "Local Posts require public sourceUrl media; use LocalPostWorkflow")
	}
	if input.ReplyToID != nil || input.QuotePostID != nil {
		return nil, unsupported("publish", "Google Business Profile Local Posts do not support reply or quote publication fields")
	}
	if input.Visibility != nil && *input.Visibility != "" && *input.Visibility != "public" {
		return nil, unsupported("publish", "Google Business Profile Local Posts only support public visibility")
	}
	post, err := client.CreateLocalPost(ctx, LocalPostCreateRequest{
		LanguageCode: client.languageCode, Summary: *input.Text, TopicType: LocalPostStandard,
	}, options...)
	if err != nil {
		return nil, err
	}
	return mapPost(client.accountID, client.locationID, post), nil
}

func (client *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := client.getLocalPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return mapPublishStatus(post), nil
}

func (client *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	if !validResourceSegment(postID) {
		return invalidArgument("delete_post", "local post ID must be a bounded resource ID segment")
	}
	if err := client.requireScope("delete_post"); err != nil {
		return err
	}
	return client.api.JSON(ctx, http.MethodDelete, "/"+client.localPostResource(postID), nil, nil, nil, options...)
}

func (client *Client) CreateLocalPost(ctx context.Context, input LocalPostCreateRequest, options ...socialhub.CallOption) (*LocalPost, error) {
	if input.LanguageCode == "" {
		input.LanguageCode = client.languageCode
	}
	if err := validateLocalPostCreate(input); err != nil {
		return nil, err
	}
	if err := client.requireScope("local_post_create"); err != nil {
		return nil, err
	}
	var response LocalPost
	path := "/" + client.locationResource() + "/localPosts"
	if err := client.api.JSON(ctx, http.MethodPost, path, nil, input, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateLocalPost("local_post_create", &response, ""); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) UpdateLocalPost(ctx context.Context, postID string, input LocalPostPatchRequest, options ...socialhub.CallOption) (*LocalPost, error) {
	if !validResourceSegment(postID) {
		return nil, invalidArgument("local_post_update", "local post ID must be a bounded resource ID segment")
	}
	mask, err := validateLocalPostPatch(input)
	if err != nil {
		return nil, err
	}
	if err := client.requireScope("local_post_update"); err != nil {
		return nil, err
	}
	query := url.Values{"updateMask": {strings.Join(mask, ",")}}
	var response LocalPost
	if err := client.api.JSON(ctx, http.MethodPatch, "/"+client.localPostResource(postID), query, input, &response, options...); err != nil {
		return nil, err
	}
	if err := client.validateLocalPost("local_post_update", &response, postID); err != nil {
		return nil, err
	}
	return &response, nil
}

func validateLocalPostCreate(input LocalPostCreateRequest) error {
	if input.LanguageCode != "" && !validLanguageCode(input.LanguageCode) || !validOptionalText(input.Summary, 100_000) {
		return invalidArgument("local_post_create", "language code or summary is invalid")
	}
	if input.TopicType != LocalPostStandard && input.TopicType != LocalPostEvent && input.TopicType != LocalPostOffer {
		return unsupported("local_post_create", "topic_type must be STANDARD, EVENT, or OFFER; ALERT authoring is not generally available")
	}
	if strings.TrimSpace(input.Summary) == "" && len(input.Media) == 0 && input.Event == nil {
		return invalidArgument("local_post_create", "summary, media, or event content is required")
	}
	if input.CallToAction != nil && !validCallToAction(*input.CallToAction) {
		return invalidArgument("local_post_create", "call to action is invalid")
	}
	if !validLocalPostMedia(input.Media) {
		return invalidArgument("local_post_create", "Local Post media requires a documented format and public source URL")
	}
	if input.TopicType == LocalPostEvent || input.TopicType == LocalPostOffer {
		if input.Event == nil || !validEvent(*input.Event) {
			return invalidArgument("local_post_create", "EVENT and OFFER posts require a valid event and complete schedule")
		}
	} else if input.Event != nil {
		return invalidArgument("local_post_create", "event is only valid for EVENT and OFFER posts")
	}
	if input.TopicType == LocalPostOffer {
		if input.Offer == nil || !validOffer(*input.Offer) {
			return invalidArgument("local_post_create", "OFFER posts require valid offer details")
		}
		if input.CallToAction != nil {
			return invalidArgument("local_post_create", "callToAction is ignored for OFFER posts and must be omitted")
		}
	} else if input.Offer != nil {
		return invalidArgument("local_post_create", "offer is only valid for OFFER posts")
	}
	return nil
}

func validateLocalPostPatch(input LocalPostPatchRequest) ([]string, error) {
	mask := make([]string, 0, 8)
	if input.LanguageCode != nil {
		if !validLanguageCode(*input.LanguageCode) {
			return nil, invalidArgument("local_post_update", "language code is invalid")
		}
		mask = append(mask, "languageCode")
	}
	if input.Summary != nil {
		if !validOptionalText(*input.Summary, 100_000) {
			return nil, invalidArgument("local_post_update", "summary is invalid")
		}
		mask = append(mask, "summary")
	}
	if input.CallToAction != nil {
		if !validCallToAction(*input.CallToAction) {
			return nil, invalidArgument("local_post_update", "call to action is invalid")
		}
		mask = append(mask, "callToAction")
	}
	if input.ScheduledTime != nil {
		if input.ScheduledTime.IsZero() {
			return nil, invalidArgument("local_post_update", "scheduled time is invalid")
		}
		mask = append(mask, "scheduledTime")
	}
	if input.Event != nil {
		if !validEvent(*input.Event) {
			return nil, invalidArgument("local_post_update", "event is invalid")
		}
		mask = append(mask, "event")
	}
	if input.Media != nil {
		if !validLocalPostMedia(*input.Media) {
			return nil, invalidArgument("local_post_update", "media is invalid")
		}
		mask = append(mask, "media")
	}
	if input.TopicType != nil {
		if *input.TopicType != LocalPostStandard && *input.TopicType != LocalPostEvent && *input.TopicType != LocalPostOffer {
			return nil, unsupported("local_post_update", "topic type must be STANDARD, EVENT, or OFFER")
		}
		mask = append(mask, "topicType")
	}
	if input.Offer != nil {
		if !validOffer(*input.Offer) {
			return nil, invalidArgument("local_post_update", "offer is invalid")
		}
		mask = append(mask, "offer")
	}
	if len(mask) == 0 {
		return nil, invalidArgument("local_post_update", "at least one mutable Local Post field is required")
	}
	return mask, nil
}

func validCallToAction(input CallToAction) bool {
	switch input.ActionType {
	case ActionCall:
		return input.URL == ""
	case ActionBook, ActionOrder, ActionShop, ActionLearnMore, ActionSignUp:
		return validPublicURL(input.URL)
	default:
		return false
	}
}

func validLocalPostMedia(items []LocalPostMedia) bool {
	for _, item := range items {
		if item.MediaFormat != MediaFormatPhoto && item.MediaFormat != MediaFormatVideo || !validPublicURL(item.SourceURL) {
			return false
		}
	}
	return true
}

func validEvent(input LocalPostEventDetails) bool {
	return validRequiredText(input.Title, 4096) && validTimeInterval(input.Schedule)
}

func validTimeInterval(input TimeInterval) bool {
	if !validDate(input.StartDate) || !validDate(input.EndDate) || !validTimeOfDay(input.StartTime) || !validTimeOfDay(input.EndTime) {
		return false
	}
	start := localDateTime(input.StartDate, input.StartTime)
	end := localDateTime(input.EndDate, input.EndTime)
	return !end.Before(start)
}

func validDate(input Date) bool {
	if input.Year < 1 || input.Year > 9999 || input.Month < 1 || input.Month > 12 || input.Day < 1 || input.Day > 31 {
		return false
	}
	value := time.Date(input.Year, time.Month(input.Month), input.Day, 0, 0, 0, 0, time.UTC)
	return value.Year() == input.Year && int(value.Month()) == input.Month && value.Day() == input.Day
}

func validTimeOfDay(input TimeOfDay) bool {
	return input.Hours >= 0 && input.Hours <= 23 && input.Minutes >= 0 && input.Minutes <= 59 &&
		input.Seconds >= 0 && input.Seconds <= 59 && input.Nanos >= 0 && input.Nanos <= 999_999_999
}

func localDateTime(date Date, clock TimeOfDay) time.Time {
	return time.Date(date.Year, time.Month(date.Month), date.Day, clock.Hours, clock.Minutes, clock.Seconds, clock.Nanos, time.UTC)
}

func validOffer(input LocalPostOfferDetails) bool {
	return validOptionalText(input.CouponCode, 4096) && validOptionalText(input.TermsConditions, 100_000) &&
		(input.RedeemOnlineURL == "" || validPublicURL(input.RedeemOnlineURL))
}
