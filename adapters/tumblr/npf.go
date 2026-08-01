package tumblr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"social-hub/internal/transport"
	"social-hub/pkg/socialhub"
)

const maxNPFJSONBytes = 1_000_000

type npfPayload struct {
	Content               []json.RawMessage `json:"content"`
	Layout                []json.RawMessage `json:"layout,omitempty"`
	State                 NPFState          `json:"state,omitempty"`
	PublishOn             string            `json:"publish_on,omitempty"`
	Date                  string            `json:"date,omitempty"`
	Tags                  string            `json:"tags,omitempty"`
	SourceURL             string            `json:"source_url,omitempty"`
	SendToTwitter         *bool             `json:"send_to_twitter,omitempty"`
	IsPrivate             bool              `json:"is_private,omitempty"`
	Slug                  string            `json:"slug,omitempty"`
	InteractabilityReblog string            `json:"interactability_reblog,omitempty"`
	ParentTumblelogUUID   string            `json:"parent_tumblelog_uuid,omitempty"`
	ParentPostID          string            `json:"parent_post_id,omitempty"`
	ReblogKey             string            `json:"reblog_key,omitempty"`
	HideTrail             bool              `json:"hide_trail,omitempty"`
	ExcludeTrailItems     []int             `json:"exclude_trail_items,omitempty"`
}

type uploadWriteResult struct {
	err      error
	mismatch bool
}

func (c *Client) CreateNPF(ctx context.Context, blog string, input NPFPostRequest, options ...socialhub.CallOption) (*NPFResult, error) {
	selected, err := c.selectedBlog(blog)
	if err != nil {
		return nil, err
	}
	return c.writeNPF(ctx, http.MethodPost, "/blog/"+url.PathEscape(selected)+"/posts", "create_npf", input, false, options...)
}

func (c *Client) EditNPF(ctx context.Context, blog, postID string, input NPFPostRequest, options ...socialhub.CallOption) (*NPFResult, error) {
	if !validPostID(postID) {
		return nil, invalidArgument("edit_npf", "numeric post ID is required")
	}
	selected, err := c.selectedBlog(blog)
	if err != nil {
		return nil, err
	}
	return c.writeNPF(ctx, http.MethodPut, "/blog/"+url.PathEscape(selected)+"/posts/"+url.PathEscape(postID), "edit_npf", input, true, options...)
}

func (c *Client) GetNPF(ctx context.Context, blog, postID string, options ...socialhub.CallOption) (*NPFPost, error) {
	if !validPostID(postID) {
		return nil, invalidArgument("get_npf", "numeric post ID is required")
	}
	selected, err := c.selectedBlog(blog)
	if err != nil {
		return nil, err
	}
	user, err := c.requireUser("get_npf")
	if err != nil {
		return nil, err
	}
	if err := c.requireScopes("get_npf", "basic"); err != nil {
		return nil, err
	}
	var response tumblrPost
	query := url.Values{"post_format": {"npf"}}
	if err := c.request(ctx, user, http.MethodGet, "/blog/"+url.PathEscape(selected)+"/posts/"+url.PathEscape(postID), query, nil, &response, options...); err != nil {
		return nil, err
	}
	if response.identifier() != postID {
		return nil, platformError("get_npf", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapNPFPost(response), nil
}

func (c *Client) writeNPF(ctx context.Context, method, path, operation string, input NPFPostRequest, editing bool, options ...socialhub.CallOption) (*NPFResult, error) {
	user, err := c.requireUser(operation)
	if err != nil {
		return nil, err
	}
	if err := c.requireScopes(operation, "write"); err != nil {
		return nil, err
	}
	payload, uploads, err := c.prepareNPF(input, editing)
	if err != nil {
		return nil, err
	}
	var response tumblrIDResponse
	if len(uploads) == 0 {
		err = c.request(ctx, user, method, path, nil, payload, &response, options...)
	} else {
		err = c.multipartNPF(ctx, user, method, path, payload, uploads, &response, options...)
	}
	if err != nil {
		return nil, err
	}
	id := string(response.ID)
	if !validPostID(id) {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	state := payload.State
	if state == "" {
		state = NPFPublished
	}
	return &NPFResult{ID: id, State: state}, nil
}

func (c *Client) prepareNPF(input NPFPostRequest, editing bool) (npfPayload, []NPFMediaUpload, error) {
	if len(input.Content) > 1000 {
		return npfPayload{}, nil, invalidArgument("npf", "content must not exceed 1000 blocks")
	}
	if len(input.Content) == 0 && (input.Reblog == nil || editing) {
		return npfPayload{}, nil, invalidArgument("npf", "content is required for original and edited posts")
	}
	state := input.State
	if state == "" {
		state = NPFPublished
	}
	if !validNPFState(state) {
		return npfPayload{}, nil, invalidArgument("npf", "post state is invalid")
	}
	now := c.clock.Now()
	if input.PublishOn != nil && (state != NPFQueue || !input.PublishOn.After(now)) {
		return npfPayload{}, nil, invalidArgument("npf", "publish_on requires queue state and a future time")
	}
	if input.Date != nil && input.Date.After(now) {
		return npfPayload{}, nil, invalidArgument("npf", "backdate must not be in the future")
	}
	if input.SourceURL != "" && !validWebURL(input.SourceURL) {
		return npfPayload{}, nil, invalidArgument("npf", "source URL must be an absolute HTTP(S) URL without credentials")
	}
	if input.InteractabilityReblog != "" && input.InteractabilityReblog != "everyone" && input.InteractabilityReblog != "noone" {
		return npfPayload{}, nil, invalidArgument("npf", "interactability_reblog must be everyone or noone")
	}
	content := cloneRawMessages(input.Content)
	blockTypes := make([]string, len(content))
	counts := map[string]int{}
	for index, block := range content {
		blockType, text, err := validateContentBlock(block)
		if err != nil {
			return npfPayload{}, nil, invalidArgument("npf", fmt.Sprintf("content block %d is invalid", index))
		}
		blockTypes[index] = blockType
		counts[blockType]++
		if blockType == "text" && utf8.RuneCountInString(text) > 4096 {
			return npfPayload{}, nil, invalidArgument("npf", "text blocks must not exceed 4096 Unicode code points")
		}
	}
	if counts["image"] > 30 || counts["video"] > 10 || counts["audio"] > 10 || counts["link"] > 10 {
		return npfPayload{}, nil, invalidArgument("npf", "content exceeds Tumblr's media or link block limits")
	}
	for index, layout := range input.Layout {
		if !jsonObject(layout) {
			return npfPayload{}, nil, invalidArgument("npf", fmt.Sprintf("layout block %d must be a JSON object", index))
		}
	}
	uploads := append([]NPFMediaUpload(nil), input.Uploads...)
	usedBlocks := make(map[int]struct{}, len(uploads))
	nativeVideos := 0
	for index, upload := range uploads {
		if upload.BlockIndex < 0 || upload.BlockIndex >= len(content) || upload.Reader == nil || strings.TrimSpace(upload.Filename) == "" || strings.TrimSpace(upload.MIME) == "" || upload.Size <= 0 {
			return npfPayload{}, nil, invalidArgument("npf", fmt.Sprintf("upload %d is incomplete", index))
		}
		if _, exists := usedBlocks[upload.BlockIndex]; exists {
			return npfPayload{}, nil, invalidArgument("npf", "only one upload may target each content block")
		}
		usedBlocks[upload.BlockIndex] = struct{}{}
		switch blockTypes[upload.BlockIndex] {
		case "image", "audio":
		case "video":
			nativeVideos++
		default:
			return npfPayload{}, nil, invalidArgument("npf", "uploads require image, audio, or video blocks")
		}
		identifier := strconv.Itoa(upload.BlockIndex)
		patched, err := withMediaIdentifier(content[upload.BlockIndex], identifier, strings.TrimSpace(upload.MIME))
		if err != nil {
			return npfPayload{}, nil, invalidArgument("npf", "unable to bind upload to content block")
		}
		content[upload.BlockIndex] = patched
		uploads[index].MIME = strings.TrimSpace(upload.MIME)
		uploads[index].Filename = strings.TrimSpace(upload.Filename)
	}
	if nativeVideos > 1 {
		return npfPayload{}, nil, invalidArgument("npf", "only one native video upload is allowed per post")
	}
	payload := npfPayload{
		Content: content, Layout: cloneRawMessages(input.Layout), State: state, Tags: strings.Join(input.Tags, ","),
		SourceURL: input.SourceURL, SendToTwitter: input.SendToTwitter, IsPrivate: input.IsPrivate, Slug: input.Slug,
		InteractabilityReblog: input.InteractabilityReblog,
	}
	if input.PublishOn != nil {
		payload.PublishOn = input.PublishOn.UTC().Format(time.RFC3339)
	}
	if input.Date != nil {
		payload.Date = input.Date.UTC().Format(time.RFC3339)
	}
	if input.Reblog != nil {
		if !validReblogTarget(*input.Reblog) {
			return npfPayload{}, nil, invalidArgument("npf", "reblog target is incomplete or invalid")
		}
		payload.ParentTumblelogUUID = input.Reblog.BlogUUID
		payload.ParentPostID = input.Reblog.PostID
		payload.ReblogKey = input.Reblog.ReblogKey
		payload.HideTrail = input.Reblog.HideTrail
		payload.ExcludeTrailItems = append([]int(nil), input.Reblog.ExcludeTrailItems...)
	}
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) > maxNPFJSONBytes {
		return npfPayload{}, nil, invalidArgument("npf", "NPF JSON exceeds Tumblr's 1 MB stored-content limit")
	}
	return payload, uploads, nil
}

func (c *Client) multipartNPF(ctx context.Context, user *transport.Client, method, path string, payload npfPayload, uploads []NPFMediaUpload, output any, options ...socialhub.CallOption) error {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return platformError("npf_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	pipeReader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	contentType := writer.FormDataContentType()
	done := make(chan uploadWriteResult, 1)
	go func() {
		result := uploadWriteResult{}
		jsonHeader := make(textproto.MIMEHeader)
		jsonHeader.Set("Content-Disposition", `form-data; name="json"`)
		jsonHeader.Set("Content-Type", "application/json")
		part, err := writer.CreatePart(jsonHeader)
		if err == nil {
			_, err = part.Write(encoded)
		}
		result.err = err
		for _, upload := range uploads {
			if result.err != nil {
				break
			}
			header := make(textproto.MIMEHeader)
			header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%d"; filename="%s"`, upload.BlockIndex, escapeQuotes(upload.Filename)))
			header.Set("Content-Type", upload.MIME)
			part, result.err = writer.CreatePart(header)
			if result.err != nil {
				break
			}
			var copied int64
			copied, result.err = io.Copy(part, io.LimitReader(upload.Reader, upload.Size+1))
			if result.err == nil && copied != upload.Size {
				result.err, result.mismatch = fmt.Errorf("declared upload size does not match stream"), true
			}
		}
		if closeErr := writer.Close(); result.err == nil {
			result.err = closeErr
		}
		_ = pipeWriter.CloseWithError(result.err)
		done <- result
	}()
	request, err := user.NewRequest(ctx, method, path, nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.CloseWithError(err)
		<-done
		return err
	}
	request.Header.Set("Content-Type", contentType)
	var envelope tumblrEnvelope
	httpErr := user.Do(request, &envelope)
	_ = pipeReader.CloseWithError(httpErr)
	writeResult := <-done
	if writeResult.mismatch {
		return invalidArgument("npf_upload", "uploaded byte count does not match declared size")
	}
	if httpErr != nil {
		return httpErr
	}
	if writeResult.err != nil {
		return platformError("npf_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeResult.err)
	}
	return decodeEnvelope(envelope, output)
}

func validateContentBlock(raw json.RawMessage) (string, string, error) {
	if !jsonObject(raw) {
		return "", "", fmt.Errorf("block must be object")
	}
	var block struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &block); err != nil || !validToken(block.Type) {
		return "", "", fmt.Errorf("block type is invalid")
	}
	return block.Type, block.Text, nil
}

func withMediaIdentifier(raw json.RawMessage, identifier, mimeType string) (json.RawMessage, error) {
	var block map[string]json.RawMessage
	if err := json.Unmarshal(raw, &block); err != nil {
		return nil, err
	}
	media, err := json.Marshal(map[string]string{"identifier": identifier, "type": mimeType})
	if err != nil {
		return nil, err
	}
	block["media"] = media
	return json.Marshal(block)
}

func jsonObject(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return len(trimmed) >= 2 && trimmed[0] == '{' && json.Valid(raw)
}

func validNPFState(value NPFState) bool {
	switch value {
	case NPFPublished, NPFQueue, NPFDraft, NPFPrivate, NPFUnapproved:
		return true
	default:
		return false
	}
}

func validReblogTarget(target NPFReblogTarget) bool {
	if strings.TrimSpace(target.BlogUUID) == "" || !validPostID(target.PostID) || strings.TrimSpace(target.ReblogKey) == "" || (target.HideTrail && len(target.ExcludeTrailItems) > 0) {
		return false
	}
	seen := make(map[int]struct{}, len(target.ExcludeTrailItems))
	for _, index := range target.ExcludeTrailItems {
		if index < 0 {
			return false
		}
		if _, exists := seen[index]; exists {
			return false
		}
		seen[index] = struct{}{}
	}
	return true
}

func validToken(value string) bool {
	if strings.TrimSpace(value) == "" || len(value) > 64 {
		return false
	}
	for _, character := range value {
		if !(character == '_' || character == '-' || (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || (character >= '0' && character <= '9')) {
			return false
		}
	}
	return true
}

func validWebURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil
}

func escapeQuotes(value string) string {
	value = strings.ReplaceAll(value, "\r", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return strings.ReplaceAll(value, "\"", "\\\"")
}
