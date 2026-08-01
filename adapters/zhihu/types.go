package zhihu

import "time"

// SearchRequest selects Zhihu site-search results.
type SearchRequest struct {
	Query string
	Count int
}

// SearchResult is the documented Zhihu site-search response.
type SearchResult struct {
	HasMore      bool
	SearchHashID string
	EmptyReason  string
	Items        []SearchItem
}

// SearchItem preserves fields specific to Zhihu ranked search.
type SearchItem struct {
	Title           string
	ContentType     string
	ContentID       string
	ContentText     string
	URL             string
	CommentCount    int64
	VoteUpCount     int64
	AuthorName      string
	AuthorAvatar    string
	AuthorBadge     string
	AuthorBadgeText string
	EditedAt        *time.Time
	Comments        []string
	AuthorityLevel  string
	RankingScore    float64
}

// HotListResult is the documented current Zhihu hot list.
type HotListResult struct {
	Total int64
	Items []HotListItem
}

// HotListItem is one question or article returned by the hot list.
type HotListItem struct {
	Title        string
	URL          string
	ThumbnailURL string
	Summary      string
}

type searchData struct {
	HasMore      bool             `json:"HasMore"`
	SearchHashID string           `json:"SearchHashId"`
	EmptyReason  string           `json:"EmptyReason"`
	Items        []searchWireItem `json:"Items"`
}

type searchWireItem struct {
	Title           string            `json:"Title"`
	ContentType     string            `json:"ContentType"`
	ContentID       string            `json:"ContentID"`
	ContentText     string            `json:"ContentText"`
	URL             string            `json:"Url"`
	CommentCount    int64             `json:"CommentCount"`
	VoteUpCount     int64             `json:"VoteUpCount"`
	AuthorName      string            `json:"AuthorName"`
	AuthorAvatar    string            `json:"AuthorAvatar"`
	AuthorBadge     string            `json:"AuthorBadge"`
	AuthorBadgeText string            `json:"AuthorBadgeText"`
	EditTime        int64             `json:"EditTime"`
	Comments        []commentWireItem `json:"CommentInfoList"`
	AuthorityLevel  string            `json:"AuthorityLevel"`
	RankingScore    float64           `json:"RankingScore"`
}

type commentWireItem struct {
	Content string `json:"Content"`
}

type hotListData struct {
	Total int64             `json:"Total"`
	Items []hotListWireItem `json:"Items"`
}

type hotListWireItem struct {
	Title        string `json:"Title"`
	URL          string `json:"Url"`
	ThumbnailURL string `json:"ThumbnailUrl"`
	Summary      string `json:"Summary"`
}

type userContentsData struct {
	Items  []userContentItem `json:"Items"`
	Paging paging            `json:"Paging"`
}

type userContentItem struct {
	ContentType   string `json:"ContentType"`
	URL           string `json:"Url"`
	CreatedAt     int64  `json:"CreatedAt"`
	LikeCount     int64  `json:"LikeCount"`
	CommentCount  int64  `json:"CommentCount"`
	FavoriteCount int64  `json:"FavoriteCount"`
	Title         string `json:"Title"`
	Summary       string `json:"Summary"`
}

type paging struct {
	IsEnd      bool   `json:"IsEnd"`
	NextOffset string `json:"NextOffset"`
	Totals     int64  `json:"Totals"`
}
