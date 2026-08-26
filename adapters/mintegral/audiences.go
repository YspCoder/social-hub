package mintegral

import (
	"context"

	mtg "github.com/jageros/mintegral-go"

	"social-hub/pkg/socialhub"
)

type AudiencesWorkflow interface {
	List(context.Context, mtg.AudienceListRequest, ...socialhub.CallOption) (mtg.AudienceList, error)
	PresignUpload(context.Context, mtg.AudiencePresignRequest, ...socialhub.CallOption) (mtg.AudienceUploadPlan, error)
	Upload(context.Context, mtg.AudienceUploadPlan, mtg.UploadSource, ...socialhub.CallOption) (mtg.AudienceUploadResult, error)
	UploadFile(context.Context, mtg.AudiencePresignRequest, mtg.UploadSource, ...socialhub.CallOption) (mtg.AudienceUploadResult, error)
	Create(context.Context, mtg.CreateAudienceRequest, ...socialhub.CallOption) (mtg.AudienceMutationResult, error)
	Update(context.Context, mtg.UpdateAudienceRequest, ...socialhub.CallOption) (mtg.AudienceMutationResult, error)
	Delete(context.Context, mtg.DeleteAudienceRequest, ...socialhub.CallOption) error
}

type AudiencesService struct{ client *Client }

func (service *AudiencesService) List(ctx context.Context, input mtg.AudienceListRequest, options ...socialhub.CallOption) (mtg.AudienceList, error) {
	return callValue(ctx, "audiences_list", options, func(callCtx context.Context) (mtg.AudienceList, error) {
		return service.client.sdk.Audiences().List(callCtx, input)
	})
}

func (service *AudiencesService) PresignUpload(ctx context.Context, input mtg.AudiencePresignRequest, options ...socialhub.CallOption) (mtg.AudienceUploadPlan, error) {
	return callValue(ctx, "audience_upload_presign", options, func(callCtx context.Context) (mtg.AudienceUploadPlan, error) {
		return service.client.sdk.Audiences().PresignUpload(callCtx, input)
	})
}

func (service *AudiencesService) Upload(ctx context.Context, plan mtg.AudienceUploadPlan, source mtg.UploadSource, options ...socialhub.CallOption) (mtg.AudienceUploadResult, error) {
	return callValue(ctx, "audience_upload", options, func(callCtx context.Context) (mtg.AudienceUploadResult, error) {
		return service.client.sdk.Audiences().Upload(callCtx, plan, source)
	})
}

func (service *AudiencesService) UploadFile(ctx context.Context, input mtg.AudiencePresignRequest, source mtg.UploadSource, options ...socialhub.CallOption) (mtg.AudienceUploadResult, error) {
	return callValue(ctx, "audience_file_upload", options, func(callCtx context.Context) (mtg.AudienceUploadResult, error) {
		return service.client.sdk.Audiences().UploadFile(callCtx, input, source)
	})
}

func (service *AudiencesService) Create(ctx context.Context, input mtg.CreateAudienceRequest, options ...socialhub.CallOption) (mtg.AudienceMutationResult, error) {
	return callValue(ctx, "audience_create", options, func(callCtx context.Context) (mtg.AudienceMutationResult, error) {
		return service.client.sdk.Audiences().Create(callCtx, input)
	})
}

func (service *AudiencesService) Update(ctx context.Context, input mtg.UpdateAudienceRequest, options ...socialhub.CallOption) (mtg.AudienceMutationResult, error) {
	return callValue(ctx, "audience_update", options, func(callCtx context.Context) (mtg.AudienceMutationResult, error) {
		return service.client.sdk.Audiences().Update(callCtx, input)
	})
}

func (service *AudiencesService) Delete(ctx context.Context, input mtg.DeleteAudienceRequest, options ...socialhub.CallOption) error {
	return callNoValue(ctx, "audience_delete", options, func(callCtx context.Context) error {
		return service.client.sdk.Audiences().Delete(callCtx, input)
	})
}

var _ AudiencesWorkflow = (*AudiencesService)(nil)
