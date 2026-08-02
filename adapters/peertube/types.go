package peertube

import "time"

type ActorImage struct {
	Path      string     `json:"path,omitempty"`
	FileURL   string     `json:"fileUrl,omitempty"`
	Width     int        `json:"width,omitempty"`
	Height    int        `json:"height,omitempty"`
	CreatedAt *time.Time `json:"createdAt,omitempty"`
	UpdatedAt *time.Time `json:"updatedAt,omitempty"`
}

// Account is a federated PeerTube actor account.
type Account struct {
	ID                    int64        `json:"id"`
	UserID                *int64       `json:"userId,omitempty"`
	URL                   string       `json:"url,omitempty"`
	Name                  string       `json:"name"`
	DisplayName           string       `json:"displayName,omitempty"`
	Description           *string      `json:"description,omitempty"`
	Host                  string       `json:"host,omitempty"`
	Avatars               []ActorImage `json:"avatars,omitempty"`
	HostRedundancyAllowed *bool        `json:"hostRedundancyAllowed,omitempty"`
	FollowingCount        int64        `json:"followingCount,omitempty"`
	FollowersCount        int64        `json:"followersCount,omitempty"`
	CreatedAt             *time.Time   `json:"createdAt,omitempty"`
	UpdatedAt             *time.Time   `json:"updatedAt,omitempty"`
}

type AccountSummary struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	URL         string       `json:"url,omitempty"`
	Host        string       `json:"host,omitempty"`
	Avatars     []ActorImage `json:"avatars,omitempty"`
}

type VideoChannelSummary struct {
	ID          int64        `json:"id"`
	Name        string       `json:"name,omitempty"`
	DisplayName string       `json:"displayName,omitempty"`
	URL         string       `json:"url,omitempty"`
	Host        string       `json:"host,omitempty"`
	Avatars     []ActorImage `json:"avatars,omitempty"`
}

// VideoChannel is a federated PeerTube channel actor.
type VideoChannel struct {
	ID          int64          `json:"id"`
	URL         string         `json:"url,omitempty"`
	Name        string         `json:"name"`
	DisplayName string         `json:"displayName,omitempty"`
	Description *string        `json:"description,omitempty"`
	Support     *string        `json:"support,omitempty"`
	Host        string         `json:"host,omitempty"`
	Avatars     []ActorImage   `json:"avatars,omitempty"`
	Banners     []ActorImage   `json:"banners,omitempty"`
	IsLocal     bool           `json:"isLocal,omitempty"`
	CreatedAt   *time.Time     `json:"createdAt,omitempty"`
	UpdatedAt   *time.Time     `json:"updatedAt,omitempty"`
	Owner       AccountSummary `json:"ownerAccount,omitempty"`
}

type NumberConstant struct {
	ID    int    `json:"id"`
	Label string `json:"label,omitempty"`
}

type StringConstant struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

type Thumbnail struct {
	FileURL     string `json:"fileUrl,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	AspectRatio string `json:"aspectRatio,omitempty"`
}

type VideoFile struct {
	ID              int64          `json:"id"`
	FileURL         string         `json:"fileUrl,omitempty"`
	FileDownloadURL string         `json:"fileDownloadUrl,omitempty"`
	Size            int64          `json:"size,omitempty"`
	Width           float64        `json:"width,omitempty"`
	Height          float64        `json:"height,omitempty"`
	FPS             float64        `json:"fps,omitempty"`
	Resolution      NumberConstant `json:"resolution,omitempty"`
	HasAudio        bool           `json:"hasAudio,omitempty"`
	HasVideo        bool           `json:"hasVideo,omitempty"`
}

type StreamingPlaylist struct {
	ID          int64       `json:"id"`
	PlaylistURL string      `json:"playlistUrl,omitempty"`
	Files       []VideoFile `json:"files,omitempty"`
}

// Video is the typed PeerTube video representation used by read workflows.
type Video struct {
	ID                    int64               `json:"id"`
	UUID                  string              `json:"uuid"`
	ShortUUID             string              `json:"shortUUID,omitempty"`
	URL                   string              `json:"url,omitempty"`
	Name                  string              `json:"name"`
	Description           *string             `json:"description,omitempty"`
	TruncatedDescription  *string             `json:"truncatedDescription,omitempty"`
	CreatedAt             *time.Time          `json:"createdAt,omitempty"`
	PublishedAt           *time.Time          `json:"publishedAt,omitempty"`
	UpdatedAt             *time.Time          `json:"updatedAt,omitempty"`
	OriginallyPublishedAt *time.Time          `json:"originallyPublishedAt,omitempty"`
	Duration              int64               `json:"duration,omitempty"`
	Views                 int64               `json:"views,omitempty"`
	Likes                 int64               `json:"likes,omitempty"`
	Dislikes              int64               `json:"dislikes,omitempty"`
	Comments              int64               `json:"comments,omitempty"`
	IsLive                bool                `json:"isLive,omitempty"`
	IsLocal               bool                `json:"isLocal,omitempty"`
	NSFW                  bool                `json:"nsfw,omitempty"`
	Privacy               NumberConstant      `json:"privacy,omitempty"`
	State                 NumberConstant      `json:"state,omitempty"`
	Category              NumberConstant      `json:"category,omitempty"`
	Licence               NumberConstant      `json:"licence,omitempty"`
	Language              StringConstant      `json:"language,omitempty"`
	Account               AccountSummary      `json:"account,omitempty"`
	Channel               VideoChannelSummary `json:"channel,omitempty"`
	Tags                  []string            `json:"tags,omitempty"`
	Thumbnails            []Thumbnail         `json:"thumbnails,omitempty"`
	Files                 []VideoFile         `json:"files,omitempty"`
	StreamingPlaylists    []StreamingPlaylist `json:"streamingPlaylists,omitempty"`
}

type VideoListResponse struct {
	Total int64   `json:"total"`
	Data  []Video `json:"data"`
}

type VideoChannelListResponse struct {
	Total int64          `json:"total"`
	Data  []VideoChannel `json:"data"`
}

// VideoComment is one PeerTube comment or top-level thread root.
type VideoComment struct {
	ID                          int64      `json:"id"`
	URL                         string     `json:"url,omitempty"`
	Text                        string     `json:"text,omitempty"`
	ThreadID                    int64      `json:"threadId,omitempty"`
	InReplyToCommentID          *int64     `json:"inReplyToCommentId,omitempty"`
	VideoID                     int64      `json:"videoId,omitempty"`
	CreatedAt                   *time.Time `json:"createdAt,omitempty"`
	UpdatedAt                   *time.Time `json:"updatedAt,omitempty"`
	DeletedAt                   *time.Time `json:"deletedAt,omitempty"`
	IsDeleted                   bool       `json:"isDeleted,omitempty"`
	HeldForReview               bool       `json:"heldForReview,omitempty"`
	TotalRepliesFromVideoAuthor int64      `json:"totalRepliesFromVideoAuthor,omitempty"`
	TotalReplies                int64      `json:"totalReplies,omitempty"`
	Account                     Account    `json:"account,omitempty"`
}

type commentThreadResponse struct {
	Total                   int64          `json:"total"`
	TotalNotDeletedComments int64          `json:"totalNotDeletedComments"`
	Data                    []VideoComment `json:"data"`
}

type commentPostResponse struct {
	Comment VideoComment `json:"comment"`
}

// VideoCommentThread preserves PeerTube's recursive thread shape.
type VideoCommentThread struct {
	Comment  VideoComment         `json:"comment"`
	Children []VideoCommentThread `json:"children,omitempty"`
}

type oauthLocalClient struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type oauthTokenResponse struct {
	TokenType             string `json:"token_type"`
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	ExpiresIn             int64  `json:"expires_in"`
	RefreshTokenExpiresIn int64  `json:"refresh_token_expires_in"`
}
