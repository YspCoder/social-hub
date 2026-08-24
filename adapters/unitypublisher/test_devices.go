package unitypublisher

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

type TestDevicesWorkflow interface {
	ListTestDevices(context.Context, Platform, ...socialhub.CallOption) ([]TestDevice, error)
	CreateTestDevice(context.Context, CreateTestDeviceRequest, MutationOptions, ...socialhub.CallOption) (*TestDevice, error)
	GetTestDevice(context.Context, string, ...socialhub.CallOption) (*TestDevice, error)
	UpdateTestDevice(context.Context, string, UpdateTestDeviceRequest, MutationOptions, ...socialhub.CallOption) (*TestDevice, error)
	DeleteTestDevice(context.Context, string, MutationOptions, ...socialhub.CallOption) error
}

func (client *Client) ListTestDevices(ctx context.Context, platform Platform, options ...socialhub.CallOption) ([]TestDevice, error) {
	const operation = "test_devices_list"
	query := make(url.Values)
	if platform != "" {
		if !validPlatform(platform) {
			return nil, invalidArgument(operation, "platform filter is invalid")
		}
		query.Set("platform", string(platform))
	}
	var output []TestDevice
	if err := client.getJSON(ctx, operation, client.organizationPath()+"/test-devices", query, &output, options...); err != nil {
		return nil, err
	}
	for _, device := range output {
		if !validTestDevice(device) {
			return nil, platformContractError(operation, "Unity returned an invalid Test Device")
		}
	}
	return output, nil
}

func (client *Client) CreateTestDevice(ctx context.Context, input CreateTestDeviceRequest, mutation MutationOptions, options ...socialhub.CallOption) (*TestDevice, error) {
	const operation = "test_device_create"
	if !validCreateTestDevice(input) {
		return nil, invalidArgument(operation, "test device name, advertising ID, or platform is invalid")
	}
	var output TestDevice
	if err := client.postJSON(ctx, operation, client.organizationPath()+"/test-devices", mutationQuery(mutation), input, &output, options...); err != nil {
		return nil, err
	}
	if !validTestDevice(output) {
		return nil, platformContractError(operation, "Unity returned an invalid Test Device")
	}
	return &output, nil
}

func (client *Client) GetTestDevice(ctx context.Context, testDeviceID string, options ...socialhub.CallOption) (*TestDevice, error) {
	const operation = "test_device_get"
	path, err := client.testDevicePath(operation, testDeviceID)
	if err != nil {
		return nil, err
	}
	var output TestDevice
	if err := client.getJSON(ctx, operation, path, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != testDeviceID || !validTestDevice(output) {
		return nil, ownershipError(operation, "Test Device")
	}
	return &output, nil
}

func (client *Client) UpdateTestDevice(ctx context.Context, testDeviceID string, input UpdateTestDeviceRequest, mutation MutationOptions, options ...socialhub.CallOption) (*TestDevice, error) {
	const operation = "test_device_update"
	path, err := client.testDevicePath(operation, testDeviceID)
	if err != nil {
		return nil, err
	}
	if !validUpdateTestDevice(input) {
		return nil, invalidArgument(operation, "test device patch is empty or contains an invalid field")
	}
	var output TestDevice
	if err := client.patchJSON(ctx, operation, path, mutationQuery(mutation), input, &output, options...); err != nil {
		return nil, err
	}
	if output.ID != testDeviceID || !validTestDevice(output) {
		return nil, ownershipError(operation, "Test Device")
	}
	return &output, nil
}

func (client *Client) DeleteTestDevice(ctx context.Context, testDeviceID string, mutation MutationOptions, options ...socialhub.CallOption) error {
	const operation = "test_device_delete"
	path, err := client.testDevicePath(operation, testDeviceID)
	if err != nil {
		return err
	}
	return client.deleteJSON(ctx, operation, path, mutationQuery(mutation), options...)
}

func validCreateTestDevice(input CreateTestDeviceRequest) bool {
	return validText(input.Name, 1024) && validText(input.AdvertisingID, 1024) && validNullablePlatform(input.Platform)
}

func validUpdateTestDevice(input UpdateTestDeviceRequest) bool {
	if input.Platform == nil && input.Name == nil && input.AdvertisingID == nil {
		return false
	}
	return validNullablePlatform(input.Platform) && validOptionalText(input.Name, 1024) && validOptionalText(input.AdvertisingID, 1024)
}

func validNullablePlatform(value *NullablePlatform) bool {
	return value == nil || value.Value == nil || validPlatform(*value.Value)
}

func validTestDevice(value TestDevice) bool {
	return validUUID(value.ID) && validText(value.Name, 1024) && validText(value.AdvertisingID, 1024) &&
		(value.Platform == nil || validPlatform(*value.Platform))
}
