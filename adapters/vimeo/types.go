package vimeo

import (
	"encoding/json"
	"time"
)

type vimeoPage[T any] struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
	Paging  struct {
		Next     string `json:"next"`
		Previous string `json:"previous"`
	} `json:"paging"`
	Data []T `json:"data"`
}

type vimeoPicture struct {
	URI      string `json:"uri"`
	Active   bool   `json:"active"`
	BaseLink string `json:"base_link"`
	Sizes    []struct {
		Width  int    `json:"width"`
		Height int    `json:"height"`
		Link   string `json:"link"`
	} `json:"sizes"`
}

type vimeoUser struct {
	URI         string       `json:"uri"`
	Name        string       `json:"name"`
	Link        string       `json:"link"`
	Location    string       `json:"location"`
	Bio         string       `json:"bio"`
	CreatedTime *time.Time   `json:"created_time"`
	Account     string       `json:"account"`
	Pictures    vimeoPicture `json:"pictures"`
	Websites    []struct {
		Name        string `json:"name"`
		Link        string `json:"link"`
		Description string `json:"description"`
	} `json:"websites"`
	ResourceKey string `json:"resource_key"`
}

type vimeoVideo struct {
	URI          string     `json:"uri"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Link         string     `json:"link"`
	Duration     int64      `json:"duration"`
	Width        int        `json:"width"`
	Height       int        `json:"height"`
	CreatedTime  *time.Time `json:"created_time"`
	ModifiedTime *time.Time `json:"modified_time"`
	ReleaseTime  *time.Time `json:"release_time"`
	Privacy      struct {
		View string `json:"view"`
	} `json:"privacy"`
	Pictures vimeoPicture `json:"pictures"`
	Stats    struct {
		Plays *int64 `json:"plays"`
	} `json:"stats"`
	User   vimeoUser `json:"user"`
	Status string    `json:"status"`
	Upload struct {
		Approach   string `json:"approach"`
		Status     string `json:"status"`
		UploadLink string `json:"upload_link"`
		Size       int64  `json:"size"`
	} `json:"upload"`
	Transcode struct {
		Status string `json:"status"`
	} `json:"transcode"`
	ResourceKey string `json:"resource_key"`
	Metadata    struct {
		Connections struct {
			Comments struct {
				Total *int64 `json:"total"`
			} `json:"comments"`
			Likes struct {
				Total *int64 `json:"total"`
			} `json:"likes"`
		} `json:"connections"`
	} `json:"metadata"`
}

type vimeoActivity struct {
	Type string     `json:"type"`
	Time *time.Time `json:"time"`
	Clip vimeoVideo `json:"clip"`
	User vimeoUser  `json:"user"`
}

type vimeoComment struct {
	URI         string     `json:"uri"`
	Type        string     `json:"type"`
	Text        string     `json:"text"`
	CreatedOn   *time.Time `json:"created_on"`
	User        vimeoUser  `json:"user"`
	ResourceKey string     `json:"resource_key"`
	Metadata    struct {
		Connections struct {
			User vimeoUser `json:"user"`
		} `json:"connections"`
	} `json:"metadata"`
}

type vimeoErrorEnvelope struct {
	Error            string          `json:"error"`
	DeveloperMessage string          `json:"developer_message"`
	ErrorCode        json.RawMessage `json:"error_code"`
	Link             string          `json:"link"`
}
