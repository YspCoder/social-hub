package weibo

import (
	"bytes"
	"encoding/json"
	"time"

	"social-hub/pkg/socialhub"
)

type stringID string

func (id *stringID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || bytes.Equal(data, []byte("null")) {
		*id = ""
		return nil
	}
	if data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		*id = stringID(value)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return err
	}
	*id = stringID(number.String())
	return nil
}

type weiboUser struct {
	APIError
	ID              stringID `json:"id"`
	IDString        string   `json:"idstr"`
	ScreenName      string   `json:"screen_name"`
	Name            string   `json:"name"`
	ProfileImageURL string   `json:"profile_image_url"`
	AvatarLarge     string   `json:"avatar_large"`
	ProfileURL      string   `json:"profile_url"`
	Domain          string   `json:"domain"`
	Verified        bool     `json:"verified"`
	VerifiedType    int      `json:"verified_type"`
	Description     string   `json:"description"`
	FollowersCount  int64    `json:"followers_count"`
	FriendsCount    int64    `json:"friends_count"`
	StatusesCount   int64    `json:"statuses_count"`
}

type weiboStatus struct {
	APIError
	ID              stringID        `json:"id"`
	IDString        string          `json:"idstr"`
	MID             stringID        `json:"mid"`
	Text            string          `json:"text"`
	CreatedAt       string          `json:"created_at"`
	User            *weiboUser      `json:"user"`
	RetweetedStatus *weiboStatus    `json:"retweeted_status"`
	ReplyStatusID   stringID        `json:"in_reply_to_status_id"`
	ThumbnailPic    string          `json:"thumbnail_pic"`
	BmiddlePic      string          `json:"bmiddle_pic"`
	OriginalPic     string          `json:"original_pic"`
	PicIDs          []string        `json:"pic_ids"`
	PicURLs         []weiboPicture  `json:"pic_urls"`
	RepostsCount    int64           `json:"reposts_count"`
	CommentsCount   int64           `json:"comments_count"`
	AttitudesCount  int64           `json:"attitudes_count"`
	Visible         weiboVisibility `json:"visible"`
}

type weiboPicture struct {
	ThumbnailPic string `json:"thumbnail_pic"`
}

type weiboVisibility struct {
	Type   int      `json:"type"`
	ListID stringID `json:"list_id"`
}

type weiboComment struct {
	APIError
	ID           stringID      `json:"id"`
	IDString     string        `json:"idstr"`
	Text         string        `json:"text"`
	CreatedAt    string        `json:"created_at"`
	User         *weiboUser    `json:"user"`
	Status       *weiboStatus  `json:"status"`
	ReplyComment *weiboComment `json:"reply_comment"`
}

type statusListResponse struct {
	APIError
	Statuses    []weiboStatus `json:"statuses"`
	TotalNumber int           `json:"total_number"`
}

type commentListResponse struct {
	APIError
	Comments    []weiboComment `json:"comments"`
	TotalNumber int            `json:"total_number"`
}

func mapUser(accountID socialhub.AccountID, input weiboUser) *socialhub.User {
	id := firstNonEmpty(input.IDString, string(input.ID))
	profileURL := ""
	if input.ProfileURL != "" {
		profileURL = "https://weibo.com/u/" + id
	}
	avatarURL := firstNonEmpty(input.AvatarLarge, input.ProfileImageURL)
	accountType := "personal"
	if input.Verified {
		accountType = "verified"
	}
	extension, _ := json.Marshal(map[string]any{
		"verified":        input.Verified,
		"verified_type":   input.VerifiedType,
		"description":     input.Description,
		"followers_count": input.FollowersCount,
		"friends_count":   input.FriendsCount,
		"statuses_count":  input.StatusesCount,
	})
	return &socialhub.User{
		Platform:    "weibo",
		AccountID:   accountID,
		ID:          id,
		Username:    stringPointer(input.ScreenName),
		DisplayName: stringPointer(firstNonEmpty(input.Name, input.ScreenName)),
		AvatarURL:   stringPointer(avatarURL),
		ProfileURL:  stringPointer(profileURL),
		AccountType: stringPointer(accountType),
		Extensions:  map[string]json.RawMessage{"weibo.user": extension},
	}
}

func mapStatus(accountID socialhub.AccountID, input weiboStatus, observedAt time.Time) *socialhub.Post {
	id := firstNonEmpty(input.IDString, string(input.ID))
	post := &socialhub.Post{
		Platform:  "weibo",
		AccountID: accountID,
		ID:        id,
		Text:      stringPointer(input.Text),
		CreatedAt: parseWeiboTime(input.CreatedAt),
	}
	if input.User != nil {
		authorID := firstNonEmpty(input.User.IDString, string(input.User.ID))
		post.AuthorID = stringPointer(authorID)
		if authorID != "" {
			postURL := "https://weibo.com/" + authorID + "/" + id
			post.URL = &postURL
		}
	}
	if input.ReplyStatusID != "" && string(input.ReplyStatusID) != "0" {
		post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationReply, PostID: string(input.ReplyStatusID)})
	}
	if input.RetweetedStatus != nil {
		targetID := firstNonEmpty(input.RetweetedStatus.IDString, string(input.RetweetedStatus.ID))
		if targetID != "" {
			post.Relations = append(post.Relations, socialhub.PostRelation{Type: socialhub.RelationRepost, PostID: targetID})
		}
	}
	post.Media = mapPictures(input)
	post.Metrics = []socialhub.Metric{
		{Name: "reposts", Value: float64(input.RepostsCount), AsOf: observedAt, Definition: "Weibo reposts_count"},
		{Name: "comments", Value: float64(input.CommentsCount), AsOf: observedAt, Definition: "Weibo comments_count"},
		{Name: "attitudes", Value: float64(input.AttitudesCount), AsOf: observedAt, Definition: "Weibo attitudes_count"},
	}
	extension := map[string]any{"mid": string(input.MID), "visibility_type": input.Visible.Type, "visibility_list_id": string(input.Visible.ListID)}
	if input.RetweetedStatus != nil {
		extension["retweeted_status"] = input.RetweetedStatus
	}
	raw, _ := json.Marshal(extension)
	post.Extensions = map[string]json.RawMessage{"weibo.status": raw}
	return post
}

func mapPictures(input weiboStatus) []socialhub.Media {
	count := len(input.PicIDs)
	if len(input.PicURLs) > count {
		count = len(input.PicURLs)
	}
	if count == 0 && input.OriginalPic != "" {
		return []socialhub.Media{{URL: input.OriginalPic, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady}}
	}
	media := make([]socialhub.Media, 0, count)
	for index := 0; index < count; index++ {
		var id, mediaURL string
		if index < len(input.PicIDs) {
			id = input.PicIDs[index]
		}
		if index < len(input.PicURLs) {
			mediaURL = input.PicURLs[index].ThumbnailPic
		}
		if count == 1 && input.OriginalPic != "" {
			mediaURL = input.OriginalPic
		}
		media = append(media, socialhub.Media{ID: id, URL: mediaURL, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady})
	}
	return media
}

func mapComment(accountID socialhub.AccountID, fallbackPostID string, input weiboComment) socialhub.Comment {
	postID := fallbackPostID
	if input.Status != nil {
		postID = firstNonEmpty(input.Status.IDString, string(input.Status.ID), fallbackPostID)
	}
	comment := socialhub.Comment{
		Platform:  "weibo",
		AccountID: accountID,
		ID:        firstNonEmpty(input.IDString, string(input.ID)),
		PostID:    postID,
		Text:      input.Text,
		CreatedAt: parseWeiboTime(input.CreatedAt),
	}
	if input.User != nil {
		comment.AuthorID = stringPointer(firstNonEmpty(input.User.IDString, string(input.User.ID)))
	}
	if input.ReplyComment != nil {
		comment.ParentID = stringPointer(firstNonEmpty(input.ReplyComment.IDString, string(input.ReplyComment.ID)))
	}
	return comment
}

func parseWeiboTime(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse("Mon Jan 02 15:04:05 -0700 2006", value)
	if err != nil {
		return nil
	}
	return &parsed
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
