package unitypublisher

import (
	"context"

	"social-hub/pkg/socialhub"
)

type ApplicationsWorkflow interface {
	ListApplications(context.Context, ...socialhub.CallOption) ([]Application, error)
	CreateApplication(context.Context, CreateApplicationRequest, MutationOptions, ...socialhub.CallOption) (*Application, error)
	GetApplication(context.Context, string, ...socialhub.CallOption) (*Application, error)
	UpdateApplication(context.Context, string, UpdateApplicationRequest, MutationOptions, ...socialhub.CallOption) (*Application, error)
	GetApplicationTestMode(context.Context, string, ...socialhub.CallOption) (*ApplicationTestMode, error)
	UpdateApplicationTestMode(context.Context, string, UpdateTestModeRequest, MutationOptions, ...socialhub.CallOption) (*ApplicationTestMode, error)
}

func (client *Client) ListApplications(ctx context.Context, options ...socialhub.CallOption) ([]Application, error) {
	const operation = "applications_list"
	var output []Application
	if err := client.getJSON(ctx, operation, client.organizationPath()+"/applications", nil, &output, options...); err != nil {
		return nil, err
	}
	for _, application := range output {
		if !validApplication(application) {
			return nil, platformContractError(operation, "Unity returned an invalid Application")
		}
	}
	return output, nil
}

func (client *Client) CreateApplication(ctx context.Context, input CreateApplicationRequest, mutation MutationOptions, options ...socialhub.CallOption) (*Application, error) {
	const operation = "application_create"
	if !validCreateApplication(input) {
		return nil, invalidArgument(operation, "application name, platform, store, privacy, project, or icon URL is invalid")
	}
	var output Application
	if err := client.postJSON(ctx, operation, client.organizationPath()+"/applications", mutationQuery(mutation), input, &output, options...); err != nil {
		return nil, err
	}
	if !validApplication(output) {
		return nil, platformContractError(operation, "Unity returned an invalid Application")
	}
	return &output, nil
}

func (client *Client) GetApplication(ctx context.Context, applicationID string, options ...socialhub.CallOption) (*Application, error) {
	const operation = "application_get"
	path, err := client.applicationPath(operation, applicationID)
	if err != nil {
		return nil, err
	}
	var output Application
	if err := client.getJSON(ctx, operation, path, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != applicationID || !validApplication(output) {
		return nil, ownershipError(operation, "Application")
	}
	return &output, nil
}

func (client *Client) UpdateApplication(ctx context.Context, applicationID string, input UpdateApplicationRequest, mutation MutationOptions, options ...socialhub.CallOption) (*Application, error) {
	const operation = "application_update"
	path, err := client.applicationPath(operation, applicationID)
	if err != nil {
		return nil, err
	}
	if !validUpdateApplication(input) {
		return nil, invalidArgument(operation, "application patch is empty or contains an invalid name, store, or privacy field")
	}
	var output Application
	if err := client.patchJSON(ctx, operation, path, mutationQuery(mutation), input, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != applicationID || !validApplication(output) {
		return nil, ownershipError(operation, "Application")
	}
	return &output, nil
}

func (client *Client) GetApplicationTestMode(ctx context.Context, applicationID string, options ...socialhub.CallOption) (*ApplicationTestMode, error) {
	const operation = "application_test_mode_get"
	path, err := client.applicationPath(operation, applicationID)
	if err != nil {
		return nil, err
	}
	var output ApplicationTestMode
	if err := client.getJSON(ctx, operation, path+"/test-mode", nil, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != applicationID || !validApplicationTestMode(output) {
		return nil, ownershipError(operation, "Application test mode")
	}
	return &output, nil
}

func (client *Client) UpdateApplicationTestMode(ctx context.Context, applicationID string, input UpdateTestModeRequest, mutation MutationOptions, options ...socialhub.CallOption) (*ApplicationTestMode, error) {
	const operation = "application_test_mode_update"
	path, err := client.applicationPath(operation, applicationID)
	if err != nil {
		return nil, err
	}
	if !validTestMode(input.TestMode) {
		return nil, invalidArgument(operation, "test mode must be forceAll or forceOff")
	}
	var output ApplicationTestMode
	if err := client.patchJSON(ctx, operation, path+"/test-mode", mutationQuery(mutation), input, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != applicationID || !validApplicationTestMode(output) {
		return nil, ownershipError(operation, "Application test mode")
	}
	return &output, nil
}

func validCreateApplication(input CreateApplicationRequest) bool {
	if !validText(input.Name, 1024) || !validPlatform(input.Platform) || !validOptionalURL(input.IconURL) ||
		!validOptionalText(input.StoreID, 1024) || input.Store != nil && !validStore(*input.Store) ||
		!validOptionalText(input.ProjectName, 1024) {
		return false
	}
	if input.ProjectID != nil && (!validUUID(*input.ProjectID) || input.ProjectName != nil) {
		return false
	}
	return true
}

func validUpdateApplication(input UpdateApplicationRequest) bool {
	if input.Name == nil && input.StoreID == nil && input.Store == nil && input.Privacy == nil && input.KidsSettings == nil {
		return false
	}
	return validOptionalText(input.Name, 1024) && validOptionalText(input.StoreID, 1024) &&
		(input.Store == nil || validStore(*input.Store))
}

func validApplication(value Application) bool {
	return validPathID(value.ID) && validText(value.Name, 1024) && validPlatform(value.Platform) &&
		(value.ProjectID == nil || validUUID(*value.ProjectID)) && (value.Store == nil || validStore(*value.Store)) &&
		(value.TestMode == nil || validTestMode(*value.TestMode))
}

func validApplicationTestMode(value ApplicationTestMode) bool {
	return validPathID(value.ID) && (value.TestMode == nil || validTestMode(*value.TestMode))
}

func validTestMode(value TestMode) bool {
	return value == TestModeForceAll || value == TestModeForceOff
}
