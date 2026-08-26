package mintegral

import (
	"context"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

type OffersWorkflow interface {
	List(context.Context, mtg.OfferListRequest, ...socialhub.CallOption) (mtg.OfferPage, error)
	Create(context.Context, mtg.CreateOfferRequest, ...socialhub.CallOption) (mtg.Offer, error)
	Update(context.Context, mtg.UpdateOfferRequest, ...socialhub.CallOption) (mtg.Offer, error)
	UpdateBids(context.Context, mtg.UpdateOfferBidsRequest, ...socialhub.CallOption) (mtg.OfferMutationResult, error)
	UpdateBudget(context.Context, mtg.UpdateOfferBudgetRequest, ...socialhub.CallOption) error
	SetStatus(context.Context, mtg.SetOfferStatusRequest, ...socialhub.CallOption) error
	UpdateTrafficDelivery(context.Context, mtg.UpdateTrafficDeliveryRequest, ...socialhub.CallOption) error
	UpdateTracking(context.Context, mtg.UpdateOfferTrackingRequest, ...socialhub.CallOption) (mtg.OfferTracking, error)
	SetAudiences(context.Context, mtg.SetOfferAudiencesRequest, ...socialhub.CallOption) error
	UpdateTargetGoal(context.Context, mtg.UpdateOfferTargetGoalRequest, ...socialhub.CallOption) error
	ApplyCreatives(context.Context, mtg.ApplyOfferCreativesRequest, ...socialhub.CallOption) ([]string, error)
}

type OffersService struct{ client *Client }

func (service *OffersService) List(ctx context.Context, input mtg.OfferListRequest, options ...socialhub.CallOption) (mtg.OfferPage, error) {
	return callValue(ctx, "offers_list", options, func(callCtx context.Context) (mtg.OfferPage, error) {
		return service.client.sdk.Offers().List(callCtx, input)
	})
}

func (service *OffersService) Create(ctx context.Context, input mtg.CreateOfferRequest, options ...socialhub.CallOption) (mtg.Offer, error) {
	return callValue(ctx, "offer_create", options, func(callCtx context.Context) (mtg.Offer, error) {
		return service.client.sdk.Offers().Create(callCtx, input)
	})
}

func (service *OffersService) Update(ctx context.Context, input mtg.UpdateOfferRequest, options ...socialhub.CallOption) (mtg.Offer, error) {
	return callValue(ctx, "offer_update", options, func(callCtx context.Context) (mtg.Offer, error) {
		return service.client.sdk.Offers().Update(callCtx, input)
	})
}

func (service *OffersService) UpdateBids(ctx context.Context, input mtg.UpdateOfferBidsRequest, options ...socialhub.CallOption) (mtg.OfferMutationResult, error) {
	return callValue(ctx, "offer_bids_update", options, func(callCtx context.Context) (mtg.OfferMutationResult, error) {
		return service.client.sdk.Offers().UpdateBids(callCtx, input)
	})
}

func (service *OffersService) UpdateBudget(ctx context.Context, input mtg.UpdateOfferBudgetRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "offer_budget_update", options, func(callCtx context.Context) error {
		return service.client.sdk.Offers().UpdateBudget(callCtx, input)
	})
}

func (service *OffersService) SetStatus(ctx context.Context, input mtg.SetOfferStatusRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "offer_status_set", options, func(callCtx context.Context) error {
		return service.client.sdk.Offers().SetStatus(callCtx, input)
	})
}

func (service *OffersService) UpdateTrafficDelivery(ctx context.Context, input mtg.UpdateTrafficDeliveryRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "offer_traffic_delivery_update", options, func(callCtx context.Context) error {
		return service.client.sdk.Offers().UpdateTrafficDelivery(callCtx, input)
	})
}

func (service *OffersService) UpdateTracking(ctx context.Context, input mtg.UpdateOfferTrackingRequest, options ...socialhub.CallOption) (mtg.OfferTracking, error) {
	return callValue(ctx, "offer_tracking_update", options, func(callCtx context.Context) (mtg.OfferTracking, error) {
		return service.client.sdk.Offers().UpdateTracking(callCtx, input)
	})
}

func (service *OffersService) SetAudiences(ctx context.Context, input mtg.SetOfferAudiencesRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "offer_audiences_set", options, func(callCtx context.Context) error {
		return service.client.sdk.Offers().SetAudiences(callCtx, input)
	})
}

func (service *OffersService) UpdateTargetGoal(ctx context.Context, input mtg.UpdateOfferTargetGoalRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "offer_target_goal_update", options, func(callCtx context.Context) error {
		return service.client.sdk.Offers().UpdateTargetGoal(callCtx, input)
	})
}

// ApplyCreatives exposes Mintegral's deprecated legacy endpoint for complete
// upstream coverage. New integrations should use CreativeSets instead.
func (service *OffersService) ApplyCreatives(ctx context.Context, input mtg.ApplyOfferCreativesRequest, options ...socialhub.CallOption) ([]string, error) {
	return callValue(ctx, "offer_creatives_apply", options, func(callCtx context.Context) ([]string, error) {
		return service.client.sdk.Offers().ApplyCreatives(callCtx, input)
	})
}

var _ OffersWorkflow = (*OffersService)(nil)
