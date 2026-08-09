package unitystatistics

import (
	"context"
	"io"
	"time"

	"social-hub/pkg/socialhub"
)

const DefaultMaxReportBytes int64 = 256 << 20

type Scale string

const (
	ScaleSummary Scale = "summary"
	ScaleHour    Scale = "hour"
	ScaleDay     Scale = "day"
	ScaleWeek    Scale = "week"
	ScaleMonth   Scale = "month"
)

type ReportFormat string

const (
	FormatCSV  ReportFormat = "csv"
	FormatJSON ReportFormat = "json"
)

type Compression string

const (
	CompressionIdentity Compression = "identity"
	CompressionGzip     Compression = "gzip"
	CompressionDeflate  Compression = "deflate"
)

type AcquisitionMetric string
type SKANMetric string
type AcquisitionBreakdown string
type SKANBreakdown string
type CreativePackType string
type Platform string
type CountryCode string

const (
	BreakdownApp              AcquisitionBreakdown = "app"
	BreakdownCampaign         AcquisitionBreakdown = "campaign"
	BreakdownCountry          AcquisitionBreakdown = "country"
	BreakdownCreativePack     AcquisitionBreakdown = "creativePack"
	BreakdownCreativePackType AcquisitionBreakdown = "creativePackType"
	BreakdownOSVersion        AcquisitionBreakdown = "osVersion"
	BreakdownPlatform         AcquisitionBreakdown = "platform"
	BreakdownSourceAppID      AcquisitionBreakdown = "sourceAppId"
	BreakdownStore            AcquisitionBreakdown = "store"
	BreakdownTargetGame       AcquisitionBreakdown = "targetGame"
	BreakdownEventType        AcquisitionBreakdown = "eventType"
	BreakdownEventName        AcquisitionBreakdown = "eventName"
)

const (
	SKANBreakdownApp             SKANBreakdown = "app"
	SKANBreakdownCampaign        SKANBreakdown = "campaign"
	SKANBreakdownConversionValue SKANBreakdown = "conversionValue"
	SKANBreakdownTargetGame      SKANBreakdown = "targetGame"
)

const (
	CreativePackVideo         CreativePackType = "video"
	CreativePackPlayable      CreativePackType = "playable"
	CreativePackVideoPlayable CreativePackType = "video+playable"
)

const (
	PlatformIOS     Platform = "ios"
	PlatformAndroid Platform = "android"
)

type AcquisitionsReportRequest struct {
	Start             time.Time
	End               time.Time
	Scale             Scale
	Metrics           []AcquisitionMetric
	Breakdowns        []AcquisitionBreakdown
	AppIDs            []string
	CampaignIDs       []string
	GameIDs           []string
	CreativePackIDs   []string
	CreativePackTypes []CreativePackType
	Countries         []CountryCode
	Platforms         []Platform
	EventTypes        []string
	EventNames        []string
	Format            ReportFormat
	EOFMarker         bool
}

type SKANReportRequest struct {
	Start       time.Time
	End         time.Time
	Scale       Scale
	Metrics     []SKANMetric
	Breakdowns  []SKANBreakdown
	AppIDs      []string
	CampaignIDs []string
	GameIDs     []string
	Format      ReportFormat
	EOFMarker   bool
}

type DownloadOptions struct {
	// MaxBytes bounds decompressed output. Zero uses DefaultMaxReportBytes.
	MaxBytes int64
	// Compression selects the requested HTTP transfer encoding. The zero value
	// requests identity encoding.
	Compression Compression
}

type ReportResult struct {
	StatusCode      int
	Format          ReportFormat
	BytesWritten    int64
	ContentType     string
	ContentEncoding string
	NoData          bool
	EOFVerified     bool
	DataRows        int64
	RateLimitPolicy string
	RateLimit       string
	UnityRateLimit  string
}

type ReportsWorkflow interface {
	DownloadAcquisitionsReport(context.Context, AcquisitionsReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (ReportResult, error)
	DownloadSKANReport(context.Context, SKANReportRequest, io.Writer, DownloadOptions, ...socialhub.CallOption) (ReportResult, error)
}

var _ ReportsWorkflow = (*Client)(nil)
