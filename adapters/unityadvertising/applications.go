package unityadvertising

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"social-hub/pkg/socialhub"
)

type App struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	Store                  Store           `json:"store"`
	StoreID                *string         `json:"storeId"`
	ADomain                *string         `json:"adomain"`
	GameID                 *int64          `json:"gameId"`
	CreatedAt              *time.Time      `json:"createdAt"`
	UpdatedAt              *time.Time      `json:"updatedAt"`
	AppAttributionClickURL *string         `json:"appAttributionClickUrl"`
	AppAttributionStartURL *string         `json:"appAttributionStartUrl"`
	Raw                    json.RawMessage `json:"-"`
}

type ListAppsRequest struct {
	Offset  int64
	Limit   int
	Store   Store
	StoreID string
}

type CreateAppRequest interface {
	isCreateAppRequest()
}

type CreateAppleAppRequest struct {
	Store               Store           `json:"store"`
	StoreID             string          `json:"storeId"`
	ADomain             *NullableString `json:"adomain,omitempty"`
	AttributionClickURL *NullableString `json:"appAttributionClickUrl,omitempty"`
	AttributionStartURL *NullableString `json:"appAttributionStartUrl,omitempty"`
}

func (CreateAppleAppRequest) isCreateAppRequest() {}

type CreateGoogleAppRequest struct {
	Store               Store           `json:"store"`
	StoreID             string          `json:"storeId"`
	ADomain             *NullableString `json:"adomain,omitempty"`
	AttributionClickURL *NullableString `json:"appAttributionClickUrl,omitempty"`
	AttributionStartURL *NullableString `json:"appAttributionStartUrl,omitempty"`
}

func (CreateGoogleAppRequest) isCreateAppRequest() {}

type UpdateAppRequest struct {
	ADomain                   *NullableString `json:"adomain,omitempty"`
	AttributionClickURL       *NullableString `json:"appAttributionClickUrl,omitempty"`
	AttributionStartURL       *NullableString `json:"appAttributionStartUrl,omitempty"`
	AppLevelAttributionUpdate *NullableString `json:"appLevelAttributionUpdateType,omitempty"`
}

type AppsWorkflow interface {
	ListApps(context.Context, ListAppsRequest, ...socialhub.CallOption) (Page[App], error)
	GetApp(context.Context, string, ...socialhub.CallOption) (*App, error)
	CreateApp(context.Context, CreateAppRequest, ...socialhub.CallOption) (*App, error)
	UpdateApp(context.Context, string, UpdateAppRequest, ...socialhub.CallOption) (*App, error)
	DeleteApp(context.Context, string, ...socialhub.CallOption) error
}

func (client *Client) ListApps(ctx context.Context, input ListAppsRequest, options ...socialhub.CallOption) (Page[App], error) {
	if input.Offset < 0 || input.Limit < 0 || input.Limit > 1000 || input.Store != "" && !validStore(input.Store) ||
		input.StoreID != "" && !validOpaque(input.StoreID, 4096) {
		return Page[App]{}, invalidArgument("app_list", "offset, limit, or app filter is invalid")
	}
	query := make(url.Values)
	if input.Offset > 0 {
		query.Set("offset", formatInt64(input.Offset))
	}
	if input.Limit > 0 {
		query.Set("limit", formatInt(input.Limit))
	}
	if input.Store != "" {
		query.Set("filter[store]", string(input.Store))
	}
	if input.StoreID != "" {
		query.Set("filter[storeId]", input.StoreID)
	}
	var page Page[App]
	if err := client.getJSON(ctx, "app_list", client.organizationPath()+"/apps", query, &page, options...); err != nil {
		return Page[App]{}, err
	}
	if !validPage(page, 1000, func(app App) bool { return validApp(app) }) {
		return Page[App]{}, platformContractError("app_list", "Unity returned an invalid app page")
	}
	return page, nil
}

func (client *Client) GetApp(ctx context.Context, campaignSetID string, options ...socialhub.CallOption) (*App, error) {
	path, err := client.appPath("app_get", campaignSetID)
	if err != nil {
		return nil, err
	}
	var app App
	if err := client.getJSON(ctx, "app_get", path, nil, &app, options...); err != nil {
		return nil, err
	}
	if !validApp(app) || app.ID != campaignSetID {
		return nil, platformContractError("app_get", "Unity returned an app that does not match the requested ID")
	}
	return &app, nil
}

func (client *Client) CreateApp(ctx context.Context, input CreateAppRequest, options ...socialhub.CallOption) (*App, error) {
	if err := validateCreateApp(input); err != nil {
		return nil, err
	}
	var app App
	if err := client.postJSON(ctx, "app_create", client.organizationPath()+"/apps", input, &app, options...); err != nil {
		return nil, err
	}
	if !validApp(app) {
		return nil, platformContractError("app_create", "Unity returned an invalid app")
	}
	return &app, nil
}

func (client *Client) UpdateApp(ctx context.Context, campaignSetID string, input UpdateAppRequest, options ...socialhub.CallOption) (*App, error) {
	path, err := client.appPath("app_update", campaignSetID)
	if err != nil {
		return nil, err
	}
	if err := validateUpdateApp(input); err != nil {
		return nil, err
	}
	var app App
	if err := client.patchJSON(ctx, "app_update", path, input, &app, options...); err != nil {
		return nil, err
	}
	if !validApp(app) || app.ID != campaignSetID {
		return nil, platformContractError("app_update", "Unity returned an app that does not match the requested ID")
	}
	return &app, nil
}

func (client *Client) DeleteApp(ctx context.Context, campaignSetID string, options ...socialhub.CallOption) error {
	path, err := client.appPath("app_delete", campaignSetID)
	if err != nil {
		return err
	}
	return client.deleteJSON(ctx, "app_delete", path, options...)
}

func validateCreateApp(input CreateAppRequest) error {
	var store Store
	var storeID string
	var adomain, click, start *NullableString
	switch typed := input.(type) {
	case CreateAppleAppRequest:
		store, storeID, adomain, click, start = typed.Store, typed.StoreID, typed.ADomain, typed.AttributionClickURL, typed.AttributionStartURL
		if store != StoreApple || !digitsOnly(storeID) {
			return invalidArgument("app_create", "Apple apps require store=apple and a numeric store ID")
		}
	case *CreateAppleAppRequest:
		if typed == nil {
			return invalidArgument("app_create", "create app request is required")
		}
		return validateCreateApp(*typed)
	case CreateGoogleAppRequest:
		store, storeID, adomain, click, start = typed.Store, typed.StoreID, typed.ADomain, typed.AttributionClickURL, typed.AttributionStartURL
		if store != StoreGoogle || !validOpaque(storeID, 4096) {
			return invalidArgument("app_create", "Google apps require store=google and a store ID")
		}
	case *CreateGoogleAppRequest:
		if typed == nil {
			return invalidArgument("app_create", "create app request is required")
		}
		return validateCreateApp(*typed)
	default:
		return invalidArgument("app_create", "create app request must be Apple or Google")
	}
	_ = store
	_ = storeID
	if !validNullableText(adomain, 4, 100) || !validNullableHTTPSURL(click) || !validNullableHTTPSURL(start) {
		return invalidArgument("app_create", "app domain or attribution URL is invalid")
	}
	return nil
}

func validateUpdateApp(input UpdateAppRequest) error {
	if input.ADomain == nil && input.AttributionClickURL == nil && input.AttributionStartURL == nil && input.AppLevelAttributionUpdate == nil {
		return invalidArgument("app_update", "at least one app field is required")
	}
	if !validNullableText(input.ADomain, 4, 100) || !validNullableHTTPSURL(input.AttributionClickURL) || !validNullableHTTPSURL(input.AttributionStartURL) {
		return invalidArgument("app_update", "app domain or attribution URL is invalid")
	}
	if input.AppLevelAttributionUpdate != nil && input.AppLevelAttributionUpdate.Value != nil {
		switch AttributionUpdateType(*input.AppLevelAttributionUpdate.Value) {
		case AttributionNewAudiences, AttributionNewAndExistingLive, AttributionNewAndExistingAudiences:
		default:
			return invalidArgument("app_update", "app-level attribution update type is invalid")
		}
	}
	return nil
}

func validApp(app App) bool {
	return validMongoID(app.ID) && (app.Store == "" || validStore(app.Store))
}

func (app *App) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*appAlias)(app), &app.Raw)
}

type appAlias App
