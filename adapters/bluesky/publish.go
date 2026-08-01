package bluesky

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (c *Client) Publish(ctx context.Context, input socialhub.CreatePostRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := input.Validate(); err != nil {
		return nil, platformError("publish", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	if input.Visibility != nil && *input.Visibility != "public" {
		return nil, invalidArgument("publish", "Bluesky repository posts are public")
	}
	request := PostRecordRequest{Media: make([]PostMedia, 0, len(input.MediaIDs))}
	if input.Text != nil {
		request.Text = *input.Text
	}
	for _, mediaID := range input.MediaIDs {
		request.Media = append(request.Media, PostMedia{MediaID: mediaID})
	}
	if input.ReplyToID != nil {
		request.ReplyToURI = *input.ReplyToID
	}
	if input.QuotePostID != nil {
		request.QuoteURI = *input.QuotePostID
	}
	return c.CreateRecord(ctx, request, options...)
}

func (c *Client) CreateRecord(ctx context.Context, input PostRecordRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if err := validatePostRecord(input); err != nil {
		return nil, err
	}
	mediaEmbed, media, err := c.buildMediaEmbed(input.Media)
	if err != nil {
		return nil, err
	}
	var reply *bskyReplyRef
	if input.ReplyToURI != "" {
		resolved, err := c.resolveReply(ctx, input.ReplyToURI, options...)
		if err != nil {
			return nil, err
		}
		reply = &resolved
	}
	var quoteRef *strongRef
	if input.QuoteURI != "" {
		resolved, err := c.resolveStrongRef(ctx, input.QuoteURI, options...)
		if err != nil {
			return nil, err
		}
		quoteRef = &resolved
	}
	embed := combineEmbeds(mediaEmbed, quoteRef)
	createdAt := c.clock.Now().UTC()
	record := postRecordInput{
		Type: collectionPost, Text: input.Text, CreatedAt: createdAt.Format(time.RFC3339Nano),
		Languages: append([]string(nil), input.Languages...), Reply: reply, Embed: embed,
	}
	response, err := c.createRepositoryRecord(ctx, collectionPost, input.RecordKey, record, options...)
	if err != nil {
		return nil, err
	}
	encodedRecord, _ := json.Marshal(record)
	extension, _ := json.Marshal(struct {
		CID              string          `json:"cid"`
		ValidationStatus string          `json:"validation_status,omitempty"`
		Record           json.RawMessage `json:"record"`
	}{response.CID, response.ValidationStatus, encodedRecord})
	post := &socialhub.Post{
		Platform: "bluesky", AccountID: c.accountID, ID: response.URI, AuthorID: stringPointer(c.repo), Text: stringPointer(input.Text),
		Media: media, CreatedAt: &createdAt, Visibility: stringPointer("public"),
		Status:     &socialhub.PublishStatus{ID: response.URI, State: socialhub.PublishStatePublished, UpdatedAt: &createdAt},
		Extensions: map[string]json.RawMessage{"bluesky.post": extension},
	}
	if reply != nil {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: reply.Parent.URI})
	}
	if quoteRef != nil {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationQuote, PostID: quoteRef.URI})
	}
	return post, nil
}

func validatePostRecord(input PostRecordRequest) error {
	if input.Text == "" && len(input.Media) == 0 && input.QuoteURI == "" {
		return invalidArgument("create_record", "post requires text, media, or a quoted record")
	}
	if len(input.Languages) > 3 {
		return invalidArgument("create_record", "at most three post languages are allowed")
	}
	for _, language := range input.Languages {
		if strings.TrimSpace(language) == "" || strings.ContainsAny(language, " \t\r\n") {
			return invalidArgument("create_record", "post languages must be non-empty language tags")
		}
	}
	if len(input.Media) > 4 {
		return invalidArgument("create_record", "at most four image blobs are allowed")
	}
	for _, media := range input.Media {
		if strings.TrimSpace(media.MediaID) == "" {
			return invalidArgument("create_record", "media IDs must not be empty")
		}
		if (media.Width == 0) != (media.Height == 0) || media.Width < 0 || media.Height < 0 {
			return invalidArgument("create_record", "media width and height must both be positive or both be omitted")
		}
	}
	if input.RecordKey != "" && !validRecordKey(input.RecordKey) {
		return invalidArgument("create_record", "record key is invalid")
	}
	if input.ReplyToURI != "" {
		parsed, err := parseRecordURI(input.ReplyToURI)
		if err != nil || parsed.Collection != collectionPost {
			return invalidArgument("create_record", "reply target must be a post AT URI")
		}
	}
	if input.QuoteURI != "" {
		parsed, err := parseRecordURI(input.QuoteURI)
		if err != nil || parsed.Collection != collectionPost {
			return invalidArgument("create_record", "quote target must be a post AT URI")
		}
	}
	return nil
}

func (c *Client) buildMediaEmbed(inputs []PostMedia) (any, []socialhub.Media, error) {
	if len(inputs) == 0 {
		return nil, nil, nil
	}
	c.uploadMu.Lock()
	blobs := make([]blobRef, len(inputs))
	for index, input := range inputs {
		blob, ok := c.blobs[input.MediaID]
		if !ok {
			c.uploadMu.Unlock()
			return nil, nil, invalidArgument("create_record", "media IDs must come from completed uploads on this client")
		}
		blobs[index] = blob
	}
	c.uploadMu.Unlock()
	media := make([]socialhub.Media, 0, len(inputs))
	videoCount := 0
	for index, input := range inputs {
		mapped := mapUploadedBlob(blobs[index], input)
		media = append(media, mapped)
		if mapped.Type == socialhub.MediaTypeVideo {
			videoCount++
		}
	}
	if videoCount > 0 {
		if len(inputs) != 1 || videoCount != 1 {
			return nil, nil, invalidArgument("create_record", "a Bluesky video post requires exactly one video and no images")
		}
		input := inputs[0]
		return videoEmbed{
			Type: "app.bsky.embed.video", Video: blobs[0], Alt: input.Alt, AspectRatio: newAspectRatio(input.Width, input.Height),
		}, media, nil
	}
	images := make([]imageInput, 0, len(inputs))
	for index, input := range inputs {
		images = append(images, imageInput{Alt: input.Alt, Image: blobs[index], AspectRatio: newAspectRatio(input.Width, input.Height)})
	}
	return imageEmbed{Type: "app.bsky.embed.images", Images: images}, media, nil
}

func newAspectRatio(width, height int) *aspectRatio {
	if width <= 0 || height <= 0 {
		return nil
	}
	return &aspectRatio{Width: width, Height: height}
}

func combineEmbeds(media any, quote *strongRef) any {
	if quote == nil {
		return media
	}
	record := recordEmbed{Type: "app.bsky.embed.record", Record: *quote}
	if media == nil {
		return record
	}
	return recordWithMediaEmbed{Type: "app.bsky.embed.recordWithMedia", Record: record, Media: media}
}

func (c *Client) resolveStrongRef(ctx context.Context, uri string, options ...socialhub.CallOption) (strongRef, error) {
	view, err := c.getPostView(ctx, uri, options...)
	if err != nil {
		return strongRef{}, err
	}
	return strongRef{URI: view.URI, CID: view.CID}, nil
}

func (c *Client) resolveReply(ctx context.Context, uri string, options ...socialhub.CallOption) (bskyReplyRef, error) {
	view, err := c.getPostView(ctx, uri, options...)
	if err != nil {
		return bskyReplyRef{}, err
	}
	parent := strongRef{URI: view.URI, CID: view.CID}
	root := parent
	var record bskyPostRecord
	if json.Unmarshal(view.Record, &record) == nil && record.Reply != nil && record.Reply.Root.URI != "" && record.Reply.Root.CID != "" {
		root = record.Reply.Root
	}
	return bskyReplyRef{Root: root, Parent: parent}, nil
}

func (c *Client) createRepositoryRecord(ctx context.Context, collection, recordKey string, record any, options ...socialhub.CallOption) (*createRecordResponse, error) {
	request := createRecordRequest{Repo: c.repo, Collection: collection, RecordKey: recordKey, Record: record}
	var response createRecordResponse
	if err := c.post(ctx, "com.atproto.repo.createRecord", request, &response, options...); err != nil {
		return nil, err
	}
	parsed, err := parseRecordURI(response.URI)
	if err != nil || parsed.Repo != c.repo || parsed.Collection != collection || response.CID == "" {
		return nil, platformError("create_record", socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	return &response, nil
}

func (c *Client) PublishStatus(ctx context.Context, postID string, options ...socialhub.CallOption) (*socialhub.PublishStatus, error) {
	post, err := c.GetPost(ctx, postID, options...)
	if err != nil {
		return nil, err
	}
	return &socialhub.PublishStatus{ID: post.ID, State: socialhub.PublishStatePublished, UpdatedAt: post.CreatedAt}, nil
}

func (c *Client) DeletePost(ctx context.Context, postID string, options ...socialhub.CallOption) error {
	return c.deleteRecord(ctx, postID, collectionPost, options...)
}

func (c *Client) deleteRecord(ctx context.Context, uri, collection string, options ...socialhub.CallOption) error {
	parsed, err := parseRecordURI(uri)
	if err != nil || parsed.Repo != c.repo || parsed.Collection != collection {
		return invalidArgument("delete_record", "record URI must belong to the configured repo and expected collection")
	}
	return c.post(ctx, "com.atproto.repo.deleteRecord", deleteRecordRequest{
		Repo: c.repo, Collection: collection, RecordKey: parsed.RecordKey,
	}, nil, options...)
}

func (c *Client) React(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, false, options...)
}

func (c *Client) RemoveReaction(ctx context.Context, input socialhub.ReactionRequest, options ...socialhub.CallOption) error {
	return c.setReaction(ctx, input, true, options...)
}

func (c *Client) setReaction(ctx context.Context, input socialhub.ReactionRequest, remove bool, options ...socialhub.CallOption) error {
	if input.ActorID != "" && input.ActorID != c.repo {
		return invalidArgument("react", "actor must be the configured Bluesky repo DID")
	}
	parsed, err := parseRecordURI(input.TargetID)
	if err != nil || parsed.Collection != collectionPost {
		return invalidArgument("react", "target must be a post AT URI")
	}
	collection := ""
	switch input.Kind {
	case socialhub.ReactionLike:
		collection = collectionLike
	case socialhub.ReactionRepost:
		collection = collectionRepost
	default:
		return invalidArgument("react", "reaction must be LIKE or REPOST")
	}
	view, err := c.getPostView(ctx, input.TargetID, options...)
	if err != nil {
		return err
	}
	reactionURI := view.Viewer.Like
	if input.Kind == socialhub.ReactionRepost {
		reactionURI = view.Viewer.Repost
	}
	if remove {
		if reactionURI == "" {
			return nil
		}
		return c.deleteRecord(ctx, reactionURI, collection, options...)
	}
	if reactionURI != "" {
		return nil
	}
	record := struct {
		Type      string    `json:"$type"`
		Subject   strongRef `json:"subject"`
		CreatedAt string    `json:"createdAt"`
	}{collection, strongRef{URI: view.URI, CID: view.CID}, c.clock.Now().UTC().Format(time.RFC3339Nano)}
	_, err = c.createRepositoryRecord(ctx, collection, "", record, options...)
	return err
}

func (c *Client) Comment(ctx context.Context, input socialhub.CreateCommentRequest, options ...socialhub.CallOption) (*socialhub.Comment, error) {
	if strings.TrimSpace(input.PostID) == "" || strings.TrimSpace(input.Text) == "" {
		return nil, invalidArgument("comment", "post ID and text are required")
	}
	target := input.PostID
	if input.ParentID != nil {
		if strings.TrimSpace(*input.ParentID) == "" {
			return nil, invalidArgument("comment", "parent ID must not be empty")
		}
		target = *input.ParentID
	}
	post, err := c.CreateRecord(ctx, PostRecordRequest{Text: input.Text, ReplyToURI: target}, options...)
	if err != nil {
		return nil, err
	}
	return &socialhub.Comment{
		Platform: "bluesky", AccountID: c.accountID, ID: post.ID, PostID: input.PostID,
		AuthorID: post.AuthorID, ParentID: stringPointer(target), Text: input.Text, CreatedAt: post.CreatedAt,
	}, nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID string, options ...socialhub.CallOption) error {
	return c.DeletePost(ctx, commentID, options...)
}

func mapUploadedBlob(blob blobRef, input PostMedia) socialhub.Media {
	mediaType := socialhub.MediaTypeImage
	if blob.MIMEType == "video/mp4" {
		mediaType = socialhub.MediaTypeVideo
	} else if blob.MIMEType == "image/gif" {
		mediaType = socialhub.MediaTypeAnimation
	}
	extension, _ := json.Marshal(struct {
		Blob        blobRef      `json:"blob"`
		Alt         string       `json:"alt,omitempty"`
		AspectRatio *aspectRatio `json:"aspect_ratio,omitempty"`
	}{blob, input.Alt, newAspectRatio(input.Width, input.Height)})
	size := blob.Size
	return socialhub.Media{
		ID: blob.Ref.Link, MIME: blob.MIMEType, Type: mediaType, Size: &size,
		Width: intPointer(input.Width), Height: intPointer(input.Height), State: socialhub.MediaStateReady,
		Extensions: map[string]json.RawMessage{"bluesky.blob": extension},
	}
}
