package analyticsdata

import (
	"context"

	"social-hub/pkg/socialhub"
)

type MetadataWorkflow interface {
	GetMetadata(context.Context, ...socialhub.CallOption) (*MetadataResponse, error)
	CheckCompatibility(context.Context, CheckCompatibilityRequest, ...socialhub.CallOption) (*CompatibilityResponse, error)
}

func (client *Client) GetMetadata(ctx context.Context, options ...socialhub.CallOption) (*MetadataResponse, error) {
	const operation = "metadata_get"
	var output MetadataResponse
	if err := client.getJSON(ctx, operation, "/v1beta/"+client.propertyName()+"/metadata", nil, &output, options...); err != nil {
		return nil, err
	}
	if !validMetadataResponse(&output, client.propertyName()+"/metadata") {
		return nil, platformContractError(operation, "Google Analytics returned malformed or cross-property metadata")
	}
	return &output, nil
}

func (client *Client) CheckCompatibility(ctx context.Context, input CheckCompatibilityRequest, options ...socialhub.CallOption) (*CompatibilityResponse, error) {
	const operation = "compatibility_check"
	if !validCompatibilityRequest(input) {
		return nil, invalidArgument(operation, "dimensions, metrics, filters, or compatibility filter are invalid")
	}
	var output CompatibilityResponse
	if err := client.postJSON(ctx, operation, "/v1beta/"+client.propertyName()+":checkCompatibility", input, &output, options...); err != nil {
		return nil, err
	}
	if !validCompatibilityResponse(&output, input.CompatibilityFilter) {
		return nil, platformContractError(operation, "Google Analytics returned malformed compatibility metadata")
	}
	return &output, nil
}

var _ MetadataWorkflow = (*Client)(nil)
