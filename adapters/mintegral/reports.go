package mintegral

import (
	"context"
	"errors"
	"io"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

type ReportsWorkflow interface {
	Status(context.Context, mtg.ReportQuery, ...socialhub.CallOption) (mtg.ReportStatus, error)
	Open(context.Context, mtg.ReportOpenRequest, ...socialhub.CallOption) (*ReportStream, error)
	Consume(context.Context, mtg.ReportConsumeRequest, mtg.ReportHandler, ...socialhub.CallOption) (mtg.ReportDelivery, error)
}

type ReportsService struct{ client *Client }

func (service *ReportsService) Status(ctx context.Context, input mtg.ReportQuery, options ...socialhub.CallOption) (mtg.ReportStatus, error) {
	return callValue(ctx, "report_status", options, func(callCtx context.Context) (mtg.ReportStatus, error) {
		return service.client.sdk.Reports().Status(callCtx, input)
	})
}

func (service *ReportsService) Open(ctx context.Context, input mtg.ReportOpenRequest, options ...socialhub.CallOption) (*ReportStream, error) {
	callCtx, cancel, err := callContext(ctx, "report_open", options)
	if err != nil {
		return nil, err
	}
	stream, err := service.client.sdk.Reports().Open(callCtx, input)
	if err != nil {
		cancel()
		return nil, mapError("report_open", err)
	}
	if stream == nil {
		cancel()
		return nil, mapError("report_open", mtg.ErrUnexpectedResponse)
	}
	return &ReportStream{upstream: stream, cancel: cancel}, nil
}

func (service *ReportsService) Consume(ctx context.Context, input mtg.ReportConsumeRequest, handler mtg.ReportHandler, options ...socialhub.CallOption) (mtg.ReportDelivery, error) {
	return callValue(ctx, "report_consume", options, func(callCtx context.Context) (mtg.ReportDelivery, error) {
		return service.client.sdk.Reports().Consume(callCtx, input, handler)
	})
}

// ReportStream maps Mintegral TSV decoding and transport failures while
// preserving io.EOF as the normal end-of-stream signal.
type ReportStream struct {
	upstream *mtg.ReportStream
	cancel   context.CancelFunc
}

func (stream *ReportStream) Next() (mtg.ReportRow, error) {
	if stream == nil || stream.upstream == nil {
		return mtg.ReportRow{}, mapError("report_next", io.ErrClosedPipe)
	}
	row, err := stream.upstream.Next()
	if errors.Is(err, io.EOF) {
		return row, io.EOF
	}
	if err != nil {
		return mtg.ReportRow{}, mapError("report_next", err)
	}
	return row, nil
}

func (stream *ReportStream) Close() error {
	if stream == nil {
		return nil
	}
	var err error
	if stream.upstream != nil {
		err = stream.upstream.Close()
	}
	if stream.cancel != nil {
		stream.cancel()
	}
	return mapError("report_close", err)
}

var _ ReportsWorkflow = (*ReportsService)(nil)
