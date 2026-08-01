package imgur

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

// ShareImage publishes one uploaded image to the public Gallery.
func (client *Client) ShareImage(ctx context.Context, input GalleryShareRequest, options ...socialhub.CallOption) error {
	api, err := client.requireUser("share_image")
	if err != nil {
		return err
	}
	if !validIdentifier(input.ImageID) || !validText(input.Title, true) {
		return invalidArgument("share_image", "a valid image ID and non-empty title are required")
	}
	if input.Topic != "" && !validText(input.Topic, false) {
		return invalidArgument("share_image", "topic contains invalid text")
	}
	values := url.Values{"title": {input.Title}, "terms": {"1"}, "mature": {strconv.FormatBool(input.Mature)}}
	if input.Topic != "" {
		values.Set("topic", input.Topic)
	}
	for _, tag := range input.Tags {
		if !validIdentifier(tag) {
			return invalidArgument("share_image", "Gallery tags must be valid identifiers")
		}
		values.Add("tags[]", tag)
	}
	return client.basic(ctx, api, http.MethodPost, path("gallery", "image", input.ImageID), values, options...)
}

// RemoveFromGallery removes a post from Gallery without deleting its source image.
func (client *Client) RemoveFromGallery(ctx context.Context, imageID string, options ...socialhub.CallOption) error {
	api, err := client.requireUser("remove_from_gallery")
	if err != nil {
		return err
	}
	if !validIdentifier(imageID) {
		return invalidArgument("remove_from_gallery", "a valid Gallery image ID is required")
	}
	return client.basic(ctx, api, http.MethodDelete, path("gallery", imageID), nil, options...)
}

// Vote applies an explicit Gallery vote direction.
func (client *Client) Vote(ctx context.Context, targetID string, vote GalleryVote, options ...socialhub.CallOption) error {
	api, err := client.requireUser("gallery_vote")
	if err != nil {
		return err
	}
	if !validIdentifier(targetID) || vote != GalleryVoteUp && vote != GalleryVoteDown && vote != GalleryVoteVeto {
		return invalidArgument("gallery_vote", "a valid Gallery ID and vote direction are required")
	}
	return client.basic(ctx, api, http.MethodPost, path("gallery", targetID, "vote", string(vote)), nil, options...)
}

// Publish maps common publishing to sharing exactly one existing image in Gallery.
func (client *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if input.Text == nil || !validText(*input.Text, true) || len(input.MediaIDs) != 1 || !validIdentifier(input.MediaIDs[0]) {
		return nil, invalidArgument("publish", "Imgur Gallery publishing requires a title and exactly one uploaded image ID")
	}
	if input.ReplyToID != nil || input.QuotePostID != nil {
		return nil, unsupported("publish", "Gallery publishing does not support reply or quote relationships")
	}
	if input.Visibility != nil && *input.Visibility != "public" && *input.Visibility != "public_gallery" {
		return nil, unsupported("publish", "Imgur common publishing supports only public Gallery visibility")
	}
	imageID := input.MediaIDs[0]
	if err := client.ShareImage(ctx, GalleryShareRequest{ImageID: imageID, Title: *input.Text}, options...); err != nil {
		return nil, err
	}
	now := client.clock.Now().UTC()
	visibility := "public_gallery"
	return &socialhub.Post{
		Platform: "imgur", AccountID: client.accountID, ID: imageID, Text: input.Text,
		Media: []socialhub.Media{{ID: imageID, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady}},
		URL:   pointer("https://imgur.com/" + imageID), Visibility: &visibility, CreatedAt: &now,
		Status: &socialhub.PublishStatus{ID: imageID, State: socialhub.PublishStatePublished, UpdatedAt: &now},
	}, nil
}

// PublishStatus reports whether an image is currently shared in Gallery.
func (client *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	image, err := client.GetImage(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	state, message := socialhub.PublishStatePending, "image is not currently shared in Gallery"
	if image.InGallery {
		state, message = socialhub.PublishStatePublished, ""
	}
	return &socialhub.PublishStatus{ID: image.ID, State: state, Message: message, UpdatedAt: unixTime(image.Datetime)}, nil
}

// DeletePost removes the image from Gallery but preserves the source image.
func (client *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	return client.RemoveFromGallery(ctx, postID, options...)
}

// React maps common likes to an Imgur Gallery up-vote.
func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.ActorID != "" && input.ActorID != client.username && input.ActorID != "me" || input.Kind != socialhub.ReactionLike {
		return invalidArgument("react", "Imgur common reactions support Gallery likes by the configured account")
	}
	return client.Vote(ctx, input.TargetID, GalleryVoteUp, options...)
}

// RemoveReaction maps common like removal to Imgur's veto vote.
func (client *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if input.ActorID != "" && input.ActorID != client.username && input.ActorID != "me" || input.Kind != socialhub.ReactionLike {
		return invalidArgument("remove_reaction", "Imgur common reactions support Gallery likes by the configured account")
	}
	return client.Vote(ctx, input.TargetID, GalleryVoteVeto, options...)
}

// ListComments returns Gallery comments in best order and flattens nested replies.
func (client *Client) ListComments(ctx context.Context, input socialhub.ListCommentsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Comment], error) {
	if !validIdentifier(input.PostID) || input.Cursor != "" || input.MaxResults < 0 {
		return socialhub.Page[socialhub.Comment]{}, invalidArgument("list_comments", "a valid Gallery ID, empty cursor, and non-negative max results are required")
	}
	var comments []Comment
	if err := client.request(ctx, client.public, http.MethodGet, path("gallery", input.PostID, "comments", "best"), nil, &comments, options...); err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	items, err := client.mapComments(input.PostID, comments)
	if err != nil {
		return socialhub.Page[socialhub.Comment]{}, err
	}
	if input.MaxResults > 0 && len(items) > input.MaxResults {
		items = items[:input.MaxResults]
	}
	return socialhub.Page[socialhub.Comment]{Items: items}, nil
}

// Comment creates a Gallery comment or reply.
func (client *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	api, err := client.requireUser("comment")
	if err != nil {
		return nil, err
	}
	if !validIdentifier(input.PostID) || !validText(input.Text, true) {
		return nil, invalidArgument("comment", "a valid Gallery ID and non-empty comment are required")
	}
	values := url.Values{"image_id": {input.PostID}, "comment": {input.Text}}
	if input.ParentID != nil {
		if !validIdentifier(*input.ParentID) {
			return nil, invalidArgument("comment", "parent comment ID is invalid")
		}
		values.Set("parent_id", *input.ParentID)
	}
	var response idResponse
	if err := client.form(ctx, api, http.MethodPost, path("comment"), values, &response, options...); err != nil {
		return nil, err
	}
	identifier := string(response.ID)
	if !validIdentifier(identifier) {
		return nil, platformError("comment", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	now := client.clock.Now().UTC()
	return &socialhub.Comment{
		Platform: "imgur", AccountID: client.accountID, ID: identifier, PostID: input.PostID,
		AuthorID: optionalPointer(firstNonEmpty(client.username, "me")), ParentID: input.ParentID, Text: input.Text, CreatedAt: &now,
	}, nil
}

// DeleteComment deletes a comment owned by the authenticated user.
func (client *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	api, err := client.requireUser("delete_comment")
	if err != nil {
		return err
	}
	if !validIdentifier(commentID) {
		return invalidArgument("delete_comment", "a valid comment ID is required")
	}
	return client.basic(ctx, api, http.MethodDelete, path("comment", commentID), nil, options...)
}

var _ GalleryWorkflow = (*Client)(nil)
