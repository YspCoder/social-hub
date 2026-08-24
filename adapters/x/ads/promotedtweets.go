package ads

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListPromotedTweets(ctx context.Context, input ListPromotedTweetsRequest, options ...socialhub.CallOption) (socialhub.Page[PromotedTweet], error) {
	const operation = "promoted_tweets_list"
	if !validList(input.Cursor, input.Count) || len(input.LineItemIDs) > 0 && !validUniqueAdsIDs(input.LineItemIDs, 200) {
		return socialhub.Page[PromotedTweet]{}, invalidArgument(operation, "cursor, count, or Line Item IDs are invalid")
	}
	query := listQuery(input.Cursor, input.Count)
	if len(input.LineItemIDs) > 0 {
		query.Set("line_item_ids", strings.Join(input.LineItemIDs, ","))
	}
	var response listResponse[PromotedTweet]
	if _, err := client.get(ctx, client.resourcePath("promoted_tweets"), query, &response, options...); err != nil {
		return socialhub.Page[PromotedTweet]{}, err
	}
	allowedParents := stringSet(input.LineItemIDs)
	for index := range response.Data {
		if err := validatePromotedTweet(operation, &response.Data[index], "", allowedParents); err != nil {
			return socialhub.Page[PromotedTweet]{}, err
		}
	}
	return cursorPage(operation, response.Data, response.NextCursor)
}

func (client *Client) GetPromotedTweet(ctx context.Context, id string, options ...socialhub.CallOption) (*PromotedTweet, error) {
	const operation = "promoted_tweet_get"
	if !validAdsID(id) {
		return nil, invalidArgument(operation, "Promoted Tweet ID must be base36")
	}
	var response singleResponse[PromotedTweet]
	if _, err := client.get(ctx, client.resourcePath("promoted_tweets")+"/"+id, nil, &response, options...); err != nil {
		return nil, err
	}
	if err := validatePromotedTweet(operation, &response.Data, id, nil); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func (client *Client) AssociateTweets(ctx context.Context, input AssociateTweetsRequest, options ...socialhub.CallOption) ([]PromotedTweet, error) {
	const operation = "promoted_tweets_associate"
	if !validAdsID(input.LineItemID) || !validUniqueTweetIDs(input.TweetIDs, 50) {
		return nil, invalidArgument(operation, "Line Item ID or Tweet IDs are invalid")
	}
	lineItem, err := client.GetLineItem(ctx, input.LineItemID, options...)
	if err != nil {
		return nil, err
	}
	if lineItem.Deleted || lineItem.EntityStatus != StatusPaused || lineItem.ProductType != ProductPromotedTweets {
		return nil, invalidArgument(operation, "Tweets may only be associated with a non-deleted, PAUSED PROMOTED_TWEETS Line Item")
	}
	query := url.Values{"line_item_id": {input.LineItemID}, "tweet_ids": {strings.Join(input.TweetIDs, ",")}}
	var response listResponse[PromotedTweet]
	if _, err := client.write(ctx, http.MethodPost, client.resourcePath("promoted_tweets"), query, &response, options...); err != nil {
		return nil, err
	}
	if len(response.Data) != len(input.TweetIDs) {
		return nil, platformContractError(operation, "X returned an unexpected number of Promoted Tweets")
	}
	remaining := stringSet(input.TweetIDs)
	for index := range response.Data {
		if err := validatePromotedTweet(operation, &response.Data[index], "", map[string]struct{}{input.LineItemID: {}}); err != nil {
			return nil, err
		}
		if _, exists := remaining[response.Data[index].TweetID]; !exists {
			return nil, platformContractError(operation, "X returned an unexpected Tweet association")
		}
		delete(remaining, response.Data[index].TweetID)
	}
	return response.Data, nil
}

func validatePromotedTweet(operation string, value *PromotedTweet, expectedID string, allowedParents map[string]struct{}) error {
	if !validAdsID(value.ID) || expectedID != "" && value.ID != expectedID || !validAdsID(value.LineItemID) || !validTweetID(value.TweetID) {
		return platformContractError(operation, "X returned a missing or mismatched Promoted Tweet identity")
	}
	if len(allowedParents) > 0 {
		if _, exists := allowedParents[value.LineItemID]; !exists {
			return platformContractError(operation, "X returned a Promoted Tweet from another Line Item")
		}
	}
	return nil
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}
