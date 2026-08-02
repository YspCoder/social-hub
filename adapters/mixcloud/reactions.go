package mixcloud

import (
	"context"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if !client.validActor(input.ActorID) {
		return invalidArgument("react", "actor must match the configured Mixcloud user")
	}
	var err error
	switch input.Kind {
	case socialhub.ReactionLike:
		_, err = client.Favourite(ctx, input.TargetID, options...)
	case socialhub.ReactionRepost:
		_, err = client.Repost(ctx, input.TargetID, options...)
	default:
		return unsupported("react", "Mixcloud common reactions support LIKE and REPOST")
	}
	return err
}

func (client *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	if !client.validActor(input.ActorID) {
		return invalidArgument("remove_reaction", "actor must match the configured Mixcloud user")
	}
	var err error
	switch input.Kind {
	case socialhub.ReactionLike:
		_, err = client.Unfavourite(ctx, input.TargetID, options...)
	case socialhub.ReactionRepost:
		_, err = client.Unrepost(ctx, input.TargetID, options...)
	default:
		return unsupported("remove_reaction", "Mixcloud common reaction removal supports LIKE and REPOST")
	}
	return err
}

func (client *Client) Comment(context.Context, socialhub.CreateCommentRequest, ...socialhub.CallOption) (*socialhub.Comment, error) {
	return nil, unsupported("comment", "Mixcloud documents comment reads but no comment creation endpoint")
}

func (client *Client) DeleteComment(context.Context, string, ...socialhub.CallOption) error {
	return unsupported("delete_comment", "Mixcloud does not document comment deletion")
}

func (client *Client) Favourite(ctx context.Context, key string, options ...socialhub.CallOption) (*ActionResponse, error) {
	return client.cloudcastAction(ctx, http.MethodPost, key, "favorite", options...)
}

func (client *Client) Unfavourite(ctx context.Context, key string, options ...socialhub.CallOption) (*ActionResponse, error) {
	return client.cloudcastAction(ctx, http.MethodDelete, key, "favorite", options...)
}

func (client *Client) Repost(ctx context.Context, key string, options ...socialhub.CallOption) (*ActionResponse, error) {
	return client.cloudcastAction(ctx, http.MethodPost, key, "repost", options...)
}

func (client *Client) Unrepost(ctx context.Context, key string, options ...socialhub.CallOption) (*ActionResponse, error) {
	return client.cloudcastAction(ctx, http.MethodDelete, key, "repost", options...)
}

func (client *Client) ListenLater(ctx context.Context, key string, options ...socialhub.CallOption) (*ActionResponse, error) {
	return client.cloudcastAction(ctx, http.MethodPost, key, "listen-later", options...)
}

func (client *Client) RemoveListenLater(ctx context.Context, key string, options ...socialhub.CallOption) (*ActionResponse, error) {
	return client.cloudcastAction(ctx, http.MethodDelete, key, "listen-later", options...)
}

func (client *Client) Follow(ctx context.Context, user string, options ...socialhub.CallOption) (*ActionResponse, error) {
	username, _, ok := parseUserKey(user)
	if !ok {
		return nil, invalidArgument("follow", "username or user key is invalid")
	}
	return client.action(ctx, http.MethodPost, "/"+username+"/follow/", "follow", options...)
}

func (client *Client) Unfollow(ctx context.Context, user string, options ...socialhub.CallOption) (*ActionResponse, error) {
	username, _, ok := parseUserKey(user)
	if !ok {
		return nil, invalidArgument("unfollow", "username or user key is invalid")
	}
	return client.action(ctx, http.MethodDelete, "/"+username+"/follow/", "unfollow", options...)
}

func (client *Client) cloudcastAction(ctx context.Context, method, key, actionName string, options ...socialhub.CallOption) (*ActionResponse, error) {
	username, slug, _, ok := parseCloudcastKey(key)
	if !ok {
		return nil, invalidArgument(actionName, "Cloudcast key must contain a username and slug")
	}
	path := "/" + username + "/" + slug + "/" + actionName + "/"
	return client.action(ctx, method, path, actionName, options...)
}

func (client *Client) action(ctx context.Context, method, path, operation string, options ...socialhub.CallOption) (*ActionResponse, error) {
	var response ActionResponse
	if err := client.request(ctx, method, path, nil, nil, "", &response, options...); err != nil {
		return nil, err
	}
	if response.Result == nil || !response.Result.Success {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) validActor(actor string) bool {
	if actor == "" {
		return true
	}
	username, _, ok := parseUserKey(actor)
	return ok && strings.EqualFold(username, client.username)
}
