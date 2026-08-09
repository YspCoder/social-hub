package youtubereporting

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

const DefaultMaxDownloadBytes int64 = 256 << 20

type Page[T any] struct {
	Items         []T
	NextPageToken string
}

type ListRequest struct {
	PageSize             int32
	PageToken            string
	IncludeSystemManaged bool
}

type ListReportsRequest struct {
	PageSize           int32
	PageToken          string
	CreatedAfter       time.Time
	StartTimeAtOrAfter time.Time
	StartTimeBefore    time.Time
}

type CreateJobInput struct {
	ReportTypeID string
	Name         string
}

type ReportType struct {
	ID            string          `json:"id,omitempty"`
	Name          string          `json:"name,omitempty"`
	DeprecateTime string          `json:"deprecateTime,omitempty"`
	SystemManaged bool            `json:"systemManaged,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type Job struct {
	ID            string          `json:"id,omitempty"`
	ReportTypeID  string          `json:"reportTypeId,omitempty"`
	Name          string          `json:"name,omitempty"`
	CreateTime    string          `json:"createTime,omitempty"`
	ExpireTime    string          `json:"expireTime,omitempty"`
	SystemManaged bool            `json:"systemManaged,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type Report struct {
	ID            string          `json:"id,omitempty"`
	JobID         string          `json:"jobId,omitempty"`
	StartTime     string          `json:"startTime,omitempty"`
	EndTime       string          `json:"endTime,omitempty"`
	CreateTime    string          `json:"createTime,omitempty"`
	JobExpireTime string          `json:"jobExpireTime,omitempty"`
	DownloadURL   string          `json:"downloadUrl,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type DownloadOptions struct {
	// MaxBytes bounds decompressed output. Zero uses DefaultMaxDownloadBytes.
	MaxBytes int64
	// Gzip asks Google for gzip transfer encoding and transparently decompresses
	// it before writing the CSV to Output.
	Gzip bool
}

type DownloadResult struct {
	Report          Report
	BytesWritten    int64
	ContentType     string
	ContentEncoding string
	ETag            string
	LastModified    string
}

type ReportingWorkflow interface {
	ListReportTypes(context.Context, ListRequest, ...socialhub.CallOption) (Page[ReportType], error)
	CreateJob(context.Context, CreateJobInput, ...socialhub.CallOption) (Job, error)
	GetJob(context.Context, string, ...socialhub.CallOption) (Job, error)
	ListJobs(context.Context, ListRequest, ...socialhub.CallOption) (Page[Job], error)
	DeleteJob(context.Context, string, ...socialhub.CallOption) error
	ListReports(context.Context, string, ListReportsRequest, ...socialhub.CallOption) (Page[Report], error)
	GetReport(context.Context, string, string, ...socialhub.CallOption) (Report, error)
	DownloadReport(context.Context, string, string, io.Writer, DownloadOptions, ...socialhub.CallOption) (DownloadResult, error)
}

type listReportTypesResponse struct {
	ReportTypes   []ReportType    `json:"reportTypes,omitempty"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type listJobsResponse struct {
	Jobs          []Job           `json:"jobs,omitempty"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

type listReportsResponse struct {
	Reports       []Report        `json:"reports,omitempty"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
	Raw           json.RawMessage `json:"-"`
}

func captureRaw(data []byte, target any, raw *json.RawMessage) error {
	if err := json.Unmarshal(data, target); err != nil {
		return err
	}
	*raw = append((*raw)[:0], data...)
	return nil
}

func (value *ReportType) UnmarshalJSON(data []byte) error {
	type alias ReportType
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *Job) UnmarshalJSON(data []byte) error {
	type alias Job
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *Report) UnmarshalJSON(data []byte) error {
	type alias Report
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *listReportTypesResponse) UnmarshalJSON(data []byte) error {
	type alias listReportTypesResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *listJobsResponse) UnmarshalJSON(data []byte) error {
	type alias listJobsResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}

func (value *listReportsResponse) UnmarshalJSON(data []byte) error {
	type alias listReportsResponse
	return captureRaw(data, (*alias)(value), &value.Raw)
}
