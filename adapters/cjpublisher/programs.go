package cjpublisher

import (
	"context"

	"social-hub/pkg/socialhub"
)

const listProgramTermsQuery = `
query ListCJProgramTerms(
  $publisherId: ID!, $offset: Int, $limit: Int, $filters: ContractFilters
) {
  publisher {
    contracts(publisherId: $publisherId, offset: $offset, limit: $limit, filters: $filters) {
      totalCount
      count
      resultList {
        startTime
        endTime
        status
        advertiserId
        programTerms {
          id
          name
          specialTerms { name body }
          isDefault
          actionTerms {
            id
            actionTracker { id name description type }
            referralPeriod
            referralOccurrences
            lockingMethod { type durationInDays }
            performanceIncentives {
              threshold { type value }
              reward { type commissionType value }
              currency
            }
            commissions {
              rank
              situation { id name }
              itemList { id name }
              promotionalProperties { id name }
              isViewThrough
              rate { type value currency }
            }
          }
        }
      }
    }
  }
}`

func (client *Client) ListProgramTerms(
	ctx context.Context,
	input ListProgramTermsRequest,
	options ...socialhub.CallOption,
) (ProgramTermsResponse, error) {
	const operation = "list_program_terms"
	if !validListProgramTerms(input) {
		return ProgramTermsResponse{}, invalidArgument(operation, "advertiser, active date range, offset, or limit is invalid")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 10
	}
	variables := map[string]any{
		"publisherId": client.publisherID,
		"offset":      input.Offset,
		"limit":       limit,
	}
	filters := make(map[string]any)
	setVariable(filters, "advertiserId", input.AdvertiserID)
	setTimeVariable(filters, "activeAfter", input.ActiveAfter)
	setTimeVariable(filters, "activeBefore", input.ActiveBefore)
	if len(filters) > 0 {
		variables["filters"] = filters
	}
	var data struct {
		Publisher struct {
			Contracts ProgramTermsResponse `json:"contracts"`
		} `json:"publisher"`
	}
	meta, raw, err := client.doGraphQL(ctx, client.programsAPI, operation, listProgramTermsQuery, variables, &data, options...)
	data.Publisher.Contracts.Meta, data.Publisher.Contracts.Raw = meta, raw
	if err == nil {
		err = validateProgramTermsResponse(operation, data.Publisher.Contracts, input.AdvertiserID, limit)
	}
	return data.Publisher.Contracts, err
}
