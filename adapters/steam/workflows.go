package steam

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

func (client *Client) GetPlayerSummaries(
	ctx context.Context,
	input GetPlayerSummariesRequest,
	options ...socialhub.CallOption,
) (PlayerSummariesResponse, error) {
	const operation = "get_player_summaries"
	if err := validatePlayerSummaries(input); err != nil {
		return PlayerSummariesResponse{}, err
	}
	api, err := client.requireAuthenticated(operation)
	if err != nil {
		return PlayerSummariesResponse{}, err
	}
	identifiers := make([]string, len(input.SteamIDs))
	requested := make(map[string]struct{}, len(input.SteamIDs))
	for index, steamID := range input.SteamIDs {
		identifiers[index] = string(steamID)
		canonical, _ := canonicalSteamID(steamID)
		requested[canonical] = struct{}{}
	}
	query := url.Values{"steamids": {strings.Join(identifiers, ",")}}
	var envelope struct {
		Response *PlayerSummariesResponse `json:"response"`
	}
	meta, raw, err := client.getJSON(
		ctx, api, operation, "/ISteamUser/GetPlayerSummaries/v2/", query, client.webAPIKey, &envelope, options...,
	)
	if err != nil {
		return PlayerSummariesResponse{}, err
	}
	if envelope.Response == nil || envelope.Response.Players == nil {
		return PlayerSummariesResponse{}, platformContractError(operation, "Steam omitted the response or players array")
	}
	seen := make(map[string]struct{}, len(envelope.Response.Players))
	for _, player := range envelope.Response.Players {
		canonical, valid := canonicalSteamID(player.SteamID)
		if !valid {
			return PlayerSummariesResponse{}, platformContractError(operation, "Steam returned an invalid player SteamID")
		}
		if _, expected := requested[canonical]; !expected {
			return PlayerSummariesResponse{}, platformContractError(operation, "Steam returned an unrequested player SteamID")
		}
		if _, duplicate := seen[canonical]; duplicate {
			return PlayerSummariesResponse{}, platformContractError(operation, "Steam returned a duplicate player SteamID")
		}
		seen[canonical] = struct{}{}
	}
	result := *envelope.Response
	result.Meta, result.Raw = meta, raw
	return result, nil
}

func (client *Client) GetNewsForApp(
	ctx context.Context,
	input GetNewsForAppRequest,
	options ...socialhub.CallOption,
) (AppNewsResponse, error) {
	const operation = "get_news_for_app"
	if err := validateNewsRequest(input); err != nil {
		return AppNewsResponse{}, err
	}
	query := url.Values{"appid": {strconv.FormatUint(uint64(input.AppID), 10)}}
	if input.MaxLength != 0 {
		query.Set("maxlength", strconv.FormatUint(uint64(input.MaxLength), 10))
	}
	if input.EndDate != nil {
		query.Set("enddate", strconv.FormatInt(input.EndDate.Unix(), 10))
	}
	if input.Count != 0 {
		query.Set("count", strconv.FormatUint(uint64(input.Count), 10))
	}
	if len(input.Feeds) > 0 {
		query.Set("feeds", strings.Join(input.Feeds, ","))
	}
	if len(input.Tags) > 0 {
		query.Set("tags", strings.Join(input.Tags, ","))
	}
	var envelope struct {
		AppNews *AppNewsResponse `json:"appnews"`
	}
	meta, raw, err := client.getJSON(
		ctx, client.public, operation, "/ISteamNews/GetNewsForApp/v2/", query, "", &envelope, options...,
	)
	if err != nil {
		return AppNewsResponse{}, err
	}
	if envelope.AppNews == nil || envelope.AppNews.NewsItems == nil {
		return AppNewsResponse{}, platformContractError(operation, "Steam omitted the appnews object or newsitems array")
	}
	if envelope.AppNews.AppID != input.AppID || envelope.AppNews.Count < 0 ||
		len(envelope.AppNews.NewsItems) > effectiveNewsCount(input.Count) ||
		len(envelope.AppNews.NewsItems) > envelope.AppNews.Count {
		return AppNewsResponse{}, platformContractError(operation, "Steam returned an inconsistent app ID or count")
	}
	seen := make(map[string]struct{}, len(envelope.AppNews.NewsItems))
	for _, item := range envelope.AppNews.NewsItems {
		if item.GID == "" || item.AppID != input.AppID {
			return AppNewsResponse{}, platformContractError(operation, "Steam returned a news item without a GID or with an inconsistent app ID")
		}
		if _, duplicate := seen[item.GID]; duplicate {
			return AppNewsResponse{}, platformContractError(operation, "Steam returned a duplicate news item GID")
		}
		seen[item.GID] = struct{}{}
		if !validNewsValues(item.Tags) {
			return AppNewsResponse{}, platformContractError(operation, "Steam returned invalid news tags")
		}
	}
	result := *envelope.AppNews
	result.Meta, result.Raw = meta, raw
	return result, nil
}

func effectiveNewsCount(value uint32) int {
	if value == 0 {
		return 20
	}
	return int(value)
}
