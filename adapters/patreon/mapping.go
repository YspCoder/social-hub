package patreon

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func mapUser(accountID socialhub.AccountID, input userResource) *socialhub.User {
	extension := input.Raw
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	accountType := "patreon"
	return &socialhub.User{
		Platform: "patreon", AccountID: accountID, ID: input.ID,
		Username: cleanStringPointer(input.Attributes.Vanity), DisplayName: firstStringPointer(input.Attributes.FullName, input.Attributes.FirstName),
		AvatarURL: firstStringPointer(input.Attributes.ImageURL, input.Attributes.ThumbURL), ProfileURL: cleanStringPointer(input.Attributes.URL), AccountType: &accountType,
		Extensions: map[string]json.RawMessage{"patreon.user": append(json.RawMessage(nil), extension...)},
	}
}

func mapPost(accountID socialhub.AccountID, input postResource, observedAt time.Time) *socialhub.Post {
	extension := input.Raw
	if len(extension) == 0 {
		extension, _ = json.Marshal(input)
	}
	post := &socialhub.Post{
		Platform: "patreon", AccountID: accountID, ID: input.ID,
		Text: firstStringPointer(input.Attributes.Content, input.Attributes.Title), CreatedAt: input.Attributes.PublishedAt,
		URL: cleanStringPointer(input.Attributes.URL), Status: mapPublishStatus(input, observedAt),
		Extensions: map[string]json.RawMessage{"patreon.post": append(json.RawMessage(nil), extension...)},
	}
	if input.Relationships.User.Data != nil && validResourceID(input.Relationships.User.Data.ID) {
		post.AuthorID = stringPointer(input.Relationships.User.Data.ID)
	}
	if input.Attributes.IsPublic != nil {
		visibility := "patrons"
		if *input.Attributes.IsPublic {
			visibility = "public"
		}
		post.Visibility = &visibility
	}
	if input.Attributes.EmbedURL != nil && strings.TrimSpace(*input.Attributes.EmbedURL) != "" {
		post.Media = []socialhub.Media{{URL: *input.Attributes.EmbedURL, Type: socialhub.MediaTypeDocument, State: socialhub.MediaStateReady}}
	}
	return post
}

func mapPublishStatus(input postResource, observedAt time.Time) *socialhub.PublishStatus {
	state := socialhub.PublishStatePending
	if input.Attributes.PublishedAt != nil {
		state = socialhub.PublishStatePublished
	}
	message := stringValue(input.Attributes.AppStatus)
	switch strings.ToLower(message) {
	case "failed", "error", "rejected":
		state = socialhub.PublishStateFailed
	}
	updatedAt := input.Attributes.PublishedAt
	if updatedAt == nil {
		updatedAt = &observedAt
	}
	return &socialhub.PublishStatus{ID: input.ID, State: state, Message: message, UpdatedAt: updatedAt}
}

func mapCampaign(input campaignResource) Campaign {
	creatorID := ""
	if input.Relationships.Creator.Data != nil {
		creatorID = input.Relationships.Creator.Data.ID
	}
	return Campaign{
		ID: input.ID, CreatorID: creatorID, Name: cleanStringPointer(input.Attributes.Name),
		CreationName: cleanStringPointer(input.Attributes.CreationName), Summary: cleanStringPointer(input.Attributes.Summary),
		URL: cleanStringPointer(input.Attributes.URL), ImageURL: firstStringPointer(input.Attributes.ImageURL, input.Attributes.ImageSmallURL),
		Vanity: cleanStringPointer(input.Attributes.Vanity), Currency: cleanStringPointer(input.Attributes.Currency),
		PatronCount: input.Attributes.PatronCount, Monthly: input.Attributes.Monthly, NSFW: input.Attributes.NSFW,
		CreatedAt: input.Attributes.CreatedAt, PublishedAt: input.Attributes.PublishedAt, Raw: append(json.RawMessage(nil), input.Raw...),
	}
}

func mapMember(input memberResource) Member {
	member := Member{
		ID: input.ID, FullName: cleanStringPointer(input.Attributes.FullName), Email: cleanStringPointer(input.Attributes.Email),
		PatronStatus: cleanStringPointer(input.Attributes.PatronStatus), LastChargeStatus: cleanStringPointer(input.Attributes.LastChargeStatus),
		LastChargeDate: input.Attributes.LastChargeDate, PledgeRelationshipStart: input.Attributes.PledgeRelationshipStart,
		LifetimeSupportCents: input.Attributes.CampaignLifetimeSupportCents,
		EntitledAmountCents:  input.Attributes.CurrentlyEntitledAmountCents, WillPayAmountCents: input.Attributes.WillPayAmountCents,
		Raw: append(json.RawMessage(nil), input.Raw...),
	}
	if input.Relationships.Campaign.Data != nil {
		member.CampaignID = input.Relationships.Campaign.Data.ID
	}
	if input.Relationships.User.Data != nil {
		member.UserID = input.Relationships.User.Data.ID
	}
	for _, tier := range input.Relationships.CurrentlyEntitledTiers.Data {
		if validResourceID(tier.ID) {
			member.EntitledTierIDs = append(member.EntitledTierIDs, tier.ID)
		}
	}
	return member
}

func cleanStringPointer(value *string) *string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return nil
	}
	copy := *value
	return &copy
}

func firstStringPointer(values ...*string) *string {
	for _, value := range values {
		if clean := cleanStringPointer(value); clean != nil {
			return clean
		}
	}
	return nil
}

func stringPointer(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
