package youtubereporting

import (
	"context"
	"net/url"
	"strconv"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListReportTypes(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[ReportType], error) {
	const operation = "report_type_list"
	if !validListRequest(input) {
		return Page[ReportType]{}, invalidArgument(operation, "page size or page token is invalid")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return Page[ReportType]{}, err
	}
	query := client.listQuery(input)
	var response listReportTypesResponse
	if err := client.getJSON(ctx, operation, "/v1/reportTypes", query, &response, options...); err != nil {
		return Page[ReportType]{}, err
	}
	if !validReportTypesResponse(response) || input.PageSize > 0 && len(response.ReportTypes) > int(input.PageSize) {
		return Page[ReportType]{}, platformContractError(operation, "YouTube returned invalid report types or pagination metadata")
	}
	return Page[ReportType]{Items: response.ReportTypes, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) CreateJob(ctx context.Context, input CreateJobInput, options ...socialhub.CallOption) (Job, error) {
	const operation = "job_create"
	if !validReportTypeID(input.ReportTypeID) || !validText(input.Name, 100, true) {
		return Job{}, invalidArgument(operation, "report type ID and a name of at most 100 characters are required")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return Job{}, err
	}
	query := make(url.Values)
	client.ownerQuery(query)
	request := struct {
		ReportTypeID string `json:"reportTypeId"`
		Name         string `json:"name"`
	}{ReportTypeID: input.ReportTypeID, Name: input.Name}
	var job Job
	if err := client.postJSON(ctx, operation, "/v1/jobs", query, request, &job, options...); err != nil {
		return Job{}, err
	}
	if !validJob(job) || job.ReportTypeID != input.ReportTypeID || job.Name != input.Name || job.SystemManaged {
		return Job{}, platformContractError(operation, "YouTube returned an invalid or different user-managed job")
	}
	return job, nil
}

func (client *Client) GetJob(ctx context.Context, jobID string, options ...socialhub.CallOption) (Job, error) {
	const operation = "job_get"
	if !validJobID(jobID) {
		return Job{}, invalidArgument(operation, "job ID is invalid")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return Job{}, err
	}
	query := make(url.Values)
	client.ownerQuery(query)
	var job Job
	if err := client.getJSON(ctx, operation, "/v1/jobs/"+jobID, query, &job, options...); err != nil {
		return Job{}, err
	}
	if !validJob(job) || job.ID != jobID {
		return Job{}, platformContractError(operation, "YouTube returned an invalid or different job")
	}
	return job, nil
}

func (client *Client) ListJobs(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[Job], error) {
	const operation = "job_list"
	if !validListRequest(input) {
		return Page[Job]{}, invalidArgument(operation, "page size or page token is invalid")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return Page[Job]{}, err
	}
	query := client.listQuery(input)
	var response listJobsResponse
	if err := client.getJSON(ctx, operation, "/v1/jobs", query, &response, options...); err != nil {
		return Page[Job]{}, err
	}
	if !validJobsResponse(response) || input.PageSize > 0 && len(response.Jobs) > int(input.PageSize) {
		return Page[Job]{}, platformContractError(operation, "YouTube returned invalid jobs or pagination metadata")
	}
	return Page[Job]{Items: response.Jobs, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) DeleteJob(ctx context.Context, jobID string, options ...socialhub.CallOption) error {
	const operation = "job_delete"
	if !validJobID(jobID) {
		return invalidArgument(operation, "job ID is invalid")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return err
	}
	query := make(url.Values)
	client.ownerQuery(query)
	return client.deleteJSON(ctx, operation, "/v1/jobs/"+jobID, query, options...)
}

func (client *Client) ListReports(ctx context.Context, jobID string, input ListReportsRequest, options ...socialhub.CallOption) (Page[Report], error) {
	const operation = "report_list"
	if !validJobID(jobID) || !validListReportsRequest(input) {
		return Page[Report]{}, invalidArgument(operation, "job ID, pagination, or timestamp filters are invalid")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return Page[Report]{}, err
	}
	query := make(url.Values)
	client.ownerQuery(query)
	setPage(query, input.PageSize, input.PageToken)
	setTimestamp(query, "createdAfter", input.CreatedAfter)
	setTimestamp(query, "startTimeAtOrAfter", input.StartTimeAtOrAfter)
	setTimestamp(query, "startTimeBefore", input.StartTimeBefore)
	var response listReportsResponse
	if err := client.getJSON(ctx, operation, "/v1/jobs/"+jobID+"/reports", query, &response, options...); err != nil {
		return Page[Report]{}, err
	}
	if !validReportsResponse(response, jobID) || input.PageSize > 0 && len(response.Reports) > int(input.PageSize) {
		return Page[Report]{}, platformContractError(operation, "YouTube returned invalid reports or pagination metadata")
	}
	return Page[Report]{Items: response.Reports, NextPageToken: response.NextPageToken}, nil
}

func (client *Client) GetReport(ctx context.Context, jobID, reportID string, options ...socialhub.CallOption) (Report, error) {
	const operation = "report_get"
	if !validJobID(jobID) || !validReportID(reportID) {
		return Report{}, invalidArgument(operation, "job ID or report ID is invalid")
	}
	if err := client.requireReportingScope(operation); err != nil {
		return Report{}, err
	}
	query := make(url.Values)
	client.ownerQuery(query)
	var report Report
	path := "/v1/jobs/" + jobID + "/reports/" + reportID
	if err := client.getJSON(ctx, operation, path, query, &report, options...); err != nil {
		return Report{}, err
	}
	if !validReport(report) || report.ID != reportID || report.JobID != jobID {
		return Report{}, platformContractError(operation, "YouTube returned invalid or different report metadata")
	}
	return report, nil
}

func (client *Client) listQuery(input ListRequest) url.Values {
	query := make(url.Values)
	client.ownerQuery(query)
	setPage(query, input.PageSize, input.PageToken)
	if input.IncludeSystemManaged {
		query.Set("includeSystemManaged", "true")
	}
	return query
}

func setPage(query url.Values, pageSize int32, pageToken string) {
	if pageSize > 0 {
		query.Set("pageSize", strconv.FormatInt(int64(pageSize), 10))
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
}

func setTimestamp(query url.Values, name string, value time.Time) {
	if !value.IsZero() {
		query.Set(name, value.UTC().Truncate(time.Microsecond).Format(time.RFC3339Nano))
	}
}

var _ ReportingWorkflow = (*Client)(nil)
