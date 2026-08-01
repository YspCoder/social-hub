package stackexchange

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"social-hub/pkg/socialhub"
)

func (client *Client) CreateQuestion(ctx context.Context, input CreateQuestionRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	tags, err := validateQuestion(input)
	if err != nil {
		return nil, err
	}
	if err := client.requireWrite("create_question"); err != nil {
		return nil, err
	}
	response, err := call[PostDetails](client, ctx, "questions_add", http.MethodPost, "/questions/add", nil, url.Values{
		"title": {input.Title}, "body": {input.Body}, "tags": {strings.Join(tags, ";")},
	}, options...)
	if err != nil {
		return nil, err
	}
	if len(response.Items) == 0 || firstPositive(response.Items[0].PostID, response.Items[0].QuestionID) <= 0 {
		return nil, platformError("create_question", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response.Items[0], client.clock.Now()), nil
}

func (client *Client) CreateAnswer(ctx context.Context, input CreateAnswerRequest, options ...socialhub.CallOption) (*socialhub.Post, error) {
	if !validID(input.QuestionID) || utf8.RuneCountInString(strings.TrimSpace(input.Body)) < 30 {
		return nil, invalidArgument("create_answer", "question ID and an answer body of at least 30 characters are required")
	}
	if err := client.requireWrite("create_answer"); err != nil {
		return nil, err
	}
	response, err := call[PostDetails](client, ctx, "answers_add", http.MethodPost, "/questions/"+input.QuestionID+"/answers/add", nil, url.Values{"body": {input.Body}}, options...)
	if err != nil {
		return nil, err
	}
	if len(response.Items) == 0 || firstPositive(response.Items[0].PostID, response.Items[0].AnswerID) <= 0 {
		return nil, platformError("create_answer", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapPost(client.accountID, response.Items[0], client.clock.Now()), nil
}

func (client *Client) SearchQuestions(ctx context.Context, input SearchQuestionsRequest, options ...socialhub.CallOption) (socialhub.Page[socialhub.Post], error) {
	queryText := strings.TrimSpace(input.Query)
	tags, err := normalizeTags(input.Tagged)
	if err != nil || (queryText == "" && len(tags) == 0) {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("search_questions", "query or one to five valid tags are required")
	}
	sort := input.Sort
	if sort == "" {
		sort = "relevance"
	}
	if sort != "activity" && sort != "creation" && sort != "votes" && sort != "relevance" {
		return socialhub.Page[socialhub.Post]{}, invalidArgument("search_questions", "sort must be activity, creation, votes, or relevance")
	}
	query, page, err := pageQuery(input.Cursor, input.MaxResults)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	query.Set("filter", "withbody")
	query.Set("order", "desc")
	query.Set("sort", sort)
	if queryText != "" {
		query.Set("q", queryText)
	}
	if len(tags) > 0 {
		query.Set("tagged", strings.Join(tags, ";"))
	}
	response, err := call[PostDetails](client, ctx, "search_advanced", http.MethodGet, "/search/advanced", query, nil, options...)
	if err != nil {
		return socialhub.Page[socialhub.Post]{}, err
	}
	items := make([]socialhub.Post, 0, len(response.Items))
	observedAt := client.clock.Now()
	for _, item := range response.Items {
		if firstPositive(item.PostID, item.QuestionID) > 0 {
			items = append(items, *mapPost(client.accountID, item, observedAt))
		}
	}
	return pageFrom(items, page, response.HasMore), nil
}

func validateQuestion(input CreateQuestionRequest) ([]string, error) {
	titleLength := utf8.RuneCountInString(strings.TrimSpace(input.Title))
	bodyLength := utf8.RuneCountInString(strings.TrimSpace(input.Body))
	tags, err := normalizeTags(input.Tags)
	if err != nil || titleLength < 15 || titleLength > 150 || bodyLength < 30 || len(tags) == 0 {
		return nil, invalidArgument("create_question", "title must be 15-150 characters, body at least 30 characters, and one to five valid tags are required")
	}
	return tags, nil
}

func normalizeTags(input []string) ([]string, error) {
	if len(input) > 5 {
		return nil, invalidArgument("tags", "at most five tags are supported")
	}
	tags := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, value := range input {
		tag := strings.TrimSpace(value)
		if tag == "" || utf8.RuneCountInString(tag) > 35 || strings.ContainsAny(tag, ";\r\n") {
			return nil, invalidArgument("tags", "tags must be non-empty, at most 35 characters, and must not contain separators")
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		tags = append(tags, tag)
	}
	return tags, nil
}
