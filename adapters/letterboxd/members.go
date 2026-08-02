package letterboxd

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (c *Client) GetMember(ctx context.Context, id string, options ...socialhub.CallOption) (*Member, error) {
	if err := c.requireToken("get_member"); err != nil {
		return nil, err
	}
	if !validIdentifier(id) {
		return nil, invalidArgument("get_member", "member ID is invalid")
	}
	var response Member
	if err := c.requestJSON(ctx, http.MethodGet, "/member/"+escaped(id), nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) GetMe(ctx context.Context, options ...socialhub.CallOption) (*MemberAccount, error) {
	if err := c.requireUser("get_me"); err != nil {
		return nil, err
	}
	var response MemberAccount
	if err := c.requestJSON(ctx, http.MethodGet, "/me", nil, nil, &response, options...); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) ListActivity(ctx context.Context, memberID string, input PageRequest, options ...socialhub.CallOption) (socialhub.Page[ActivityItem], error) {
	if err := c.requireToken("list_activity"); err != nil {
		return socialhub.Page[ActivityItem]{}, err
	}
	if !validIdentifier(memberID) || !validPage(input.Cursor, input.PerPage) {
		return socialhub.Page[ActivityItem]{}, invalidArgument("list_activity", "member ID or pagination is invalid")
	}
	var response pageEnvelope[ActivityItem]
	if err := c.requestJSON(ctx, http.MethodGet, "/member/"+escaped(memberID)+"/activity", pageQuery(input.Cursor, input.PerPage), nil, &response, options...); err != nil {
		return socialhub.Page[ActivityItem]{}, err
	}
	return toPage(response.Items, response.Next), nil
}

func (c *Client) ListWatchlist(ctx context.Context, memberID string, input FilmListRequest, options ...socialhub.CallOption) (socialhub.Page[FilmSummary], error) {
	if err := c.requireToken("list_watchlist"); err != nil {
		return socialhub.Page[FilmSummary]{}, err
	}
	if !validIdentifier(memberID) {
		return socialhub.Page[FilmSummary]{}, invalidArgument("list_watchlist", "member ID is invalid")
	}
	query, err := filmListQuery("list_watchlist", input)
	if err != nil {
		return socialhub.Page[FilmSummary]{}, err
	}
	var response pageEnvelope[FilmSummary]
	if err := c.requestJSON(ctx, http.MethodGet, "/member/"+escaped(memberID)+"/watchlist", query, nil, &response, options...); err != nil {
		return socialhub.Page[FilmSummary]{}, err
	}
	return toPage(response.Items, response.Next), nil
}
