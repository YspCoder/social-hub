package dailymotion

import (
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

var videoCategories = map[string]struct{}{
	"animals": {}, "auto": {}, "creation": {}, "fun": {}, "kids": {}, "lifestyle": {}, "music": {}, "news": {},
	"people": {}, "school": {}, "sport": {}, "tech": {}, "travel": {}, "tv": {}, "videogames": {}, "webcam": {},
}

var webhookEvents = map[string]struct{}{
	"video.created": {}, "video.deleted": {}, "video.published": {}, "video.complete": {},
	"video.format.ready": {}, "video.format.processing": {}, "video.format.error": {}, "video.format.deleted": {},
}

func validateCreateVideo(input CreateVideoRequest) error {
	if strings.TrimSpace(input.Title) == "" || utf8.RuneCountInString(input.Title) > 255 {
		return invalidArgument("create_video", "title must contain between 1 and 255 characters")
	}
	if utf8.RuneCountInString(input.Description) > 3000 || !validCategory(input.Category) || !validVideoVisibility(input.Visibility) {
		return invalidArgument("create_video", "description, category, or visibility is invalid")
	}
	if input.Visibility == "password" {
		if utf8.RuneCountInString(input.Password) < 1 || utf8.RuneCountInString(input.Password) > 32 {
			return invalidArgument("create_video", "password visibility requires a 1-32 character password")
		}
	} else if input.Password != "" {
		return invalidArgument("create_video", "password is only valid with password visibility")
	}
	if !validLanguage(input.Language) || !validCountry(input.Country) || utf8.RuneCountInString(input.EngagementMessage) > 500 || !validTags(input.Tags) || !validTags(input.Hashtags) || !validRemoteURL(input.SourceURL) {
		return invalidArgument("create_video", "language, country, engagement message, tags, hashtags, or source URL is invalid")
	}
	return nil
}

func validateUpdateVideo(input UpdateVideoRequest) error {
	if input.Title == nil && input.Description == nil && input.Category == nil && input.Visibility == nil && input.IsForKids == nil && input.IsExplicit == nil && input.Password == nil && input.PublishedAt == nil && input.Language == nil && input.Country == nil && input.EngagementMessage == nil && input.Hashtags == nil && input.Tags == nil && input.IsAIAltered == nil && input.EnableAIChapterGeneration == nil && input.EnableEmbed == nil && input.SourceURL == nil {
		return invalidArgument("update_video", "at least one mutable field is required")
	}
	if input.Title != nil && (strings.TrimSpace(*input.Title) == "" || utf8.RuneCountInString(*input.Title) > 255) {
		return invalidArgument("update_video", "title must contain between 1 and 255 characters")
	}
	if input.Description != nil && utf8.RuneCountInString(*input.Description) > 3000 {
		return invalidArgument("update_video", "description exceeds 3000 characters")
	}
	if input.Category != nil && !validCategory(*input.Category) || input.Visibility != nil && !validVideoVisibility(*input.Visibility) {
		return invalidArgument("update_video", "category or visibility is invalid")
	}
	if input.Password != nil && utf8.RuneCountInString(*input.Password) > 32 || input.Language != nil && !validLanguage(*input.Language) || input.Country != nil && !validCountry(*input.Country) || input.EngagementMessage != nil && utf8.RuneCountInString(*input.EngagementMessage) > 500 || input.Tags != nil && !validTags(*input.Tags) || input.Hashtags != nil && !validTags(*input.Hashtags) || input.SourceURL != nil && !validRemoteURL(*input.SourceURL) {
		return invalidArgument("update_video", "one or more update fields are invalid")
	}
	return nil
}

func validateProfileUpdate(input UpdateProfileRequest) error {
	if input.DisplayName == nil && input.Description == nil && input.SocialLinks == nil && input.Webhook == nil {
		return invalidArgument("update_profile", "at least one mutable field is required")
	}
	if input.DisplayName != nil && strings.TrimSpace(*input.DisplayName) == "" {
		return invalidArgument("update_profile", "display name must not be blank")
	}
	if input.SocialLinks != nil {
		for _, value := range []*string{input.SocialLinks.TwitterURL, input.SocialLinks.InstagramURL, input.SocialLinks.FacebookURL, input.SocialLinks.WebsiteURL} {
			if value != nil && *value != "" && !validRemoteURL(*value) {
				return invalidArgument("update_profile", "social links must be absolute HTTP(S) URLs")
			}
		}
	}
	if input.Webhook != nil {
		if input.Webhook.CallbackURL != nil && *input.Webhook.CallbackURL != "" && !validHTTPSURL(*input.Webhook.CallbackURL) {
			return invalidArgument("update_profile", "webhook callback must be an absolute HTTPS URL")
		}
		seen := make(map[string]struct{}, len(input.Webhook.Events))
		for _, event := range input.Webhook.Events {
			if _, ok := webhookEvents[event]; !ok {
				return invalidArgument("update_profile", "webhook contains an unsupported event")
			}
			if _, duplicate := seen[event]; duplicate {
				return invalidArgument("update_profile", "webhook events must not contain duplicates")
			}
			seen[event] = struct{}{}
		}
	}
	return nil
}

func validCategory(value string) bool { _, ok := videoCategories[value]; return ok }

func validVideoVisibility(value string) bool {
	return value == "public" || value == "private" || value == "password"
}

func validPlaylistVisibility(value string) bool { return value == "public" || value == "private" }

func validLanguage(value string) bool { return value == "" || validAlpha2(value) }
func validCountry(value string) bool  { return value == "" || validAlpha2(value) }

func validAlpha2(value string) bool {
	return len(value) == 2 && ((value[0] >= 'a' && value[0] <= 'z') || (value[0] >= 'A' && value[0] <= 'Z')) && ((value[1] >= 'a' && value[1] <= 'z') || (value[1] >= 'A' && value[1] <= 'Z'))
}

func validTags(values []string) bool {
	if len(values) > 150 {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 150 || strings.ContainsFunc(value, unicode.IsControl) {
			return false
		}
	}
	return true
}

func validRemoteURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.Fragment == ""
}

func validSort(value string, allowed map[string]struct{}) bool {
	if value == "" {
		return true
	}
	for _, field := range strings.Split(value, ",") {
		field = strings.TrimPrefix(field, "-")
		if _, ok := allowed[field]; !ok {
			return false
		}
	}
	return true
}

func validFilename(value string) bool {
	return strings.TrimSpace(value) != "" && utf8.RuneCountInString(value) <= 255 && !strings.ContainsFunc(value, unicode.IsControl) && !strings.ContainsAny(value, `/\\`)
}
