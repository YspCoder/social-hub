package gitlab

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

// ListIssueNotes returns system activity and comments for one project issue.
func (client *Client) ListIssueNotes(
	ctx context.Context,
	projectID string,
	issueIID string,
	input ListIssueNotesRequest,
	options ...socialhub.CallOption,
) (*Page[Note], error) {
	const operation = "list_issue_notes"
	if !validDecimalID(projectID) || !validDecimalID(issueIID) || !validListIssueNotes(input) {
		return nil, invalidArgument(operation, "project ID, issue IID, activity filter, ordering, or pagination is invalid")
	}
	query := make(url.Values)
	setQuery(query, "activity_filter", string(input.ActivityFilter))
	setQuery(query, "order_by", string(input.OrderBy))
	setQuery(query, "sort", string(input.Sort))
	setPaginationQuery(query, input.PerPage, input.Page)
	var notes []Note
	meta, raw, err := client.getJSON(ctx, operation, issueNotesPath(projectID, issueIID), query, '[', &notes, options...)
	if err != nil {
		return nil, err
	}
	for _, note := range notes {
		if !validNote(note) || string(note.ProjectID) != projectID || string(note.NoteableIID) != issueIID {
			return nil, platformContractError(operation, "GitLab returned a note without valid or matching project, issue, or note IDs")
		}
	}
	return buildPage(operation, notes, meta, raw)
}

// GetIssueNote returns one issue note by global note ID.
func (client *Client) GetIssueNote(
	ctx context.Context,
	projectID string,
	issueIID string,
	noteID string,
	options ...socialhub.CallOption,
) (*Note, ResponseMeta, error) {
	const operation = "get_issue_note"
	if !validDecimalID(projectID) || !validDecimalID(issueIID) || !validDecimalID(noteID) {
		return nil, ResponseMeta{}, invalidArgument(operation, "project ID, issue IID, or note ID is invalid")
	}
	var note Note
	path := issueNotesPath(projectID, issueIID) + "/" + noteID
	meta, _, err := client.getJSON(ctx, operation, path, nil, '{', &note, options...)
	if err != nil {
		return nil, meta, err
	}
	if !validNote(note) || string(note.ID) != noteID || string(note.ProjectID) != projectID || string(note.NoteableIID) != issueIID {
		return nil, meta, platformContractError(operation, "GitLab returned an absent or mismatched note, project, or issue ID")
	}
	return &note, meta, nil
}

func issueNotesPath(projectID, issueIID string) string {
	return projectPath(projectID) + "/issues/" + issueIID + "/notes"
}

var _ ReadWorkflow = (*Client)(nil)
