package dribbble

import (
	"context"
	"net/http"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListProjects(ctx context.Context, cursor string, maximum int, options ...socialhub.CallOption) (socialhub.Page[Project], error) {
	if err := client.requireScopes("list_projects", "public"); err != nil {
		return socialhub.Page[Project]{}, err
	}
	query, err := pageQuery(cursor, maximum)
	if err != nil {
		return socialhub.Page[Project]{}, err
	}
	var projects []Project
	metadata, err := client.requestJSON(ctx, http.MethodGet, "/user/projects", query, nil, &projects, options...)
	if err != nil {
		return socialhub.Page[Project]{}, err
	}
	next, previous := client.pageCursors(metadata.Header, client.baseURL.Path+"/user/projects")
	return socialhub.Page[Project]{Items: projects, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (client *Client) CreateProject(ctx context.Context, input CreateProjectRequest, options ...socialhub.CallOption) (*Project, error) {
	if !validText(input.Name, true, 1000) || !validText(input.Description, false, 20000) {
		return nil, invalidArgument("create_project", "project name and description are invalid")
	}
	if err := client.requireScopes("create_project", "upload"); err != nil {
		return nil, err
	}
	var response Project
	if _, err := client.requestJSON(ctx, http.MethodPost, "/projects", nil, input, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 {
		return nil, platformError("create_project", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) UpdateProject(ctx context.Context, projectID string, input UpdateProjectRequest, options ...socialhub.CallOption) (*Project, error) {
	if !validID(projectID) || input.Name == nil && input.Description == nil {
		return nil, invalidArgument("update_project", "project ID and at least one mutable field are required")
	}
	if input.Name != nil && !validText(*input.Name, true, 1000) || input.Description != nil && !validText(*input.Description, false, 20000) {
		return nil, invalidArgument("update_project", "project name or description is invalid")
	}
	if err := client.requireScopes("update_project", "upload"); err != nil {
		return nil, err
	}
	var response Project
	if _, err := client.requestJSON(ctx, http.MethodPut, "/projects/"+projectID, nil, input, &response, options...); err != nil {
		return nil, err
	}
	if response.ID <= 0 || response.ID != mustID(projectID) {
		return nil, platformError("update_project", socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return &response, nil
}

func (client *Client) DeleteProject(ctx context.Context, projectID string, options ...socialhub.CallOption) error {
	if !validID(projectID) {
		return invalidArgument("delete_project", "project ID must be a positive integer")
	}
	if err := client.requireScopes("delete_project", "upload"); err != nil {
		return err
	}
	_, err := client.requestJSON(ctx, http.MethodDelete, "/projects/"+projectID, nil, nil, nil, options...)
	return err
}
