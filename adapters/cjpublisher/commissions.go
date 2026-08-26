package cjpublisher

import (
	"context"
	"time"

	"social-hub/pkg/socialhub"
)

const listPublisherCommissionsQuery = `
query ListCJPublisherCommissions(
  $forPublishers: [String!]!,
  $sincePostingDate: String, $beforePostingDate: String,
  $sinceEventDate: String, $beforeEventDate: String,
  $sinceLockingDate: String, $beforeLockingDate: String,
  $sinceCommissionId: String, $commissionIds: [String!],
  $advertiserIds: [String!], $adIds: [String!], $websiteIds: [String!],
  $actionStatuses: [String!], $actionTypes: [String!],
  $lockingMethods: [LockingMethod!], $validationStatuses: [ValidationStatus!]
) {
  publisherCommissions(
    forPublishers: $forPublishers,
    sincePostingDate: $sincePostingDate, beforePostingDate: $beforePostingDate,
    sinceEventDate: $sinceEventDate, beforeEventDate: $beforeEventDate,
    sinceLockingDate: $sinceLockingDate, beforeLockingDate: $beforeLockingDate,
    sinceCommissionId: $sinceCommissionId, commissionIds: $commissionIds,
    advertiserIds: $advertiserIds, adIds: $adIds, websiteIds: $websiteIds,
    actionStatuses: $actionStatuses, actionTypes: $actionTypes,
    lockingMethods: $lockingMethods, validationStatuses: $validationStatuses
  ) {
    count
    limit
    maxCommissionId
    payloadComplete
    records {
      actionStatus
      actionTrackerId
      actionTrackerName
      actionType
      advertiserId
      advertiserName
      aid
      clickDate
      commissionId
      correctionReason
      country
      coupon
      eventDate
      isCrossDevice
      lockingDate
      lockingMethod
      orderId
      validationStatus
      original
      originalActionId
      postingDate
      pubCommissionAmountPubCurrency
      pubCommissionAmountUsd
      publisherId
      publisherName
      saleAmountPubCurrency
      saleAmountUsd
      source
      websiteId
      websiteName
      shopperId
      items {
        commissionItemId
        discountUsd
        discountPubCurrency
        itemListId
        totalCommissionPubCurrency
        totalCommissionUsd
        quantity
        perItemSaleAmountPubCurrency
        perItemSaleAmountUsd
        sku
      }
    }
  }
}`

func (client *Client) ListPublisherCommissions(
	ctx context.Context,
	input ListPublisherCommissionsRequest,
	options ...socialhub.CallOption,
) (CommissionsResponse, error) {
	const operation = "list_publisher_commissions"
	if !validListPublisherCommissions(input) {
		return CommissionsResponse{}, invalidArgument(operation, "commission cursor, filters, enums, or 31-day date window is invalid")
	}
	variables := map[string]any{"forPublishers": []string{client.publisherID}}
	setTimeVariable(variables, "sincePostingDate", input.SincePostingDate)
	setTimeVariable(variables, "beforePostingDate", input.BeforePostingDate)
	setTimeVariable(variables, "sinceEventDate", input.SinceEventDate)
	setTimeVariable(variables, "beforeEventDate", input.BeforeEventDate)
	setTimeVariable(variables, "sinceLockingDate", input.SinceLockingDate)
	setTimeVariable(variables, "beforeLockingDate", input.BeforeLockingDate)
	setVariable(variables, "sinceCommissionId", input.SinceCommissionID)
	setStringSliceVariable(variables, "commissionIds", input.CommissionIDs)
	setStringSliceVariable(variables, "advertiserIds", input.AdvertiserIDs)
	setStringSliceVariable(variables, "adIds", input.AdIDs)
	setStringSliceVariable(variables, "websiteIds", input.WebsiteIDs)
	setStringSliceVariable(variables, "actionStatuses", input.ActionStatuses)
	setStringSliceVariable(variables, "actionTypes", input.ActionTypes)
	if len(input.LockingMethods) > 0 {
		values := make([]string, len(input.LockingMethods))
		for index, value := range input.LockingMethods {
			values[index] = string(value)
		}
		variables["lockingMethods"] = values
	}
	if len(input.ValidationStatuses) > 0 {
		values := make([]string, len(input.ValidationStatuses))
		for index, value := range input.ValidationStatuses {
			values[index] = string(value)
		}
		variables["validationStatuses"] = values
	}
	var data struct {
		Commissions CommissionsResponse `json:"publisherCommissions"`
	}
	meta, raw, err := client.doGraphQL(
		ctx, client.commissionsAPI, operation, listPublisherCommissionsQuery, variables, &data, options...,
	)
	data.Commissions.Meta, data.Commissions.Raw = meta, raw
	if err == nil {
		err = validateCommissionsResponse(operation, data.Commissions, client.publisherID)
	}
	return data.Commissions, err
}

func setTimeVariable(variables map[string]any, key string, value time.Time) {
	if !value.IsZero() {
		variables[key] = value.UTC().Format(time.RFC3339)
	}
}
