package mintegral

import (
	"context"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

type AccountsWorkflow interface {
	Balance(context.Context, ...socialhub.CallOption) (mtg.AccountBalance, error)
}

type AccountsService struct{ client *Client }

func (service *AccountsService) Balance(ctx context.Context, options ...socialhub.CallOption) (mtg.AccountBalance, error) {
	return callValue(ctx, "account_balance", options, func(callCtx context.Context) (mtg.AccountBalance, error) {
		return service.client.sdk.Accounts().Balance(callCtx)
	})
}

type CampaignsWorkflow interface {
	List(context.Context, mtg.CampaignListRequest, ...socialhub.CallOption) (mtg.CampaignPage, error)
	Create(context.Context, mtg.CreateCampaignRequest, ...socialhub.CallOption) (mtg.Campaign, error)
	Update(context.Context, mtg.UpdateCampaignRequest, ...socialhub.CallOption) (mtg.Campaign, error)
}

type CampaignsService struct{ client *Client }

func (service *CampaignsService) List(ctx context.Context, input mtg.CampaignListRequest, options ...socialhub.CallOption) (mtg.CampaignPage, error) {
	return callValue(ctx, "campaigns_list", options, func(callCtx context.Context) (mtg.CampaignPage, error) {
		return service.client.sdk.Campaigns().List(callCtx, input)
	})
}

func (service *CampaignsService) Create(ctx context.Context, input mtg.CreateCampaignRequest, options ...socialhub.CallOption) (mtg.Campaign, error) {
	return callValue(ctx, "campaign_create", options, func(callCtx context.Context) (mtg.Campaign, error) {
		return service.client.sdk.Campaigns().Create(callCtx, input)
	})
}

func (service *CampaignsService) Update(ctx context.Context, input mtg.UpdateCampaignRequest, options ...socialhub.CallOption) (mtg.Campaign, error) {
	return callValue(ctx, "campaign_update", options, func(callCtx context.Context) (mtg.Campaign, error) {
		return service.client.sdk.Campaigns().Update(callCtx, input)
	})
}

type AppsWorkflow interface {
	Names(context.Context, mtg.AppNameRequest, ...socialhub.CallOption) ([]mtg.AppName, error)
}

type AppsService struct{ client *Client }

func (service *AppsService) Names(ctx context.Context, input mtg.AppNameRequest, options ...socialhub.CallOption) ([]mtg.AppName, error) {
	return callValue(ctx, "app_names", options, func(callCtx context.Context) ([]mtg.AppName, error) {
		return service.client.sdk.Apps().Names(callCtx, input)
	})
}

type EventsWorkflow interface {
	BidGoalSupports(context.Context, mtg.BidGoalSupportsRequest, ...socialhub.CallOption) (mtg.BidGoalSupportsResponse, error)
}

type EventsService struct{ client *Client }

func (service *EventsService) BidGoalSupports(ctx context.Context, input mtg.BidGoalSupportsRequest, options ...socialhub.CallOption) (mtg.BidGoalSupportsResponse, error) {
	return callValue(ctx, "events_bid_goal_supports", options, func(callCtx context.Context) (mtg.BidGoalSupportsResponse, error) {
		return service.client.sdk.Events().BidGoalSupports(callCtx, input)
	})
}

var _ AccountsWorkflow = (*AccountsService)(nil)
var _ CampaignsWorkflow = (*CampaignsService)(nil)
var _ AppsWorkflow = (*AppsService)(nil)
var _ EventsWorkflow = (*EventsService)(nil)
