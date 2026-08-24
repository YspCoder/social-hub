package panglemanagement

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	createAppPath = "/union/media/open_api/site/create"
	updateAppPath = "/union/media/open_api/site/update"
	queryAppPath  = "/union/media/open_api/site/query"
)

type createAppWire struct {
	authWire
	Status       AppStatus `json:"status"`
	CategoryCode int       `json:"app_category_code"`
	Name         string    `json:"app_name"`
	DownloadURL  string    `json:"download_url,omitempty"`
	MaskRuleID   *int64    `json:"mask_rule_id,omitempty"`
	MaskRuleIDs  *[]ID     `json:"mask_rule_ids,omitempty"`
	COPPA        *COPPA    `json:"coppa_value,omitempty"`
}

type updateAppWire struct {
	authWire
	AppID        ID         `json:"app_id"`
	Status       *AppStatus `json:"status,omitempty"`
	CategoryCode *int       `json:"app_category_code,omitempty"`
	Name         *string    `json:"app_name,omitempty"`
	DownloadURL  *string    `json:"download_url,omitempty"`
	MaskRuleIDs  *[]ID      `json:"mask_rule_ids,omitempty"`
	COPPA        *COPPA     `json:"coppa_value,omitempty"`
}

type listAppsWire struct {
	authWire
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	IDs      []ID        `json:"app_id,omitempty"`
	Names    []string    `json:"app_name,omitempty"`
	OS       []OSType    `json:"os_type,omitempty"`
	Statuses []AppStatus `json:"status,omitempty"`
}

type appMutationData struct {
	AppID  ID        `json:"app_id"`
	Status AppStatus `json:"status"`
}

type listAppsData struct {
	PageInfo PageInfo `json:"page_info"`
	Apps     []App    `json:"app_list"`
}

func (client *Client) CreateApp(ctx context.Context, input CreateAppRequest, options ...socialhub.CallOption) (AppMutationResult, error) {
	const operation = "app_create"
	if err := validateCreateApp(input, client.sandbox); err != nil {
		return AppMutationResult{}, err
	}
	auth, err := client.newAuth(operation)
	if err != nil {
		return AppMutationResult{}, err
	}
	wire := createAppWire{
		authWire: auth, Status: input.Status, CategoryCode: input.CategoryCode, Name: input.Name,
		DownloadURL: input.DownloadURL, MaskRuleID: input.MaskRuleID,
		MaskRuleIDs: input.MaskRuleIDs, COPPA: input.COPPA,
	}
	envelope, status, header, err := client.doJSON(ctx, operation, createAppPath, wire, auth.Sign, true, options...)
	if err != nil {
		return AppMutationResult{}, err
	}
	code := scalarCode(envelope.Code)
	pending := code == "50007"
	if code != "0" && !pending {
		if code == "" {
			failure := platformContractError(operation, "Pangle response omitted a valid business code", status)
			return AppMutationResult{}, outcomeUnknownError(operation, failure)
		}
		return AppMutationResult{}, businessError(operation, status, header, code, envelope.RequestID, client.clock.Now())
	}
	var data appMutationData
	if err := requireData(operation, envelope, status, &data); err != nil {
		return AppMutationResult{}, outcomeUnknownError(operation, err)
	}
	expectedStatus := AppStatusLive
	if client.sandbox {
		expectedStatus = AppStatusTest
	}
	if !validNumericID(string(data.AppID)) || pending && data.Status != AppStatusReview || !pending && data.Status != expectedStatus {
		failure := platformContractError(operation, "Pangle returned invalid app creation data", status)
		return AppMutationResult{}, outcomeUnknownError(operation, failure)
	}
	return AppMutationResult{
		AppID: data.AppID, Status: data.Status, PendingReview: pending, RequestID: envelope.RequestID,
	}, nil
}

func (client *Client) UpdateApp(ctx context.Context, input UpdateAppRequest, options ...socialhub.CallOption) (AppMutationResult, error) {
	const operation = "app_update"
	if err := validateUpdateApp(input, client.sandbox); err != nil {
		return AppMutationResult{}, err
	}
	auth, err := client.newAuth(operation)
	if err != nil {
		return AppMutationResult{}, err
	}
	wire := updateAppWire{
		authWire: auth, AppID: input.AppID, Status: input.Status, CategoryCode: input.CategoryCode,
		Name: input.Name, DownloadURL: input.DownloadURL, MaskRuleIDs: input.MaskRuleIDs, COPPA: input.COPPA,
	}
	envelope, status, header, err := client.doJSON(ctx, operation, updateAppPath, wire, auth.Sign, true, options...)
	if err != nil {
		return AppMutationResult{}, err
	}
	code := scalarCode(envelope.Code)
	pending := code == "50007"
	if code != "0" && !pending {
		if code == "" {
			failure := platformContractError(operation, "Pangle response omitted a valid business code", status)
			return AppMutationResult{}, outcomeUnknownError(operation, failure)
		}
		return AppMutationResult{}, businessError(operation, status, header, code, envelope.RequestID, client.clock.Now())
	}
	var data appMutationData
	if err := requireData(operation, envelope, status, &data); err != nil {
		return AppMutationResult{}, outcomeUnknownError(operation, err)
	}
	if data.AppID != input.AppID || !validAppStatus(data.Status) {
		failure := platformContractError(operation, "Pangle returned invalid app update data", status)
		return AppMutationResult{}, outcomeUnknownError(operation, failure)
	}
	return AppMutationResult{
		AppID: data.AppID, Status: data.Status, PendingReview: pending, RequestID: envelope.RequestID,
	}, nil
}

func (client *Client) ListApps(ctx context.Context, input ListAppsRequest, options ...socialhub.CallOption) (AppPage, error) {
	const operation = "apps_list"
	if err := validateListApps(input); err != nil {
		return AppPage{}, err
	}
	auth, err := client.newAuth(operation)
	if err != nil {
		return AppPage{}, err
	}
	wire := listAppsWire{
		authWire: auth, Page: input.Page, PageSize: input.PageSize,
		IDs: input.IDs, Names: input.Names, OS: input.OS, Statuses: input.Statuses,
	}
	envelope, status, header, err := client.doJSON(ctx, operation, queryAppPath, wire, auth.Sign, false, options...)
	if err != nil {
		return AppPage{}, err
	}
	code := scalarCode(envelope.Code)
	if code != "0" {
		if code == "" {
			return AppPage{}, platformContractError(operation, "Pangle response omitted a valid business code", status)
		}
		return AppPage{}, businessError(operation, status, header, code, envelope.RequestID, client.clock.Now())
	}
	var data listAppsData
	if err := requireData(operation, envelope, status, &data); err != nil {
		return AppPage{}, err
	}
	if !validPageInfo(data.PageInfo, input.Page, input.PageSize, len(data.Apps)) {
		return AppPage{}, platformContractError(operation, "Pangle returned invalid app pagination data", status)
	}
	for _, app := range data.Apps {
		if !validAppResponse(app, client.sandbox) {
			return AppPage{}, platformContractError(operation, "Pangle returned invalid app data", status)
		}
	}
	return AppPage{
		Apps: data.Apps, PageInfo: data.PageInfo, HasMore: data.PageInfo.Page < data.PageInfo.TotalPages,
	}, nil
}
