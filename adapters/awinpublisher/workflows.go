package awinpublisher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListPrograms(
	ctx context.Context,
	input ListProgramsRequest,
	options ...socialhub.CallOption,
) (ProgramsResponse, error) {
	const operation = "list_programs"
	if !validListPrograms(input) {
		return ProgramsResponse{}, invalidArgument(operation, "country code, hidden-program flag, or relationship is invalid")
	}
	query := make(url.Values)
	setOptionalQuery(query, "countryCode", input.CountryCode)
	if input.IncludeHidden {
		query.Set("includeHidden", "true")
	}
	setOptionalQuery(query, "relationship", string(input.Relationship))
	var programs []Program
	metadata, raw, err := client.getJSON(ctx, operation, client.publisherPath("/programmes"), query, &programs, options...)
	if err != nil {
		return ProgramsResponse{}, err
	}
	if programs == nil {
		return ProgramsResponse{}, platformContractError(operation, "Awin omitted the program array")
	}
	programIDs := make(map[int64]struct{}, len(programs))
	for _, program := range programs {
		programID, valid := positiveExactID(program.ID)
		if !valid {
			return ProgramsResponse{}, platformContractError(operation, "Awin returned a program without a valid ID")
		}
		if _, duplicate := programIDs[programID]; duplicate {
			return ProgramsResponse{}, platformContractError(operation, "Awin returned a duplicate program ID")
		}
		programIDs[programID] = struct{}{}
	}
	return ProgramsResponse{Programs: programs, Meta: metadata, Raw: raw}, nil
}

func (client *Client) GenerateTrackingLink(
	ctx context.Context,
	input GenerateTrackingLinkRequest,
	options ...socialhub.CallOption,
) (TrackingLink, error) {
	const operation = "generate_tracking_link"
	if !validGenerateTrackingLink(input) {
		return TrackingLink{}, invalidArgument(operation, "advertiser, destination URL, or tracking parameter is invalid")
	}
	payload := struct {
		AdvertiserID   int64                   `json:"advertiserId"`
		DestinationURL string                  `json:"destinationUrl,omitempty"`
		Parameters     *TrackingLinkParameters `json:"parameters,omitempty"`
		Shorten        bool                    `json:"shorten,omitempty"`
	}{
		AdvertiserID: input.AdvertiserID, DestinationURL: input.DestinationURL, Shorten: input.Shorten,
	}
	if input.Parameters != (TrackingLinkParameters{}) {
		parameters := input.Parameters
		payload.Parameters = &parameters
	}
	var output TrackingLink
	metadata, raw, err := client.postJSON(ctx, operation, client.publisherPath("/linkbuilder/generate"), nil, payload, &output, options...)
	if err != nil {
		return TrackingLink{}, err
	}
	if output.URL == "" {
		if output.Description != "" {
			return TrackingLink{}, trackingLinkBusinessError(output.Description, raw)
		}
		return TrackingLink{}, platformContractError(operation, "Awin returned neither a tracking URL nor a business error")
	}
	if !validRequiredWebURL(output.URL) {
		return TrackingLink{}, platformContractError(operation, "Awin returned an invalid tracking URL")
	}
	if input.Shorten && !validRequiredWebURL(output.ShortURL) {
		return TrackingLink{}, platformContractError(operation, "Awin omitted the requested short tracking URL")
	}
	output.Meta = metadata
	output.Raw = raw
	return output, nil
}

func (client *Client) ListTransactions(
	ctx context.Context,
	input ListTransactionsRequest,
	options ...socialhub.CallOption,
) (TransactionsResponse, error) {
	const operation = "list_transactions"
	if !validListTransactions(input) {
		return TransactionsResponse{}, invalidArgument(operation, "date window, advertiser IDs, date type, status, or timezone is invalid")
	}
	timezone := input.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	query := url.Values{
		"startDate": {input.StartDate.Format(time.RFC3339)},
		"endDate":   {input.EndDate.Format(time.RFC3339)},
		"timezone":  {timezone},
	}
	if len(input.AdvertiserIDs) > 0 {
		identifiers := make([]string, len(input.AdvertiserIDs))
		for index, identifier := range input.AdvertiserIDs {
			identifiers[index] = strconv.FormatInt(identifier, 10)
		}
		query.Set("advertiserId", strings.Join(identifiers, ","))
	}
	setOptionalQuery(query, "dateType", string(input.DateType))
	setOptionalQuery(query, "status", string(input.Status))
	if input.ShowBasketProducts {
		query.Set("showBasketProducts", "true")
	}
	var transactions []Transaction
	metadata, raw, err := client.getJSON(ctx, operation, client.publisherPath("/transactions/"), query, &transactions, options...)
	if err != nil {
		return TransactionsResponse{}, err
	}
	if transactions == nil {
		return TransactionsResponse{}, platformContractError(operation, "Awin omitted the transaction array")
	}
	requestedAdvertisers := make(map[int64]struct{}, len(input.AdvertiserIDs))
	for _, advertiserID := range input.AdvertiserIDs {
		requestedAdvertisers[advertiserID] = struct{}{}
	}
	transactionIDs := make(map[string]struct{}, len(transactions))
	for _, transaction := range transactions {
		transactionID := transaction.ID.String()
		advertiserID, validAdvertiser := positiveExactID(transaction.AdvertiserID)
		publisherID, validPublisher := positiveExactID(transaction.PublisherID)
		if !validExactIdentifier(transaction.ID) || !validAdvertiser || !validPublisher || publisherID != client.publisherID {
			return TransactionsResponse{}, platformContractError(operation, "Awin returned a transaction with invalid or mismatched identity fields")
		}
		if len(requestedAdvertisers) > 0 {
			if _, requested := requestedAdvertisers[advertiserID]; !requested {
				return TransactionsResponse{}, platformContractError(operation, "Awin returned a transaction for an unrequested advertiser")
			}
		}
		if _, duplicate := transactionIDs[transactionID]; duplicate {
			return TransactionsResponse{}, platformContractError(operation, "Awin returned a duplicate transaction ID")
		}
		transactionIDs[transactionID] = struct{}{}
	}
	return TransactionsResponse{Transactions: transactions, Meta: metadata, Raw: raw}, nil
}

func (client *Client) GetAdvertiserPerformance(
	ctx context.Context,
	input GetAdvertiserPerformanceRequest,
	options ...socialhub.CallOption,
) (AdvertiserPerformanceResponse, error) {
	const operation = "get_advertiser_performance"
	if !validGetAdvertiserPerformance(input) {
		return AdvertiserPerformanceResponse{}, invalidArgument(operation, "dates, region, date type, or timezone is invalid")
	}
	timezone := input.Timezone
	if timezone == "" {
		timezone = "UTC"
	}
	query := url.Values{
		"startDate": {string(input.StartDate)},
		"endDate":   {string(input.EndDate)},
		"region":    {string(input.Region)},
		"timezone":  {timezone},
	}
	setOptionalQuery(query, "dateType", string(input.DateType))
	var wire json.RawMessage
	metadata, raw, err := client.getJSON(ctx, operation, client.publisherPath("/reports/advertiser"), query, &wire, options...)
	if err != nil {
		return AdvertiserPerformanceResponse{}, err
	}
	rows, err := decodeAdvertiserPerformance(raw)
	if err != nil {
		return AdvertiserPerformanceResponse{}, platformContractError(operation, err.Error())
	}
	if rows == nil {
		return AdvertiserPerformanceResponse{}, platformContractError(operation, "Awin omitted the advertiser performance array")
	}
	advertiserIDs := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		advertiserID, validAdvertiser := positiveExactID(row.AdvertiserID)
		publisherID, validPublisher := positiveExactID(row.PublisherID)
		if !validAdvertiser || !validPublisher || publisherID != client.publisherID ||
			(row.Region != "" && row.Region != string(input.Region)) {
			return AdvertiserPerformanceResponse{}, platformContractError(operation, "Awin returned advertiser performance with invalid or mismatched identity fields")
		}
		if _, duplicate := advertiserIDs[advertiserID]; duplicate {
			return AdvertiserPerformanceResponse{}, platformContractError(operation, "Awin returned duplicate advertiser performance rows")
		}
		advertiserIDs[advertiserID] = struct{}{}
	}
	return AdvertiserPerformanceResponse{Rows: rows, Meta: metadata, Raw: raw}, nil
}

func decodeAdvertiserPerformance(data []byte) ([]AdvertiserPerformance, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, json.Unmarshal(trimmed, &[]AdvertiserPerformance{})
	}
	if trimmed[0] == '[' {
		var rows []AdvertiserPerformance
		if err := json.Unmarshal(trimmed, &rows); err != nil {
			return nil, err
		}
		return rows, nil
	}
	var wrapper struct {
		Body json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(trimmed, &wrapper); err != nil {
		return nil, err
	}
	body := bytes.TrimSpace(wrapper.Body)
	if len(body) == 0 || body[0] != '[' {
		return nil, errors.New("Awin advertiser performance wrapper omitted its array body")
	}
	var rows []AdvertiserPerformance
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (client *Client) publisherPath(suffix string) string {
	return "/publishers/" + strconv.FormatInt(client.publisherID, 10) + suffix
}

func setOptionalQuery(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

var _ PublisherWorkflow = (*Client)(nil)
