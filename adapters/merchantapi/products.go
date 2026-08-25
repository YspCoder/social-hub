package merchantapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListProducts(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (TokenPage[Product], error) {
	const operation = "products.list"
	if !validListRequest(input, 1000) {
		return TokenPage[Product]{}, invalidArgument(operation, "page size or page token is invalid")
	}
	var response struct {
		Products      []Product `json:"products"`
		NextPageToken string    `json:"nextPageToken"`
	}
	path := "/products/v1/" + client.accountName() + "/products"
	if _, err := client.getJSON(ctx, operation, path, listQuery(input), &response, options...); err != nil {
		return TokenPage[Product]{}, err
	}
	if len(response.Products) > effectivePageSize(input.PageSize, 25) || !validPageToken(response.NextPageToken) {
		return TokenPage[Product]{}, platformContractError(operation, "Merchant API returned invalid Product pagination")
	}
	seen := make(map[string]struct{}, len(response.Products))
	for _, product := range response.Products {
		if !validProduct(client.merchantAccountID, product) {
			return TokenPage[Product]{}, platformContractError(operation, "Merchant API returned a malformed or cross-account Product")
		}
		if _, found := seen[product.Name]; found {
			return TokenPage[Product]{}, platformContractError(operation, "Merchant API returned a duplicate Product")
		}
		seen[product.Name] = struct{}{}
	}
	return TokenPage[Product]{Items: append([]Product(nil), response.Products...), NextPageToken: response.NextPageToken}, nil
}

func (client *Client) GetProduct(ctx context.Context, resourceName string, options ...socialhub.CallOption) (*Product, error) {
	const operation = "products.get"
	if !validProductName(client.merchantAccountID, resourceName) {
		return nil, invalidArgument(operation, "resource name must identify a Product owned by the configured account")
	}
	var response Product
	if _, err := client.getJSON(ctx, operation, "/products/v1/"+resourceName, nil, &response, options...); err != nil {
		return nil, err
	}
	if !validProduct(client.merchantAccountID, response) ||
		!matchesProductResourceName(client.merchantAccountID, "products", resourceName, response.Name, response.Base64EncodedName) {
		return nil, platformContractError(operation, "Merchant API returned a mismatched Product")
	}
	return &response, nil
}

func (client *Client) InsertProductInput(ctx context.Context, input InsertProductInputRequest, options ...socialhub.CallOption) (*ProductInput, error) {
	const operation = "product_inputs.insert"
	if !validInsertProductInput(client.merchantAccountID, input) {
		return nil, invalidArgument(operation, "Data Source, immutable identifiers, version, product attributes, or custom attributes are invalid")
	}
	query := url.Values{"dataSource": {input.DataSource}}
	path := "/products/v1/" + client.accountName() + "/productInputs:insert"
	body := input.Input
	body.OfferID = strings.Join(strings.Fields(body.OfferID), " ")
	var response ProductInput
	header, err := client.sendJSON(ctx, operation, http.MethodPost, path, query, body, &response, true, options...)
	if err != nil {
		return nil, err
	}
	if !validProductInput(client.merchantAccountID, response) || response.OfferID != body.OfferID ||
		response.ContentLanguage != body.ContentLanguage || response.FeedLabel != body.FeedLabel ||
		response.LegacyLocal != body.LegacyLocal {
		cause := platformContractError(operation, "Merchant API returned a malformed or mismatched ProductInput")
		return &response, outcomeUnknownError(operation, cause, responseRequestID(header, client.requestIDs), client.requestIDs)
	}
	return &response, nil
}

func (client *Client) PatchProductInput(ctx context.Context, input PatchProductInputRequest, options ...socialhub.CallOption) (*ProductInput, error) {
	const operation = "product_inputs.patch"
	if !validPatchProductInput(client.merchantAccountID, input) {
		return nil, invalidArgument(operation, "Data Source, ProductInput resource, explicit update mask, or mutable fields are invalid")
	}
	body := input.Input
	body.Name = input.Name
	query := url.Values{
		"dataSource": {input.DataSource},
		"updateMask": {strings.Join(input.UpdateMask, ",")},
	}
	var response ProductInput
	header, err := client.sendJSON(ctx, operation, http.MethodPatch, "/products/v1/"+input.Name, query, body, &response, true, options...)
	if err != nil {
		return nil, err
	}
	if !validProductInput(client.merchantAccountID, response) ||
		!matchesProductResourceName(client.merchantAccountID, "productInputs", input.Name, response.Name, response.Base64EncodedName) {
		cause := platformContractError(operation, "Merchant API returned a mismatched ProductInput")
		return &response, outcomeUnknownError(operation, cause, responseRequestID(header, client.requestIDs), client.requestIDs)
	}
	return &response, nil
}

func (client *Client) DeleteProductInput(ctx context.Context, input DeleteProductInputRequest, options ...socialhub.CallOption) error {
	const operation = "product_inputs.delete"
	if !validDeleteProductInput(client.merchantAccountID, input) {
		return invalidArgument(operation, "Data Source and ProductInput resource must belong to the configured account")
	}
	query := url.Values{"dataSource": {input.DataSource}}
	var response json.RawMessage
	header, err := client.sendJSON(ctx, operation, http.MethodDelete, "/products/v1/"+input.Name, query, nil, &response, true, options...)
	if err != nil {
		return err
	}
	var object map[string]json.RawMessage
	if !validJSONObjectValue(response) || json.Unmarshal(response, &object) != nil || object == nil || len(object) != 0 {
		cause := platformContractError(operation, "Merchant API returned a malformed delete response")
		return outcomeUnknownError(operation, cause, responseRequestID(header, client.requestIDs), client.requestIDs)
	}
	return nil
}

var _ ProductWorkflow = (*Client)(nil)
