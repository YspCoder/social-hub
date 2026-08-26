package mintegral

import (
	"context"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

type CreativeSetsWorkflow interface {
	List(context.Context, mtg.CreativeSetListRequest, ...socialhub.CallOption) (mtg.CreativeSetList, error)
	Create(context.Context, mtg.CreateCreativeSetRequest, ...socialhub.CallOption) (mtg.CreativeSetMutationResult, error)
	Update(context.Context, mtg.UpdateCreativeSetRequest, ...socialhub.CallOption) error
	Delete(context.Context, mtg.DeleteCreativeSetRequest, ...socialhub.CallOption) error
}

type CreativeSetsService struct{ client *Client }

func (service *CreativeSetsService) List(ctx context.Context, input mtg.CreativeSetListRequest, options ...socialhub.CallOption) (mtg.CreativeSetList, error) {
	return callValue(ctx, "creative_sets_list", options, func(callCtx context.Context) (mtg.CreativeSetList, error) {
		return service.client.sdk.CreativeSets().List(callCtx, input)
	})
}

func (service *CreativeSetsService) Create(ctx context.Context, input mtg.CreateCreativeSetRequest, options ...socialhub.CallOption) (mtg.CreativeSetMutationResult, error) {
	return callValue(ctx, "creative_set_create", options, func(callCtx context.Context) (mtg.CreativeSetMutationResult, error) {
		return service.client.sdk.CreativeSets().Create(callCtx, input)
	})
}

func (service *CreativeSetsService) Update(ctx context.Context, input mtg.UpdateCreativeSetRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "creative_set_update", options, func(callCtx context.Context) error {
		return service.client.sdk.CreativeSets().Update(callCtx, input)
	})
}

func (service *CreativeSetsService) Delete(ctx context.Context, input mtg.DeleteCreativeSetRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "creative_set_delete", options, func(callCtx context.Context) error {
		return service.client.sdk.CreativeSets().Delete(callCtx, input)
	})
}

type CreativeAdsWorkflow interface {
	List(context.Context, mtg.CreativeAdListRequest, ...socialhub.CallOption) (mtg.CreativeAdList, error)
}

type CreativeAdsService struct{ client *Client }

func (service *CreativeAdsService) List(ctx context.Context, input mtg.CreativeAdListRequest, options ...socialhub.CallOption) (mtg.CreativeAdList, error) {
	return callValue(ctx, "creative_ads_list", options, func(callCtx context.Context) (mtg.CreativeAdList, error) {
		return service.client.sdk.CreativeAds().List(callCtx, input)
	})
}

type AssetsWorkflow interface {
	List(context.Context, mtg.AssetListRequest, ...socialhub.CallOption) (mtg.AssetList, error)
	UploadMedia(context.Context, mtg.UploadSource, ...socialhub.CallOption) (mtg.UploadedAsset, error)
	UploadPlayable(context.Context, mtg.UploadSource, ...socialhub.CallOption) (mtg.UploadedAsset, error)
}

type AssetsService struct{ client *Client }

func (service *AssetsService) List(ctx context.Context, input mtg.AssetListRequest, options ...socialhub.CallOption) (mtg.AssetList, error) {
	return callValue(ctx, "assets_list", options, func(callCtx context.Context) (mtg.AssetList, error) {
		return service.client.sdk.Assets().List(callCtx, input)
	})
}

func (service *AssetsService) UploadMedia(ctx context.Context, source mtg.UploadSource, options ...socialhub.CallOption) (mtg.UploadedAsset, error) {
	return callValue(ctx, "asset_media_upload", options, func(callCtx context.Context) (mtg.UploadedAsset, error) {
		return service.client.sdk.Assets().UploadMedia(callCtx, source)
	})
}

func (service *AssetsService) UploadPlayable(ctx context.Context, source mtg.UploadSource, options ...socialhub.CallOption) (mtg.UploadedAsset, error) {
	return callValue(ctx, "asset_playable_upload", options, func(callCtx context.Context) (mtg.UploadedAsset, error) {
		return service.client.sdk.Assets().UploadPlayable(callCtx, source)
	})
}

var _ CreativeSetsWorkflow = (*CreativeSetsService)(nil)
var _ CreativeAdsWorkflow = (*CreativeAdsService)(nil)
var _ AssetsWorkflow = (*AssetsService)(nil)
