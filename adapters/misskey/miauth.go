package misskey

import (
	"context"
	"net/url"
	"strings"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

// MiAuthClient implements Misskey's app-registration-free authorization flow.
type MiAuthClient struct {
	accountID   socialhub.AccountID
	instanceURL string
	api         *transport.Client
}

func (c *MiAuthClient) AuthorizationURL(input MiAuthRequest) (string, error) {
	if !validSessionID(input.Session) {
		return "", invalidArgument("miauth_url", "session must be a canonical UUID")
	}
	if input.Name != "" && !validBoundedString(input.Name, 256) {
		return "", invalidArgument("miauth_url", "application name is invalid")
	}
	if input.IconURL != "" && !validHTTPURL(input.IconURL) {
		return "", invalidArgument("miauth_url", "icon URL must be HTTP(S)")
	}
	if input.CallbackURL != "" && !validHTTPURL(input.CallbackURL) {
		return "", invalidArgument("miauth_url", "callback URL must be HTTP(S)")
	}
	if err := validatePermissions(input.Permissions); err != nil {
		return "", err
	}
	query := url.Values{}
	if input.Name != "" {
		query.Set("name", input.Name)
	}
	if input.IconURL != "" {
		query.Set("icon", input.IconURL)
	}
	if input.CallbackURL != "" {
		query.Set("callback", input.CallbackURL)
	}
	if len(input.Permissions) > 0 {
		query.Set("permission", strings.Join(input.Permissions, ","))
	}
	result := c.instanceURL + "/miauth/" + url.PathEscape(input.Session)
	if encoded := query.Encode(); encoded != "" {
		result += "?" + encoded
	}
	return result, nil
}

func (c *MiAuthClient) Check(ctx context.Context, session string, options ...socialhub.CallOption) (*MiAuthResult, error) {
	if !validSessionID(session) {
		return nil, invalidArgument("miauth_check", "session must be a canonical UUID")
	}
	var response struct {
		OK    bool         `json:"ok"`
		Token string       `json:"token"`
		User  *misskeyUser `json:"user"`
	}
	if err := c.api.JSON(ctx, "POST", "/miauth/"+url.PathEscape(session)+"/check", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	result := &MiAuthResult{OK: response.OK}
	if !response.OK {
		return result, nil
	}
	if !validBoundedString(response.Token, 4096) || response.User == nil {
		return nil, platformError("miauth_check", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	mapper := Client{accountID: c.accountID, instanceURL: c.instanceURL}
	user, err := mapper.mapUser(*response.User)
	if err != nil {
		return nil, err
	}
	result.AccessToken, result.User = response.Token, user
	return result, nil
}

func validatePermissions(values []string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validPermission(value) {
			return invalidArgument("miauth_url", "permissions are invalid")
		}
		if _, exists := seen[value]; exists {
			return invalidArgument("miauth_url", "permissions must be unique")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validPermission(value string) bool {
	if !validBoundedString(value, 128) {
		return false
	}
	for _, character := range value {
		if !((character >= 'a' && character <= 'z') || (character >= '0' && character <= '9') ||
			character == ':' || character == '-') {
			return false
		}
	}
	return true
}

func validSessionID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}
