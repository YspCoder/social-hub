package tvmaze

import (
	"context"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

func (c *Client) SearchPeople(ctx context.Context, query string, options ...socialhub.CallOption) ([]PersonSearchResult, error) {
	if !validQuery(query) {
		return nil, invalidArgument("search_people", "query must be a nonempty bounded value without surrounding whitespace")
	}
	values := url.Values{"q": {query}}
	var results []PersonSearchResult
	_, err := requestJSON(ctx, c.api, "search_people", "/search/people", values, &results, options...)
	return results, err
}

func (c *Client) GetPerson(ctx context.Context, personID int64, options ...socialhub.CallOption) (*Person, error) {
	if personID <= 0 {
		return nil, invalidArgument("get_person", "person_id must be positive")
	}
	var person Person
	_, err := requestJSON(ctx, c.api, "get_person", "/people/"+strconv.FormatInt(personID, 10), nil, &person, options...)
	return &person, err
}

func (c *Client) ListCastCredits(ctx context.Context, personID int64, options ...socialhub.CallOption) ([]CastCredit, error) {
	if personID <= 0 {
		return nil, invalidArgument("list_cast_credits", "person_id must be positive")
	}
	query := url.Values{"embed": {"show"}}
	var credits []CastCredit
	_, err := requestJSON(ctx, c.api, "list_cast_credits", "/people/"+strconv.FormatInt(personID, 10)+"/castcredits", query, &credits, options...)
	return credits, err
}

func (c *Client) ListCrewCredits(ctx context.Context, personID int64, options ...socialhub.CallOption) ([]CrewCredit, error) {
	if personID <= 0 {
		return nil, invalidArgument("list_crew_credits", "person_id must be positive")
	}
	query := url.Values{"embed": {"show"}}
	var credits []CrewCredit
	_, err := requestJSON(ctx, c.api, "list_crew_credits", "/people/"+strconv.FormatInt(personID, 10)+"/crewcredits", query, &credits, options...)
	return credits, err
}
