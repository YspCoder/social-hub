package appleads

import (
	"context"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListACL(ctx context.Context, pagination Pagination, options ...socialhub.CallOption) (Page[UserACL], error) {
	const operation = "acl_list"
	if !validPagination(pagination) {
		return Page[UserACL]{}, invalidArgument(operation, "pagination must use offset >= 0 and limit 1..1000")
	}
	var response responseEnvelope[[]UserACL]
	if err := client.getJSON(ctx, operation, "/acls", listQuery(pagination), &response, options...); err != nil {
		return Page[UserACL]{}, err
	}
	if err := checkEnvelopeError(operation, response.Error); err != nil {
		return Page[UserACL]{}, err
	}
	for _, acl := range response.Data {
		if !validID(acl.OrgID) || acl.OrgName == "" {
			return Page[UserACL]{}, platformContractError(operation, "ACL response contained an invalid organization")
		}
	}
	return pageResult(response.Data, response.Pagination), nil
}
