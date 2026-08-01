package whatsapp

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

const profileFields = "about,address,description,email,profile_picture_url,websites,vertical"

type profileWire struct {
	MessagingProduct  string           `json:"messaging_product"`
	About             string           `json:"about"`
	Address           string           `json:"address"`
	Description       string           `json:"description"`
	Email             string           `json:"email"`
	ProfilePictureURL string           `json:"profile_picture_url"`
	Websites          []string         `json:"websites"`
	Vertical          string           `json:"vertical"`
	BusinessProfile   *BusinessProfile `json:"business_profile"`
}

func (c *Client) GetBusinessProfile(ctx context.Context, options ...socialhub.CallOption) (*BusinessProfile, error) {
	if err := c.requireScope("get_business_profile", "whatsapp_business_management"); err != nil {
		return nil, err
	}
	var response struct {
		Data []profileWire `json:"data"`
	}
	if err := c.request(ctx, http.MethodGet, c.phonePath("whatsapp_business_profile"), url.Values{"fields": {profileFields}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != 1 {
		return nil, platformError("get_business_profile", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	wire := response.Data[0]
	if wire.BusinessProfile != nil {
		return wire.BusinessProfile, nil
	}
	return &BusinessProfile{
		MessagingProduct: wire.MessagingProduct, About: wire.About, Address: wire.Address,
		Description: wire.Description, Email: wire.Email, ProfilePictureURL: wire.ProfilePictureURL,
		Websites: wire.Websites, Vertical: wire.Vertical,
	}, nil
}

func (c *Client) UpdateBusinessProfile(ctx context.Context, input BusinessProfileUpdate, options ...socialhub.CallOption) error {
	body := map[string]any{"messaging_product": "whatsapp"}
	if input.About != nil {
		if lengthOutside(*input.About, 0, 139) {
			return invalidArgument("update_business_profile", "about must contain 1-139 characters when set")
		}
		body["about"] = *input.About
	}
	if input.Address != nil {
		if utf8.RuneCountInString(*input.Address) > 256 {
			return invalidArgument("update_business_profile", "address must not exceed 256 characters")
		}
		body["address"] = *input.Address
	}
	if input.Description != nil {
		if utf8.RuneCountInString(*input.Description) > 256 {
			return invalidArgument("update_business_profile", "description must not exceed 256 characters")
		}
		body["description"] = *input.Description
	}
	if input.Email != nil {
		if utf8.RuneCountInString(*input.Email) > 128 {
			return invalidArgument("update_business_profile", "email must not exceed 128 characters")
		}
		body["email"] = *input.Email
	}
	if input.ProfilePictureHandle != nil {
		body["profile_picture_handle"] = *input.ProfilePictureHandle
	}
	if input.Websites != nil {
		if len(*input.Websites) > 2 {
			return invalidArgument("update_business_profile", "at most two websites are supported")
		}
		for _, website := range *input.Websites {
			if utf8.RuneCountInString(website) > 256 || !validWebURL(website) {
				return invalidArgument("update_business_profile", "websites must be valid HTTP(S) URLs up to 256 characters")
			}
		}
		body["websites"] = *input.Websites
	}
	if input.Vertical != nil {
		if !validVertical(*input.Vertical) {
			return invalidArgument("update_business_profile", "vertical is invalid")
		}
		body["vertical"] = strings.ToUpper(*input.Vertical)
	}
	if len(body) == 1 {
		return invalidArgument("update_business_profile", "at least one profile field is required")
	}
	if err := c.requireScope("update_business_profile", "whatsapp_business_management"); err != nil {
		return err
	}
	var response successPayload
	if err := c.request(ctx, http.MethodPost, c.phonePath("whatsapp_business_profile"), nil, body, &response, options...); err != nil {
		return err
	}
	return requireSuccess(response, "update_business_profile")
}

func lengthOutside(value string, minimum, maximum int) bool {
	length := utf8.RuneCountInString(value)
	return length <= minimum || length > maximum
}

func validWebURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func validVertical(value string) bool {
	valid := []string{"", "UNDEFINED", "OTHER", "AUTO", "BEAUTY", "APPAREL", "EDU", "ENTERTAIN", "EVENT_PLAN", "FINANCE", "GROCERY", "GOVT", "HOTEL", "HEALTH", "NONPROFIT", "PROF_SERVICES", "RETAIL", "TRAVEL", "RESTAURANT", "NOT_A_BIZ"}
	for _, candidate := range valid {
		if strings.EqualFold(value, candidate) {
			return true
		}
	}
	return false
}
