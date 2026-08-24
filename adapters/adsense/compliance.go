package adsense

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

type ComplianceWorkflow interface {
	ListAlerts(context.Context, string, ...socialhub.CallOption) ([]Alert, error)
	ListPayments(context.Context, ...socialhub.CallOption) ([]Payment, error)
	GetPolicyIssue(context.Context, string, ...socialhub.CallOption) (*PolicyIssue, error)
	ListPolicyIssues(context.Context, ListRequest, ...socialhub.CallOption) (Page[PolicyIssue], error)
}

func (client *Client) ListAlerts(ctx context.Context, languageCode string, options ...socialhub.CallOption) ([]Alert, error) {
	const operation = "alerts_list"
	if !validLanguageCode(languageCode) {
		return nil, invalidArgument(operation, "language code is invalid")
	}
	query := make(url.Values)
	if languageCode != "" {
		query.Set("languageCode", languageCode)
	}
	var output struct {
		Alerts []Alert `json:"alerts"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/alerts", query, &output, options...); err != nil {
		return nil, err
	}
	for _, item := range output.Alerts {
		if !client.ownsResource(item.Name, client.accountName(), "alerts") {
			return nil, ownershipError(operation, "alert")
		}
	}
	return output.Alerts, nil
}

func (client *Client) ListPayments(ctx context.Context, options ...socialhub.CallOption) ([]Payment, error) {
	const operation = "payments_list"
	var output struct {
		Payments []Payment `json:"payments"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/payments", nil, &output, options...); err != nil {
		return nil, err
	}
	for _, item := range output.Payments {
		if !client.ownsResource(item.Name, client.accountName(), "payments") {
			return nil, ownershipError(operation, "payment")
		}
	}
	return output.Payments, nil
}

func (client *Client) GetPolicyIssue(ctx context.Context, issueID string, options ...socialhub.CallOption) (*PolicyIssue, error) {
	const operation = "policy_issue_get"
	name, err := client.resourceName(operation, client.accountName(), "policyIssues", issueID)
	if err != nil {
		return nil, err
	}
	var output PolicyIssue
	if err := client.getJSON(ctx, operation, "/v2/"+name, nil, &output, options...); err != nil {
		return nil, err
	}
	if output.Name != name {
		return nil, ownershipError(operation, "policy issue")
	}
	if !validPolicyIssue(output) {
		return nil, platformContractError(operation, "AdSense returned an invalid policy issue")
	}
	return &output, nil
}

func (client *Client) ListPolicyIssues(ctx context.Context, input ListRequest, options ...socialhub.CallOption) (Page[PolicyIssue], error) {
	const operation = "policy_issues_list"
	query, err := listQuery(operation, input)
	if err != nil {
		return Page[PolicyIssue]{}, err
	}
	var output struct {
		PolicyIssues  []PolicyIssue `json:"policyIssues"`
		NextPageToken string        `json:"nextPageToken"`
	}
	if err := client.getJSON(ctx, operation, "/v2/"+client.accountName()+"/policyIssues", query, &output, options...); err != nil {
		return Page[PolicyIssue]{}, err
	}
	for _, item := range output.PolicyIssues {
		if !client.ownsResource(item.Name, client.accountName(), "policyIssues") {
			return Page[PolicyIssue]{}, ownershipError(operation, "policy issue")
		}
		if !validPolicyIssue(item) {
			return Page[PolicyIssue]{}, platformContractError(operation, "AdSense returned an invalid policy issue")
		}
	}
	return Page[PolicyIssue]{Items: output.PolicyIssues, NextPageToken: output.NextPageToken}, nil
}

func validPolicyIssue(value PolicyIssue) bool {
	count, err := strconv.ParseInt(value.AdRequestCount, 10, 64)
	if err != nil || count < 0 || !validDate(value.FirstDetectedDate) || !validDate(value.LastDetectedDate) {
		return false
	}
	if dateTime(value.LastDetectedDate).Before(dateTime(value.FirstDetectedDate)) {
		return false
	}
	if !zeroDate(value.WarningEscalationDate) && !validDate(value.WarningEscalationDate) {
		return false
	}
	for _, name := range value.AdClients {
		// Associated host/secondary ad clients may belong to another account.
		if !validAdClientName(name) {
			return false
		}
	}
	return true
}
