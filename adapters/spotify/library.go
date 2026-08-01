package spotify

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

func (c *Client) ListSavedTracks(ctx context.Context, input SavedTracksRequest, options ...socialhub.CallOption) (socialhub.Page[SavedTrack], error) {
	if err := c.requireScopes("list_saved_tracks", ScopeUserLibraryRead); err != nil {
		return socialhub.Page[SavedTrack]{}, err
	}
	if !validMarket(input.Market) {
		return socialhub.Page[SavedTrack]{}, invalidArgument("list_saved_tracks", "market must be an uppercase ISO 3166-1 alpha-2 code")
	}
	query, err := pageQuery("list_saved_tracks", input.Cursor, input.MaxResults, 50)
	if err != nil {
		return socialhub.Page[SavedTrack]{}, err
	}
	if input.Market != "" {
		query.Set("market", input.Market)
	}
	var response spotifyPage[spotifySavedTrack]
	if _, err := c.requestJSON(ctx, http.MethodGet, "/me/tracks", query, nil, &response, options...); err != nil {
		return socialhub.Page[SavedTrack]{}, err
	}
	items := make([]SavedTrack, 0, len(response.Items))
	for _, item := range response.Items {
		track, err := mapTrack(item.Track)
		if err != nil {
			return socialhub.Page[SavedTrack]{}, err
		}
		items = append(items, SavedTrack{AddedAt: item.AddedAt, Track: track})
	}
	next, err := pageCursor(response.Next, c.apiBaseURL)
	if err != nil {
		return socialhub.Page[SavedTrack]{}, err
	}
	previous, err := pageCursor(response.Previous, c.apiBaseURL)
	if err != nil {
		return socialhub.Page[SavedTrack]{}, err
	}
	return socialhub.Page[SavedTrack]{Items: items, NextCursor: next, PrevCursor: previous, HasMore: next != nil}, nil
}

func (c *Client) SaveItems(ctx context.Context, uris []string, options ...socialhub.CallOption) error {
	required, err := libraryScopes("save_items", uris, true)
	if err != nil {
		return err
	}
	if err := c.requireScopes("save_items", required...); err != nil {
		return err
	}
	_, err = c.requestJSON(ctx, http.MethodPut, "/me/library", url.Values{"uris": {strings.Join(uris, ",")}}, nil, nil, options...)
	return err
}

func (c *Client) RemoveItems(ctx context.Context, uris []string, options ...socialhub.CallOption) error {
	required, err := libraryScopes("remove_items", uris, true)
	if err != nil {
		return err
	}
	if err := c.requireScopes("remove_items", required...); err != nil {
		return err
	}
	_, err = c.requestJSON(ctx, http.MethodDelete, "/me/library", url.Values{"uris": {strings.Join(uris, ",")}}, nil, nil, options...)
	return err
}

func (c *Client) ContainsItems(ctx context.Context, uris []string, options ...socialhub.CallOption) ([]bool, error) {
	required, err := libraryScopes("contains_items", uris, false)
	if err != nil {
		return nil, err
	}
	if err := c.requireScopes("contains_items", required...); err != nil {
		return nil, err
	}
	var response []bool
	if _, err := c.requestJSON(ctx, http.MethodGet, "/me/library/contains", url.Values{"uris": {strings.Join(uris, ",")}}, nil, &response, options...); err != nil {
		return nil, err
	}
	if len(response) != len(uris) {
		return nil, mappingError("contains_items", "Spotify library membership count did not match the request")
	}
	return response, nil
}

func libraryScopes(operation string, uris []string, modify bool) ([]string, error) {
	if len(uris) == 0 || len(uris) > 40 {
		return nil, invalidArgument(operation, "between 1 and 40 Spotify URIs are required")
	}
	seenURIs := make(map[string]struct{}, len(uris))
	seenScopes := make(map[string]struct{}, 3)
	var required []string
	for _, uri := range uris {
		typeName, ok := spotifyURIType(uri)
		if !ok || (modify && typeName == "artist") {
			return nil, invalidArgument(operation, "one or more Spotify library URIs are unsupported")
		}
		if _, exists := seenURIs[uri]; exists {
			return nil, invalidArgument(operation, "Spotify library URIs must not contain duplicates")
		}
		seenURIs[uri] = struct{}{}
		var scope string
		switch typeName {
		case "artist", "user":
			if modify {
				scope = ScopeUserFollowModify
			} else {
				scope = ScopeUserFollowRead
			}
		case "playlist":
			if modify {
				scope = ScopePlaylistModifyPublic
			} else {
				scope = ScopePlaylistReadPrivate
			}
		default:
			if modify {
				scope = ScopeUserLibraryModify
			} else {
				scope = ScopeUserLibraryRead
			}
		}
		if _, exists := seenScopes[scope]; !exists {
			seenScopes[scope] = struct{}{}
			required = append(required, scope)
		}
	}
	return required, nil
}

var _ LibraryWorkflow = (*Client)(nil)
