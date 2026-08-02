package deviantart

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) PostDeviationComment(ctx context.Context, input DeviationCommentRequest, options ...socialhub.CallOption) (*Comment, error) {
	if !validResourceID(input.DeviationID) || input.ParentID != "" && !validResourceID(input.ParentID) || strings.TrimSpace(input.Body) == "" {
		return nil, invalidArgument("post_deviation_comment", "Deviation ID, optional parent ID, and body are invalid")
	}
	if err := client.requireScopes("post_deviation_comment", "browse", "comment.post"); err != nil {
		return nil, err
	}
	values := url.Values{"body": {input.Body}}
	if input.ParentID != "" {
		values.Set("commentid", input.ParentID)
	}
	var response Comment
	path := apiPath("comments", "post", "deviation", url.PathEscape(input.DeviationID))
	if err := client.form(ctx, path, values, &response, options...); err != nil {
		return nil, err
	}
	if !validResourceID(response.CommentID) {
		return nil, platformError("post_deviation_comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) Favourite(ctx context.Context, input FavouriteRequest, options ...socialhub.CallOption) (*FavouriteResponse, error) {
	return client.favourite(ctx, "/collections/fave", "favourite", input, options...)
}

func (client *Client) Unfavourite(ctx context.Context, input FavouriteRequest, options ...socialhub.CallOption) (*FavouriteResponse, error) {
	return client.favourite(ctx, "/collections/unfave", "unfavourite", input, options...)
}

func (client *Client) favourite(ctx context.Context, path, operation string, input FavouriteRequest, options ...socialhub.CallOption) (*FavouriteResponse, error) {
	if !validResourceID(input.DeviationID) {
		return nil, invalidArgument(operation, "Deviation ID is invalid")
	}
	values := url.Values{"deviationid": {input.DeviationID}}
	seen := make(map[string]struct{}, len(input.FolderIDs))
	for _, folderID := range input.FolderIDs {
		if !validResourceID(folderID) {
			return nil, invalidArgument(operation, "Collection folder ID is invalid")
		}
		if _, duplicate := seen[folderID]; duplicate {
			return nil, invalidArgument(operation, "Collection folder IDs must be unique")
		}
		seen[folderID] = struct{}{}
		values.Add("folderid", folderID)
	}
	if err := client.requireScopes(operation, "browse", "collection"); err != nil {
		return nil, err
	}
	var response FavouriteResponse
	if err := client.form(ctx, path, values, &response, options...); err != nil {
		return nil, err
	}
	if !response.Success || response.Favourites < 0 {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike || !client.validActor(input.ActorID) {
		return invalidArgument("react", "DeviantArt supports only a favourite by the configured account")
	}
	_, err := client.Favourite(ctx, FavouriteRequest{DeviationID: input.TargetID}, options...)
	return err
}

func (client *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.Kind != socialhub.ReactionLike || !client.validActor(input.ActorID) {
		return invalidArgument("remove_reaction", "DeviantArt supports only removal of a favourite by the configured account")
	}
	_, err := client.Unfavourite(ctx, FavouriteRequest{DeviationID: input.TargetID}, options...)
	return err
}

func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	request := DeviationCommentRequest{DeviationID: input.PostID, Body: input.Text}
	if input.ParentID != nil {
		request.ParentID = *input.ParentID
	}
	response, err := client.PostDeviationComment(ctx, request, options...)
	if err != nil {
		return nil, err
	}
	return client.mapComment(input.PostID, *response)
}

func (client *Client) DeleteComment(context.Context, string, ...socialhub.CallOption) error {
	return unsupported("delete_comment", "DeviantArt API v1 does not expose comment deletion")
}
