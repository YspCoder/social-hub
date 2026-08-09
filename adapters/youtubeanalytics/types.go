package youtubeanalytics

import (
	"bytes"
	"encoding/json"
)

type Metric string
type Dimension string

const (
	MetricViews                   Metric = "views"
	MetricEngagedViews            Metric = "engagedViews"
	MetricLikes                   Metric = "likes"
	MetricComments                Metric = "comments"
	MetricShares                  Metric = "shares"
	MetricEstimatedMinutesWatched Metric = "estimatedMinutesWatched"
	MetricEstimatedRevenue        Metric = "estimatedRevenue"
	MetricEstimatedAdRevenue      Metric = "estimatedAdRevenue"

	DimensionDay                 Dimension = "day"
	DimensionMonth               Dimension = "month"
	DimensionVideo               Dimension = "video"
	DimensionPlaylist            Dimension = "playlist"
	DimensionChannel             Dimension = "channel"
	DimensionCountry             Dimension = "country"
	DimensionGroup               Dimension = "group"
	DimensionTrafficSourceType   Dimension = "insightTrafficSourceType"
	DimensionTrafficSourceDetail Dimension = "insightTrafficSourceDetail"
)

type Filter struct {
	Dimension Dimension
	Values    []string
}

type Sort struct {
	Name       string
	Descending bool
}

type ReportQuery struct {
	StartDate                    string
	EndDate                      string
	Metrics                      []Metric
	Dimensions                   []Dimension
	Filters                      []Filter
	Sort                         []Sort
	Currency                     string
	MaxResults                   int32
	StartIndex                   int32
	IncludeHistoricalChannelData bool
	Monetary                     bool
}

type ColumnType string
type DataType string

const (
	ColumnDimension ColumnType = "DIMENSION"
	ColumnMetric    ColumnType = "METRIC"

	DataString  DataType = "STRING"
	DataInteger DataType = "INTEGER"
	DataFloat   DataType = "FLOAT"
	DataBoolean DataType = "BOOLEAN"
)

type ColumnHeader struct {
	Name       string     `json:"name"`
	ColumnType ColumnType `json:"columnType"`
	DataType   DataType   `json:"dataType"`
}

type EmbeddedError struct {
	Domain               string   `json:"domain,omitempty"`
	Code                 string   `json:"code,omitempty"`
	Argument             []string `json:"argument,omitempty"`
	Location             string   `json:"location,omitempty"`
	LocationType         string   `json:"locationType,omitempty"`
	ExternalErrorMessage string   `json:"externalErrorMessage,omitempty"`
}

type EmbeddedErrors struct {
	RequestID string          `json:"requestId,omitempty"`
	Code      string          `json:"code,omitempty"`
	Errors    []EmbeddedError `json:"error,omitempty"`
}

type Report struct {
	Kind          string          `json:"kind"`
	ColumnHeaders []ColumnHeader  `json:"columnHeaders,omitempty"`
	Rows          [][]any         `json:"rows,omitempty"`
	Errors        *EmbeddedErrors `json:"errors,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

func (value *Report) UnmarshalJSON(data []byte) error {
	type alias Report
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode((*alias)(value)); err != nil {
		return err
	}
	value.Raw = append(value.Raw[:0], data...)
	return nil
}

type ResourceKind string

const (
	ResourceChannel  ResourceKind = "youtube#channel"
	ResourcePlaylist ResourceKind = "youtube#playlist"
	ResourceVideo    ResourceKind = "youtube#video"
	ResourceAsset    ResourceKind = "youtubePartner#asset"
)

type GroupSnippet struct {
	PublishedAt string `json:"publishedAt,omitempty"`
	Title       string `json:"title,omitempty"`
}

type GroupContentDetails struct {
	ItemCount uint64       `json:"itemCount,omitempty,string"`
	ItemType  ResourceKind `json:"itemType,omitempty"`
}

type Group struct {
	ID             string               `json:"id,omitempty"`
	ETag           string               `json:"etag,omitempty"`
	Kind           string               `json:"kind,omitempty"`
	Snippet        *GroupSnippet        `json:"snippet,omitempty"`
	ContentDetails *GroupContentDetails `json:"contentDetails,omitempty"`
	Errors         *EmbeddedErrors      `json:"errors,omitempty"`
	Raw            json.RawMessage      `json:"-"`
}

type GroupItemResource struct {
	ID   string       `json:"id,omitempty"`
	Kind ResourceKind `json:"kind,omitempty"`
}

type GroupItem struct {
	ID       string             `json:"id,omitempty"`
	GroupID  string             `json:"groupId,omitempty"`
	ETag     string             `json:"etag,omitempty"`
	Kind     string             `json:"kind,omitempty"`
	Resource *GroupItemResource `json:"resource,omitempty"`
	Errors   *EmbeddedErrors    `json:"errors,omitempty"`
	Raw      json.RawMessage    `json:"-"`
}

type ListGroupsRequest struct {
	IDs       []string
	Mine      bool
	PageToken string
}

type ListGroupsResponse struct {
	Kind          string          `json:"kind,omitempty"`
	ETag          string          `json:"etag,omitempty"`
	Items         []Group         `json:"items,omitempty"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
	Errors        *EmbeddedErrors `json:"errors,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type ListGroupItemsResponse struct {
	Kind   string          `json:"kind,omitempty"`
	ETag   string          `json:"etag,omitempty"`
	Items  []GroupItem     `json:"items,omitempty"`
	Errors *EmbeddedErrors `json:"errors,omitempty"`
	Raw    json.RawMessage `json:"-"`
}

type CreateGroupInput struct {
	Title    string
	ItemType ResourceKind
}

type AddGroupItemInput struct {
	GroupID    string
	ResourceID string
	Kind       ResourceKind
}

type AddGroupItemResult struct {
	Item           *GroupItem
	AlreadyPresent bool
}

func captureRaw(data []byte, target any, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

func (value *Group) UnmarshalJSON(data []byte) error {
	type alias Group
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *GroupItem) UnmarshalJSON(data []byte) error {
	type alias GroupItem
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *ListGroupsResponse) UnmarshalJSON(data []byte) error {
	type alias ListGroupsResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *ListGroupItemsResponse) UnmarshalJSON(data []byte) error {
	type alias ListGroupItemsResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

type QuotaPolicy struct {
	MaximumGroupItems        int
	MaximumFilterIDs         int
	MaximumTrafficSourceCost int
}

func DefaultQuotaPolicy() QuotaPolicy {
	return QuotaPolicy{MaximumGroupItems: 500, MaximumFilterIDs: 500, MaximumTrafficSourceCost: 50_000}
}
