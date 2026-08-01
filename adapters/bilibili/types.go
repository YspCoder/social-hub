package bilibili

import (
	"encoding/json"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

type userInfo struct {
	Name   string `json:"name"`
	Face   string `json:"face"`
	OpenID string `json:"openid"`
}

type archiveVideo struct {
	CID       int64  `json:"cid"`
	Filename  string `json:"filename"`
	Duration  int64  `json:"duration"`
	ShareURL  string `json:"share_url"`
	IframeURL string `json:"iframe_url"`
}

type archiveAudit struct {
	State        int    `json:"state"`
	StateDesc    string `json:"state_desc"`
	RejectReason string `json:"reject_reason"`
}

type archive struct {
	ResourceID  string       `json:"resource_id"`
	Title       string       `json:"title"`
	Cover       string       `json:"cover"`
	TID         int          `json:"tid"`
	NoReprint   int          `json:"no_reprint"`
	Description string       `json:"desc"`
	Tag         string       `json:"tag"`
	Copyright   int          `json:"copyright"`
	Video       archiveVideo `json:"video_info"`
	Audit       archiveAudit `json:"addit_info"`
	CreatedAt   int64        `json:"ctime"`
	PublishedAt int64        `json:"ptime"`
}

type archiveList struct {
	List []archive `json:"list"`
	Page struct {
		Number int `json:"pn"`
		Size   int `json:"ps"`
		Total  int `json:"total"`
	} `json:"page"`
}

func mapUser(accountID socialhub.AccountID, input userInfo) *socialhub.User {
	return &socialhub.User{Platform: "bilibili", AccountID: accountID, ID: input.OpenID, DisplayName: stringPointer(input.Name), AvatarURL: stringPointer(input.Face)}
}

func mapArchive(accountID socialhub.AccountID, openID string, input archive) *socialhub.Post {
	createdAt := unixPointer(input.CreatedAt)
	updatedAt := unixPointer(input.PublishedAt)
	if updatedAt == nil {
		updatedAt = createdAt
	}
	state := socialhub.PublishStatePending
	message := input.Audit.StateDesc
	mediaState := socialhub.MediaStateProcessing
	if input.Audit.State == 0 {
		state = socialhub.PublishStatePublished
		message = ""
		mediaState = socialhub.MediaStateReady
	} else if input.Audit.RejectReason != "" {
		state = socialhub.PublishStateFailed
		message = input.Audit.RejectReason
		mediaState = socialhub.MediaStateFailed
	}
	seconds := time.Duration(input.Video.Duration) * time.Second
	videoExtension, _ := json.Marshal(map[string]any{"cid": input.Video.CID, "filename": input.Video.Filename, "iframe_url": input.Video.IframeURL})
	postExtension, _ := json.Marshal(map[string]any{
		"tid": input.TID, "tags": strings.Split(input.Tag, ","), "copyright": input.Copyright,
		"description": input.Description, "no_reprint": input.NoReprint,
		"audit_state": input.Audit.State, "audit_state_desc": input.Audit.StateDesc,
	})
	media := []socialhub.Media{{ID: input.Video.Filename, URL: input.Video.ShareURL, Type: socialhub.MediaTypeVideo, Duration: &seconds, State: mediaState, Extensions: map[string]json.RawMessage{"bilibili.video": videoExtension}}}
	if input.Cover != "" {
		media = append(media, socialhub.Media{URL: input.Cover, Type: socialhub.MediaTypeImage, State: socialhub.MediaStateReady})
	}
	return &socialhub.Post{
		Platform: "bilibili", AccountID: accountID, ID: input.ResourceID, AuthorID: stringPointer(openID), Text: stringPointer(input.Title),
		Media: media, CreatedAt: createdAt, URL: stringPointer(input.Video.ShareURL),
		Status:     &socialhub.PublishStatus{ID: input.ResourceID, State: state, Message: message, UpdatedAt: updatedAt},
		Extensions: map[string]json.RawMessage{"bilibili.archive": postExtension},
	}
}

func unixPointer(seconds int64) *time.Time {
	if seconds <= 0 {
		return nil
	}
	value := time.Unix(seconds, 0)
	return &value
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
