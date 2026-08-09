package admob

import (
	"context"

	"social-hub/pkg/socialhub"
)

type ReportingWorkflow interface {
	GenerateNetworkReport(context.Context, NetworkReportSpec, ...socialhub.CallOption) (*Report, error)
	GenerateMediationReport(context.Context, MediationReportSpec, ...socialhub.CallOption) (*Report, error)
}

func (client *Client) GenerateNetworkReport(ctx context.Context, spec NetworkReportSpec, options ...socialhub.CallOption) (*Report, error) {
	const operation = "network_report_generate"
	if err := client.requireScope(operation, readOnlyScope, reportScope); err != nil {
		return nil, err
	}
	if !validNetworkReportSpec(spec) {
		return nil, invalidArgument(operation, "network report date range, fields, filters, sort, localization, time zone, or row limit is invalid")
	}
	expected := reportExpectation{
		DateRange: spec.DateRange, Dimensions: spec.Dimensions, Metrics: spec.Metrics,
		Localization: spec.LocalizationSettings, TimeZone: spec.TimeZone, MaximumRows: spec.MaxReportRows,
	}
	return client.postReport(ctx, operation, "/v1/"+client.accountName()+"/networkReport:generate",
		GenerateNetworkReportRequest{ReportSpec: spec}, expected, options...)
}

func (client *Client) GenerateMediationReport(ctx context.Context, spec MediationReportSpec, options ...socialhub.CallOption) (*Report, error) {
	const operation = "mediation_report_generate"
	if err := client.requireScope(operation, readOnlyScope, reportScope); err != nil {
		return nil, err
	}
	if !validMediationReportSpec(spec) {
		return nil, invalidArgument(operation, "mediation report date range, fields, filters, sort, localization, time zone, or row limit is invalid")
	}
	expected := reportExpectation{
		DateRange: spec.DateRange, Dimensions: spec.Dimensions, Metrics: spec.Metrics,
		Localization: spec.LocalizationSettings, TimeZone: spec.TimeZone, MaximumRows: spec.MaxReportRows,
	}
	return client.postReport(ctx, operation, "/v1/"+client.accountName()+"/mediationReport:generate",
		GenerateMediationReportRequest{ReportSpec: spec}, expected, options...)
}
