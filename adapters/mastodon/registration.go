package mastodon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

const maxRegistrationResponseBytes int64 = 1 << 20

// AppRegistrationRequest describes dynamic OAuth application registration.
type AppRegistrationRequest struct {
	ClientName   string
	RedirectURIs []string
	Scopes       []string
	Website      string
}

// RegisteredApplication contains credentials returned once by an instance.
// ClientSecret must be persisted through a SecretResolver-backed store.
type RegisteredApplication struct {
	ID                    string
	Name                  string
	Website               string
	ClientID              string
	ClientSecret          string
	ClientSecretExpiresAt *time.Time
	RedirectURIs          []string
	Scopes                []string
}

// RegisterApplication creates an OAuth application on the account's instance.
func (a *Adapter) RegisterApplication(ctx context.Context, accountID socialhub.AccountID, input AppRegistrationRequest) (*RegisteredApplication, error) {
	if err := validateRegistration(input); err != nil {
		return nil, err
	}
	a.mu.RLock()
	if !a.ready || a.closed {
		a.mu.RUnlock()
		return nil, platformError("register_application", socialhub.CodeConflict, socialhub.ClassPermanent, nil)
	}
	account, found := a.config.Account(accountID)
	httpClient := a.options.HTTPClient
	a.mu.RUnlock()
	if !found {
		return nil, platformError("register_application", socialhub.CodeNotFound, socialhub.ClassPermanent, nil)
	}
	var settings AccountSettings
	if err := socialhub.DecodeSettings(account.Settings, &settings); err != nil {
		return nil, platformError("register_application", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	payload := struct {
		ClientName   string   `json:"client_name"`
		RedirectURIs []string `json:"redirect_uris"`
		Scopes       string   `json:"scopes"`
		Website      string   `json:"website,omitempty"`
	}{input.ClientName, input.RedirectURIs, strings.Join(input.Scopes, " "), input.Website}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, platformError("register_application", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, normalizeInstanceURL(settings.InstanceURL)+"/api/v1/apps", bytes.NewReader(encoded))
	if err != nil {
		return nil, platformError("register_application", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, platformError("register_application", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxRegistrationResponseBytes+1))
	if err != nil {
		return nil, platformError("register_application", socialhub.CodeTemporarilyUnavailable, socialhub.ClassRetryable, err)
	}
	if int64(len(body)) > maxRegistrationResponseBytes {
		return nil, platformError("register_application", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, decodeHTTPError(response.StatusCode, response.Header, body)
	}
	var result struct {
		ID                    string   `json:"id"`
		Name                  string   `json:"name"`
		Website               string   `json:"website"`
		ClientID              string   `json:"client_id"`
		ClientSecret          string   `json:"client_secret"`
		ClientSecretExpiresAt int64    `json:"client_secret_expires_at"`
		RedirectURI           string   `json:"redirect_uri"`
		RedirectURIs          []string `json:"redirect_uris"`
		Scopes                []string `json:"scopes"`
	}
	if err := json.Unmarshal(body, &result); err != nil || result.ClientID == "" || result.ClientSecret == "" {
		return nil, platformError("register_application", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if len(result.RedirectURIs) == 0 && result.RedirectURI != "" {
		result.RedirectURIs = strings.Fields(result.RedirectURI)
	}
	var expiresAt *time.Time
	if result.ClientSecretExpiresAt > 0 {
		value := time.Unix(result.ClientSecretExpiresAt, 0).UTC()
		expiresAt = &value
	}
	return &RegisteredApplication{
		ID: result.ID, Name: result.Name, Website: result.Website, ClientID: result.ClientID, ClientSecret: result.ClientSecret,
		ClientSecretExpiresAt: expiresAt, RedirectURIs: result.RedirectURIs, Scopes: result.Scopes,
	}, nil
}

func validateRegistration(input AppRegistrationRequest) error {
	if strings.TrimSpace(input.ClientName) == "" || len(input.RedirectURIs) == 0 || len(input.Scopes) == 0 {
		return invalidArgument("register_application", "client name, redirect URIs, and scopes are required")
	}
	for _, value := range input.RedirectURIs {
		if value == "urn:ietf:wg:oauth:2.0:oob" {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
			return invalidArgument("register_application", "redirect URIs must be absolute HTTP(S) URLs or the OAuth OOB URN")
		}
	}
	if input.Website != "" {
		parsed, err := url.Parse(input.Website)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" || parsed.User != nil {
			return invalidArgument("register_application", "website must be an absolute HTTP(S) URL")
		}
	}
	for _, scope := range input.Scopes {
		if strings.TrimSpace(scope) == "" || strings.ContainsAny(scope, " \t\r\n") {
			return invalidArgument("register_application", fmt.Sprintf("invalid OAuth scope %q", scope))
		}
	}
	return nil
}
