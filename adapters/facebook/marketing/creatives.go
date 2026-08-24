package marketing

import (
	"context"
	"net/http"
	"net/url"

	"social-hub/pkg/socialhub"
)

const adCreativeFields = "id,account_id,name,object_story_id,effective_object_story_id,thumbnail_url,body,title,url_tags"

func (client *Client) GetAdCreative(ctx context.Context, creativeID string, options ...socialhub.CallOption) (*AdCreative, error) {
	if !validNumericID(creativeID) {
		return nil, invalidArgument("creative_get", "creative ID must be numeric")
	}
	if err := client.requireRead("creative_get"); err != nil {
		return nil, err
	}
	var response AdCreative
	query := url.Values{"fields": {adCreativeFields}}
	if err := client.api.JSON(ctx, http.MethodGet, "/"+url.PathEscape(creativeID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("creative_get", creativeID, response.ID); err != nil {
		return nil, err
	}
	return &response, nil
}

func (client *Client) ListAdCreatives(ctx context.Context, input ListAdCreativesRequest, options ...socialhub.CallOption) (socialhub.Page[AdCreative], error) {
	limit, err := validatePage(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[AdCreative]{}, err
	}
	if err := client.requireRead("creative_list"); err != nil {
		return socialhub.Page[AdCreative]{}, err
	}
	query := url.Values{"fields": {adCreativeFields}}
	addPaging(query, input.Cursor, limit)
	var response graphPage[AdCreative]
	if err := client.api.JSON(ctx, http.MethodGet, "/"+client.adAccountResource()+"/adcreatives", query, nil, &response, options...); err != nil {
		return socialhub.Page[AdCreative]{}, err
	}
	for _, item := range response.Data {
		if err := requireResponseID("creative_list", "", item.ID); err != nil {
			return socialhub.Page[AdCreative]{}, err
		}
	}
	return toPage(response), nil
}

func (client *Client) CreateAdCreative(ctx context.Context, input CreateAdCreativeRequest, options ...socialhub.CallOption) (*AdCreative, error) {
	if !validRequiredText(input.Name, 512) || (input.ObjectStoryID == "") == (input.ObjectStorySpec == nil) {
		return nil, invalidArgument("creative_create", "name and exactly one object story source are required")
	}
	if input.ObjectStoryID != "" && !validObjectID(input.ObjectStoryID) || input.ObjectStorySpec != nil && !validateObjectStorySpec(input.ObjectStorySpec) ||
		!validOptionalText(input.URLTags, 4096) {
		return nil, invalidArgument("creative_create", "object story source or URL tags are invalid")
	}
	if err := client.requireManagement("creative_create"); err != nil {
		return nil, err
	}
	form := url.Values{"name": {input.Name}}
	if input.ObjectStoryID != "" {
		form.Set("object_story_id", input.ObjectStoryID)
	} else if err := setJSONForm(form, "object_story_spec", input.ObjectStorySpec, "creative_create"); err != nil {
		return nil, err
	}
	if input.URLTags != "" {
		form.Set("url_tags", input.URLTags)
	}
	var response idResponse
	if err := client.postForm(ctx, "/"+client.adAccountResource()+"/adcreatives", form, &response, options...); err != nil {
		return nil, err
	}
	if err := requireResponseID("creative_create", "", response.ID); err != nil {
		return nil, err
	}
	return &AdCreative{
		ID: response.ID, AccountID: client.adAccountID, Name: input.Name, ObjectStoryID: input.ObjectStoryID,
		EffectiveObjectStoryID: response.EffectiveObjectStoryID, URLTags: input.URLTags,
	}, nil
}
