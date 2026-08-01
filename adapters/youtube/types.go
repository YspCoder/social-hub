package youtube

import "time"

type thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type thumbnails struct {
	Default thumbnail `json:"default"`
	Medium  thumbnail `json:"medium"`
	High    thumbnail `json:"high"`
	Maxres  thumbnail `json:"maxres"`
}

type channelList struct {
	Items []youtubeChannel `json:"items"`
}

type youtubeChannel struct {
	ID      string `json:"id"`
	Snippet struct {
		Title       string     `json:"title"`
		Description string     `json:"description"`
		CustomURL   string     `json:"customUrl"`
		Thumbnails  thumbnails `json:"thumbnails"`
	} `json:"snippet"`
	Statistics struct {
		ViewCount       string `json:"viewCount"`
		SubscriberCount string `json:"subscriberCount"`
		VideoCount      string `json:"videoCount"`
	} `json:"statistics"`
}

type videoList struct {
	Items []youtubeVideo `json:"items"`
}

type youtubeVideo struct {
	ID      string `json:"id"`
	Snippet struct {
		ChannelID   string     `json:"channelId"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		PublishedAt *time.Time `json:"publishedAt"`
		Thumbnails  thumbnails `json:"thumbnails"`
	} `json:"snippet"`
	ContentDetails struct {
		Duration string `json:"duration"`
	} `json:"contentDetails"`
	Status struct {
		UploadStatus            string `json:"uploadStatus"`
		PrivacyStatus           string `json:"privacyStatus"`
		MadeForKids             bool   `json:"madeForKids"`
		SelfDeclaredMadeForKids bool   `json:"selfDeclaredMadeForKids"`
		ContainsSyntheticMedia  bool   `json:"containsSyntheticMedia"`
	} `json:"status"`
	Statistics struct {
		ViewCount     string `json:"viewCount"`
		LikeCount     string `json:"likeCount"`
		CommentCount  string `json:"commentCount"`
		FavoriteCount string `json:"favoriteCount"`
	} `json:"statistics"`
}

type searchList struct {
	NextPageToken string         `json:"nextPageToken"`
	PrevPageToken string         `json:"prevPageToken"`
	Items         []searchResult `json:"items"`
}

type searchResult struct {
	ID struct {
		VideoID string `json:"videoId"`
	} `json:"id"`
	Snippet struct {
		ChannelID   string     `json:"channelId"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		PublishedAt *time.Time `json:"publishedAt"`
		Thumbnails  thumbnails `json:"thumbnails"`
	} `json:"snippet"`
}

type commentList struct {
	NextPageToken string          `json:"nextPageToken"`
	PrevPageToken string          `json:"prevPageToken"`
	Items         []commentThread `json:"items"`
}

type commentThread struct {
	ID      string `json:"id"`
	Snippet struct {
		VideoID         string         `json:"videoId"`
		TotalReplyCount int64          `json:"totalReplyCount"`
		TopLevelComment youtubeComment `json:"topLevelComment"`
	} `json:"snippet"`
	Replies struct {
		Comments []youtubeComment `json:"comments"`
	} `json:"replies"`
}

type youtubeComment struct {
	ID      string `json:"id"`
	Snippet struct {
		VideoID           string     `json:"videoId"`
		ParentID          string     `json:"parentId"`
		TextOriginal      string     `json:"textOriginal"`
		AuthorDisplayName string     `json:"authorDisplayName"`
		PublishedAt       *time.Time `json:"publishedAt"`
		LikeCount         int64      `json:"likeCount"`
		AuthorChannelID   struct {
			Value string `json:"value"`
		} `json:"authorChannelId"`
	} `json:"snippet"`
}
