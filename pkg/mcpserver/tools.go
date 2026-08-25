package mcpserver

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"social-hub/pkg/socialhub"
)

type emptyInput struct{}

type targetInput struct {
	Target TargetRef `json:"target"`
}

type getUserInput struct {
	Target TargetRef   `json:"target"`
	UserID string      `json:"user_id" jsonschema:"Platform user identifier"`
	Call   CallControl `json:"call,omitempty"`
}

type getPostInput struct {
	Target TargetRef   `json:"target"`
	PostID string      `json:"post_id" jsonschema:"Platform post identifier"`
	Call   CallControl `json:"call,omitempty"`
}

type listPostsInput struct {
	Target     TargetRef   `json:"target"`
	UserID     string      `json:"user_id,omitempty" jsonschema:"Platform user identifier; empty selects the configured account when supported"`
	Cursor     string      `json:"cursor,omitempty" jsonschema:"Opaque continuation cursor returned by the preceding call"`
	MaxResults int         `json:"max_results,omitempty" jsonschema:"Requested page size; platform limits still apply"`
	StartTime  *time.Time  `json:"start_time,omitempty"`
	EndTime    *time.Time  `json:"end_time,omitempty"`
	Call       CallControl `json:"call,omitempty"`
}

type listCommentsInput struct {
	Target     TargetRef   `json:"target"`
	PostID     string      `json:"post_id" jsonschema:"Platform post identifier"`
	Cursor     string      `json:"cursor,omitempty" jsonschema:"Opaque continuation cursor returned by the preceding call"`
	MaxResults int         `json:"max_results,omitempty" jsonschema:"Requested page size; platform limits still apply"`
	Call       CallControl `json:"call,omitempty"`
}

type getMessageInput struct {
	Target    TargetRef   `json:"target"`
	MessageID string      `json:"message_id" jsonschema:"Platform message identifier"`
	Call      CallControl `json:"call,omitempty"`
}

type publishStatusInput struct {
	Target    TargetRef   `json:"target"`
	PublishID string      `json:"publish_id" jsonschema:"Publication or container identifier returned by publish_post"`
	Call      CallControl `json:"call,omitempty"`
}

type publishPostInput struct {
	Target      TargetRef   `json:"target"`
	Text        *string     `json:"text,omitempty"`
	MediaIDs    []string    `json:"media_ids,omitempty" jsonschema:"Identifiers of media already uploaded through a trusted application workflow"`
	ReplyToID   *string     `json:"reply_to_id,omitempty"`
	QuotePostID *string     `json:"quote_post_id,omitempty"`
	Visibility  *string     `json:"visibility,omitempty"`
	Call        CallControl `json:"call,omitempty"`
}

type deletePostInput struct {
	Target TargetRef   `json:"target"`
	PostID string      `json:"post_id"`
	Call   CallControl `json:"call,omitempty"`
}

type reactionInput struct {
	Target   TargetRef              `json:"target"`
	ActorID  string                 `json:"actor_id" jsonschema:"Platform user identifier performing the reaction"`
	TargetID string                 `json:"target_id" jsonschema:"Platform post identifier receiving the reaction"`
	Kind     socialhub.ReactionKind `json:"kind" jsonschema:"Normalized reaction kind, such as like or repost"`
	Call     CallControl            `json:"call,omitempty"`
}

type createCommentInput struct {
	Target   TargetRef   `json:"target"`
	PostID   string      `json:"post_id"`
	ParentID *string     `json:"parent_id,omitempty"`
	Text     string      `json:"text"`
	Call     CallControl `json:"call,omitempty"`
}

type deleteCommentInput struct {
	Target    TargetRef   `json:"target"`
	CommentID string      `json:"comment_id"`
	Call      CallControl `json:"call,omitempty"`
}

type sendMessageInput struct {
	Target         TargetRef   `json:"target"`
	ConversationID string      `json:"conversation_id,omitempty"`
	RecipientIDs   []string    `json:"recipient_ids,omitempty"`
	Text           *string     `json:"text,omitempty"`
	MediaIDs       []string    `json:"media_ids,omitempty" jsonschema:"Identifiers of media already uploaded through a trusted application workflow"`
	ReplyToID      *string     `json:"reply_to_id,omitempty"`
	Call           CallControl `json:"call,omitempty"`
}

func (s *Service) registerReadTools(server *mcp.Server) {
	mcp.AddTool(server, readTool("socialhub_list_targets", "List configured adapter and account targets without exposing credentials."), s.listTargets)
	mcp.AddTool(server, readTool("socialhub_get_capabilities", "Get group-level capability and provider approval declarations for one configured target."), s.getCapabilities)
	mcp.AddTool(server, readTool("socialhub_get_user", "Get a normalized social-platform user by identifier."), s.getUser)
	mcp.AddTool(server, readTool("socialhub_get_post", "Get a normalized social post by identifier."), s.getPost)
	mcp.AddTool(server, readTool("socialhub_list_posts", "List normalized posts with platform-defined cursor pagination."), s.listPosts)
	mcp.AddTool(server, readTool("socialhub_list_comments", "List normalized comments for one post with platform-defined cursor pagination."), s.listComments)
	mcp.AddTool(server, readTool("socialhub_get_message", "Get a normalized message when the selected adapter supports message retrieval."), s.getMessage)
	mcp.AddTool(server, readTool("socialhub_get_publish_status", "Get asynchronous publication status when the selected adapter supports status lookup."), s.getPublishStatus)
}

func (s *Service) registerMutationTools(server *mcp.Server) {
	if s.policy.allows(OperationPublishPost) {
		mcp.AddTool(server, writeTool("socialhub_publish_post", "Publish one post to the explicitly selected account. Requires user confirmation."), s.publishPost)
	}
	if s.policy.allows(OperationDeletePost) {
		mcp.AddTool(server, destructiveTool("socialhub_delete_post", "Permanently delete one post from the explicitly selected account. Requires user confirmation."), s.deletePost)
	}
	if s.policy.allows(OperationAddReaction) {
		mcp.AddTool(server, writeTool("socialhub_add_reaction", "Add one reaction to a post as an explicitly identified actor. Requires user confirmation."), s.addReaction)
	}
	if s.policy.allows(OperationRemoveReaction) {
		mcp.AddTool(server, destructiveTool("socialhub_remove_reaction", "Remove one reaction from a post as an explicitly identified actor. Requires user confirmation."), s.removeReaction)
	}
	if s.policy.allows(OperationCreateComment) {
		mcp.AddTool(server, writeTool("socialhub_create_comment", "Create one comment or reply on the explicitly selected account. Requires user confirmation."), s.createComment)
	}
	if s.policy.allows(OperationDeleteComment) {
		mcp.AddTool(server, destructiveTool("socialhub_delete_comment", "Permanently delete one comment from the explicitly selected account. Requires user confirmation."), s.deleteComment)
	}
	if s.policy.allows(OperationSendMessage) {
		mcp.AddTool(server, writeTool("socialhub_send_message", "Send one message from the explicitly selected account. Requires user confirmation."), s.sendMessage)
	}
}

func readTool(name, description string) *mcp.Tool {
	openWorld := true
	return &mcp.Tool{
		Name: name, Description: description,
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: &openWorld},
	}
}

func writeTool(name, description string) *mcp.Tool {
	destructive, openWorld := false, true
	return &mcp.Tool{
		Name: name, Description: description,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, OpenWorldHint: &openWorld},
	}
}

func destructiveTool(name, description string) *mcp.Tool {
	destructive, openWorld := true, true
	return &mcp.Tool{
		Name: name, Description: description,
		Annotations: &mcp.ToolAnnotations{DestructiveHint: &destructive, IdempotentHint: true, OpenWorldHint: &openWorld},
	}
}

func success[T any](data T) (*mcp.CallToolResult, Result[T], error) {
	return nil, Result[T]{Data: &data}, nil
}

func failure[T any](err error) (*mcp.CallToolResult, Result[T], error) {
	return &mcp.CallToolResult{IsError: true}, Result[T]{Error: sanitizeError(err)}, nil
}

func (s *Service) listTargets(context.Context, *mcp.CallToolRequest, emptyInput) (*mcp.CallToolResult, Result[TargetsData], error) {
	return success(TargetsData{Targets: s.targetList()})
}

func (s *Service) getCapabilities(ctx context.Context, _ *mcp.CallToolRequest, input targetInput) (*mcp.CallToolResult, Result[CapabilitiesData], error) {
	client, err := s.client(ctx, input.Target)
	if err != nil {
		return failure[CapabilitiesData](err)
	}
	capabilities, err := client.Capabilities(ctx)
	if err != nil {
		return failure[CapabilitiesData](err)
	}
	names := make([]string, 0, len(capabilities))
	for name := range capabilities {
		names = append(names, string(name))
	}
	sort.Strings(names)
	items := make([]CapabilityInfo, 0, len(names))
	for _, name := range names {
		state := capabilities[socialhub.Capability(name)]
		items = append(items, CapabilityInfo{
			Name: socialhub.Capability(name), Supported: state.Supported, Approval: state.Approval,
			Scopes: append([]string(nil), state.Scopes...), Reason: state.Reason, DocURL: state.DocURL,
		})
	}
	return success(CapabilitiesData{Target: input.Target, Platform: client.Platform(), Capabilities: items})
}

func (s *Service) getUser(ctx context.Context, _ *mcp.CallToolRequest, input getUserInput) (*mcp.CallToolResult, Result[UserData], error) {
	if strings.TrimSpace(input.UserID) == "" {
		return failure[UserData](invalidArgument("get_user.user_id", "user_id is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[UserData](err)
	}
	defer cancel()
	fetcher, ok := client.Fetcher()
	if !ok {
		return failure[UserData](socialhub.UnsupportedError(client.Platform(), socialhub.CapFetch))
	}
	user, err := fetcher.GetUser(callCtx, input.UserID, options...)
	if err != nil {
		return failure[UserData](err)
	}
	return success(UserData{User: user})
}

func (s *Service) getPost(ctx context.Context, _ *mcp.CallToolRequest, input getPostInput) (*mcp.CallToolResult, Result[PostData], error) {
	if strings.TrimSpace(input.PostID) == "" {
		return failure[PostData](invalidArgument("get_post.post_id", "post_id is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[PostData](err)
	}
	defer cancel()
	fetcher, ok := client.Fetcher()
	if !ok {
		return failure[PostData](socialhub.UnsupportedError(client.Platform(), socialhub.CapFetch))
	}
	post, err := fetcher.GetPost(callCtx, input.PostID, options...)
	if err != nil {
		return failure[PostData](err)
	}
	return success(PostData{Post: post})
}

func (s *Service) listPosts(ctx context.Context, _ *mcp.CallToolRequest, input listPostsInput) (*mcp.CallToolResult, Result[PostsData], error) {
	if err := validatePage(input.MaxResults, input.StartTime, input.EndTime); err != nil {
		return failure[PostsData](err)
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[PostsData](err)
	}
	defer cancel()
	fetcher, ok := client.Fetcher()
	if !ok {
		return failure[PostsData](socialhub.UnsupportedError(client.Platform(), socialhub.CapFetch))
	}
	page, err := fetcher.ListPosts(callCtx, socialhub.ListPostsRequest{
		UserID: input.UserID, Cursor: input.Cursor, MaxResults: input.MaxResults,
		StartTime: input.StartTime, EndTime: input.EndTime,
	}, options...)
	if err != nil {
		return failure[PostsData](err)
	}
	return success(PostsData{Page: page})
}

func (s *Service) listComments(ctx context.Context, _ *mcp.CallToolRequest, input listCommentsInput) (*mcp.CallToolResult, Result[CommentsData], error) {
	if strings.TrimSpace(input.PostID) == "" {
		return failure[CommentsData](invalidArgument("list_comments.post_id", "post_id is required"))
	}
	if err := validatePage(input.MaxResults, nil, nil); err != nil {
		return failure[CommentsData](err)
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[CommentsData](err)
	}
	defer cancel()
	fetcher, ok := client.Fetcher()
	if !ok {
		return failure[CommentsData](socialhub.UnsupportedError(client.Platform(), socialhub.CapFetch))
	}
	page, err := fetcher.ListComments(callCtx, socialhub.ListCommentsRequest{
		PostID: input.PostID, Cursor: input.Cursor, MaxResults: input.MaxResults,
	}, options...)
	if err != nil {
		return failure[CommentsData](err)
	}
	return success(CommentsData{Page: page})
}

func (s *Service) getMessage(ctx context.Context, _ *mcp.CallToolRequest, input getMessageInput) (*mcp.CallToolResult, Result[MessageData], error) {
	if strings.TrimSpace(input.MessageID) == "" {
		return failure[MessageData](invalidArgument("get_message.message_id", "message_id is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[MessageData](err)
	}
	defer cancel()
	messenger, ok := client.Messenger()
	if !ok {
		return failure[MessageData](socialhub.UnsupportedError(client.Platform(), socialhub.CapMessage))
	}
	message, err := messenger.GetMessage(callCtx, input.MessageID, options...)
	if err != nil {
		return failure[MessageData](err)
	}
	return success(MessageData{Message: message})
}

func (s *Service) getPublishStatus(ctx context.Context, _ *mcp.CallToolRequest, input publishStatusInput) (*mcp.CallToolResult, Result[PublishStatusData], error) {
	if strings.TrimSpace(input.PublishID) == "" {
		return failure[PublishStatusData](invalidArgument("get_publish_status.publish_id", "publish_id is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[PublishStatusData](err)
	}
	defer cancel()
	publisher, ok := client.Publisher()
	if !ok {
		return failure[PublishStatusData](socialhub.UnsupportedError(client.Platform(), socialhub.CapPublish))
	}
	status, err := publisher.PublishStatus(callCtx, input.PublishID, options...)
	if err != nil {
		return failure[PublishStatusData](err)
	}
	return success(PublishStatusData{Status: status})
}

func (s *Service) publishPost(ctx context.Context, _ *mcp.CallToolRequest, input publishPostInput) (*mcp.CallToolResult, Result[PostData], error) {
	request := socialhub.CreatePostRequest{
		Text: input.Text, MediaIDs: input.MediaIDs, ReplyToID: input.ReplyToID,
		QuotePostID: input.QuotePostID, Visibility: input.Visibility,
	}
	if err := request.Validate(); err != nil {
		return failure[PostData](invalidArgument("publish_post", "text or media_ids is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[PostData](err)
	}
	defer cancel()
	publisher, ok := client.Publisher()
	if !ok {
		return failure[PostData](socialhub.UnsupportedError(client.Platform(), socialhub.CapPublish))
	}
	post, err := publisher.Publish(callCtx, request, options...)
	if err != nil {
		return failure[PostData](err)
	}
	return success(PostData{Post: post})
}

func (s *Service) deletePost(ctx context.Context, _ *mcp.CallToolRequest, input deletePostInput) (*mcp.CallToolResult, Result[MutationData], error) {
	if strings.TrimSpace(input.PostID) == "" {
		return failure[MutationData](invalidArgument("delete_post.post_id", "post_id is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[MutationData](err)
	}
	defer cancel()
	publisher, ok := client.Publisher()
	if !ok {
		return failure[MutationData](socialhub.UnsupportedError(client.Platform(), socialhub.CapPublish))
	}
	if err := publisher.DeletePost(callCtx, input.PostID, options...); err != nil {
		return failure[MutationData](err)
	}
	return success(MutationData{Success: true})
}

func (s *Service) addReaction(ctx context.Context, _ *mcp.CallToolRequest, input reactionInput) (*mcp.CallToolResult, Result[MutationData], error) {
	return s.changeReaction(ctx, input, false)
}

func (s *Service) removeReaction(ctx context.Context, _ *mcp.CallToolRequest, input reactionInput) (*mcp.CallToolResult, Result[MutationData], error) {
	return s.changeReaction(ctx, input, true)
}

func (s *Service) changeReaction(ctx context.Context, input reactionInput, remove bool) (*mcp.CallToolResult, Result[MutationData], error) {
	if strings.TrimSpace(input.ActorID) == "" || strings.TrimSpace(input.TargetID) == "" {
		return failure[MutationData](invalidArgument("reaction.target", "actor_id and target_id are required"))
	}
	if input.Kind != socialhub.ReactionLike && input.Kind != socialhub.ReactionRepost {
		return failure[MutationData](invalidArgument("reaction.kind", "kind must be like or repost"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[MutationData](err)
	}
	defer cancel()
	reactor, ok := client.Reactor()
	if !ok {
		return failure[MutationData](socialhub.UnsupportedError(client.Platform(), socialhub.CapReact))
	}
	request := socialhub.ReactionRequest{ActorID: input.ActorID, TargetID: input.TargetID, Kind: input.Kind}
	if remove {
		err = reactor.RemoveReaction(callCtx, request, options...)
	} else {
		err = reactor.React(callCtx, request, options...)
	}
	if err != nil {
		return failure[MutationData](err)
	}
	return success(MutationData{Success: true})
}

func (s *Service) createComment(ctx context.Context, _ *mcp.CallToolRequest, input createCommentInput) (*mcp.CallToolResult, Result[CommentData], error) {
	if strings.TrimSpace(input.PostID) == "" || strings.TrimSpace(input.Text) == "" {
		return failure[CommentData](invalidArgument("create_comment", "post_id and text are required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[CommentData](err)
	}
	defer cancel()
	reactor, ok := client.Reactor()
	if !ok {
		return failure[CommentData](socialhub.UnsupportedError(client.Platform(), socialhub.CapReact))
	}
	comment, err := reactor.Comment(callCtx, socialhub.CreateCommentRequest{
		PostID: input.PostID, ParentID: input.ParentID, Text: input.Text,
	}, options...)
	if err != nil {
		return failure[CommentData](err)
	}
	return success(CommentData{Comment: comment})
}

func (s *Service) deleteComment(ctx context.Context, _ *mcp.CallToolRequest, input deleteCommentInput) (*mcp.CallToolResult, Result[MutationData], error) {
	if strings.TrimSpace(input.CommentID) == "" {
		return failure[MutationData](invalidArgument("delete_comment.comment_id", "comment_id is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[MutationData](err)
	}
	defer cancel()
	reactor, ok := client.Reactor()
	if !ok {
		return failure[MutationData](socialhub.UnsupportedError(client.Platform(), socialhub.CapReact))
	}
	if err := reactor.DeleteComment(callCtx, input.CommentID, options...); err != nil {
		return failure[MutationData](err)
	}
	return success(MutationData{Success: true})
}

func (s *Service) sendMessage(ctx context.Context, _ *mcp.CallToolRequest, input sendMessageInput) (*mcp.CallToolResult, Result[MessageData], error) {
	if (input.Text == nil || strings.TrimSpace(*input.Text) == "") && len(input.MediaIDs) == 0 {
		return failure[MessageData](invalidArgument("send_message.content", "text or media_ids is required"))
	}
	client, callCtx, cancel, options, err := s.prepareCall(ctx, input.Target, input.Call)
	if err != nil {
		return failure[MessageData](err)
	}
	defer cancel()
	messenger, ok := client.Messenger()
	if !ok {
		return failure[MessageData](socialhub.UnsupportedError(client.Platform(), socialhub.CapMessage))
	}
	message, err := messenger.SendMessage(callCtx, socialhub.SendMessageRequest{
		ConversationID: input.ConversationID, RecipientIDs: input.RecipientIDs,
		Text: input.Text, MediaIDs: input.MediaIDs, ReplyToID: input.ReplyToID,
	}, options...)
	if err != nil {
		return failure[MessageData](err)
	}
	return success(MessageData{Message: message})
}

func (s *Service) prepareCall(ctx context.Context, target TargetRef, control CallControl) (socialhub.Client, context.Context, context.CancelFunc, []socialhub.CallOption, error) {
	callCtx, cancel, options, err := callContext(ctx, control)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	client, err := s.client(callCtx, target)
	if err != nil {
		cancel()
		return nil, nil, nil, nil, err
	}
	return client, callCtx, cancel, options, nil
}

func validatePage(maxResults int, startTime, endTime *time.Time) error {
	if maxResults < 0 {
		return invalidArgument("pagination.max_results", "max_results must not be negative")
	}
	if startTime != nil && endTime != nil && startTime.After(*endTime) {
		return invalidArgument("pagination.time_range", "start_time must not be after end_time")
	}
	return nil
}
