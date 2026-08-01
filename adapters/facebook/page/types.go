package page

import (
	"time"

	"social-hub/pkg/socialhub"
)

type idResponse struct {
	ID string `json:"id"`
}

type successResponse struct {
	Success bool `json:"success"`
}

type graphPage struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Link    string `json:"link"`
	Picture struct {
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	} `json:"picture"`
}

type graphPost struct {
	ID           string           `json:"id"`
	Message      string           `json:"message"`
	CreatedTime  *time.Time       `json:"created_time"`
	PermalinkURL string           `json:"permalink_url"`
	From         graphIdentity    `json:"from"`
	Attachments  graphAttachments `json:"attachments"`
}

type graphIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type graphAttachments struct {
	Data []graphAttachment `json:"data"`
}

type graphAttachment struct {
	Type  string `json:"type"`
	URL   string `json:"url"`
	Media struct {
		Image struct {
			Src    string `json:"src"`
			Width  *int   `json:"width"`
			Height *int   `json:"height"`
		} `json:"image"`
	} `json:"media"`
	Target struct {
		ID string `json:"id"`
	} `json:"target"`
}

type graphPaging struct {
	Cursors struct {
		Before *string `json:"before"`
		After  *string `json:"after"`
	} `json:"cursors"`
	Next string `json:"next"`
}

type graphPosts struct {
	Data   []graphPost `json:"data"`
	Paging graphPaging `json:"paging"`
}

type graphComment struct {
	ID          string        `json:"id"`
	Message     string        `json:"message"`
	CreatedTime *time.Time    `json:"created_time"`
	From        graphIdentity `json:"from"`
	Parent      *struct {
		ID string `json:"id"`
	} `json:"parent"`
}

type graphComments struct {
	Data   []graphComment `json:"data"`
	Paging graphPaging    `json:"paging"`
}

func mapPage(accountID socialhub.AccountID, input graphPage) *socialhub.User {
	return &socialhub.User{
		Platform:    "facebook",
		AccountID:   accountID,
		ID:          input.ID,
		DisplayName: stringPointer(input.Name),
		AvatarURL:   stringPointer(input.Picture.Data.URL),
		ProfileURL:  stringPointer(input.Link),
	}
}

func mapPost(accountID socialhub.AccountID, input graphPost) *socialhub.Post {
	post := &socialhub.Post{
		Platform:  "facebook",
		AccountID: accountID,
		ID:        input.ID,
		Text:      stringPointer(input.Message),
		CreatedAt: input.CreatedTime,
		URL:       stringPointer(input.PermalinkURL),
	}
	if input.From.ID != "" {
		post.AuthorID = stringPointer(input.From.ID)
	}
	for _, attachment := range input.Attachments.Data {
		mediaType := socialhub.MediaTypeDocument
		switch {
		case attachment.Type == "photo" || attachment.Type == "album":
			mediaType = socialhub.MediaTypeImage
		case attachment.Type == "video" || attachment.Type == "video_inline":
			mediaType = socialhub.MediaTypeVideo
		}
		post.Media = append(post.Media, socialhub.Media{
			ID:     attachment.Target.ID,
			URL:    firstNonEmpty(attachment.Media.Image.Src, attachment.URL),
			Type:   mediaType,
			State:  socialhub.MediaStateReady,
			Width:  attachment.Media.Image.Width,
			Height: attachment.Media.Image.Height,
		})
	}
	return post
}

func mapPostPage(accountID socialhub.AccountID, input graphPosts) socialhub.Page[socialhub.Post] {
	items := make([]socialhub.Post, 0, len(input.Data))
	for _, post := range input.Data {
		items = append(items, *mapPost(accountID, post))
	}
	return socialhub.Page[socialhub.Post]{Items: items, NextCursor: input.Paging.Cursors.After, PrevCursor: input.Paging.Cursors.Before, HasMore: input.Paging.Next != ""}
}

func mapCommentPage(accountID socialhub.AccountID, postID string, input graphComments) socialhub.Page[socialhub.Comment] {
	items := make([]socialhub.Comment, 0, len(input.Data))
	for _, comment := range input.Data {
		mapped := socialhub.Comment{Platform: "facebook", AccountID: accountID, ID: comment.ID, PostID: postID, Text: comment.Message, CreatedAt: comment.CreatedTime}
		if comment.From.ID != "" {
			mapped.AuthorID = stringPointer(comment.From.ID)
		}
		if comment.Parent != nil {
			mapped.ParentID = stringPointer(comment.Parent.ID)
		}
		items = append(items, mapped)
	}
	return socialhub.Page[socialhub.Comment]{Items: items, NextCursor: input.Paging.Cursors.After, PrevCursor: input.Paging.Cursors.Before, HasMore: input.Paging.Next != ""}
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
