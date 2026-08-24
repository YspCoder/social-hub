package applovinads

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

var errUploadSizeMismatch = errors.New("asset byte count is shorter than its declared size")

type AssetWorkflow interface {
	ListAssets(context.Context, ListAssetsRequest, ...socialhub.CallOption) ([]Asset, error)
	UploadAssets(context.Context, []UploadFile, ...socialhub.CallOption) (*UploadRef, error)
	GetAssetUploadStatus(context.Context, string, ...socialhub.CallOption) (*UploadStatus, error)
	AddAssetsToCreativeSets(context.Context, AssetAssociationRequest, ...socialhub.CallOption) error
	RemoveAssetFromCreativeSets(context.Context, AssetRemovalRequest, ...socialhub.CallOption) error
	RemoveAssetsFromAllCreativeSets(context.Context, []int64, ...socialhub.CallOption) error
}

func (client *Client) ListAssets(ctx context.Context, input ListAssetsRequest, options ...socialhub.CallOption) ([]Asset, error) {
	if input.Page < 0 || input.Size < 0 || input.Size > maximumListSize || len(input.IDs) > maximumListSize || !validStringIDs(input.IDs, true) ||
		input.ResourceType != "" && input.ResourceType != ResourceImage && input.ResourceType != ResourceHTML && input.ResourceType != ResourceVideo {
		return nil, invalidArgument("asset_list", "page, size, ids, or resource_type is invalid")
	}
	if err := client.requireAccess("asset_list"); err != nil {
		return nil, err
	}
	page, size := normalizedPage(input.Page, input.Size)
	query := url.Values{"page": {strconv.Itoa(page)}, "size": {strconv.Itoa(size)}}
	if len(input.IDs) > 0 {
		query.Set("ids", strings.Join(input.IDs, ","))
	}
	if input.ResourceType != "" {
		query.Set("resource_type", string(input.ResourceType))
	}
	var response []Asset
	if err := client.getJSON(ctx, "asset_list", "/asset/list", query, &response, options...); err != nil {
		return nil, err
	}
	for _, asset := range response {
		if !validAssetResponse(asset) {
			return nil, platformContractError("asset_list", "Axon returned an invalid Asset list")
		}
	}
	return response, nil
}

func (client *Client) UploadAssets(ctx context.Context, files []UploadFile, options ...socialhub.CallOption) (*UploadRef, error) {
	mediaTypes, err := validateUploadFiles(files)
	if err != nil {
		return nil, err
	}
	if err := client.requireAccess("asset_upload"); err != nil {
		return nil, err
	}
	pipeReader, pipeWriter := io.Pipe()
	multipartWriter := multipart.NewWriter(pipeWriter)
	request, err := client.newRequest(ctx, "asset_upload", http.MethodPost, "/asset/upload", nil, pipeReader, options...)
	if err != nil {
		_ = pipeReader.Close()
		_ = pipeWriter.Close()
		return nil, err
	}
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	done := make(chan error, 1)
	go func() {
		writeErr := writeAssetMultipart(multipartWriter, files, mediaTypes)
		if writeErr == nil {
			writeErr = multipartWriter.Close()
		}
		_ = pipeWriter.CloseWithError(writeErr)
		done <- writeErr
	}()
	var response UploadRef
	requestErr := withOperation(client.api.Do(request, &response), "asset_upload")
	_ = pipeReader.CloseWithError(requestErr)
	writeErr := <-done
	if requestErr != nil {
		return nil, requestErr
	}
	if errors.Is(writeErr, errUploadSizeMismatch) {
		return nil, invalidArgument("asset_upload", writeErr.Error())
	}
	if writeErr != nil && !errors.Is(writeErr, io.ErrClosedPipe) {
		return nil, platformError("asset_upload", socialhub.CodeInvalidArgument, socialhub.ClassPermanent, writeErr)
	}
	if !validOpaque(response.UploadID, 256) {
		return nil, platformContractError("asset_upload", "Axon returned an invalid upload ID")
	}
	return &response, nil
}

func writeAssetMultipart(writer *multipart.Writer, files []UploadFile, mediaTypes []string) error {
	for index, file := range files {
		disposition := mime.FormatMediaType("form-data", map[string]string{"name": "files", "filename": file.Filename})
		header := make(textproto.MIMEHeader)
		header.Set("Content-Disposition", disposition)
		header.Set("Content-Type", mediaTypes[index])
		part, err := writer.CreatePart(header)
		if err != nil {
			return err
		}
		written, err := io.CopyN(part, file.Reader, file.Size)
		if err != nil || written != file.Size {
			return errUploadSizeMismatch
		}
	}
	return nil
}

func (client *Client) GetAssetUploadStatus(ctx context.Context, uploadID string, options ...socialhub.CallOption) (*UploadStatus, error) {
	if !validOpaque(uploadID, 256) || strings.Contains(uploadID, ",") {
		return nil, invalidArgument("asset_upload_status", "upload ID is invalid")
	}
	if err := client.requireAccess("asset_upload_status"); err != nil {
		return nil, err
	}
	var response UploadStatus
	query := url.Values{"upload_id": {uploadID}}
	if err := client.getJSON(ctx, "asset_upload_status", "/asset/upload_result", query, &response, options...); err != nil {
		return nil, err
	}
	if !validUploadStatus(response) {
		return nil, platformContractError("asset_upload_status", "Axon returned invalid upload status counts or fields")
	}
	return &response, nil
}

func (client *Client) AddAssetsToCreativeSets(ctx context.Context, input AssetAssociationRequest, options ...socialhub.CallOption) error {
	if !validPositiveIDs(input.AssetIDs, 10) || !validPositiveIDs(input.CreativeSetIDs, 50) {
		return invalidArgument("asset_add_to_creative_sets", "Asset IDs or Creative Set IDs are invalid")
	}
	if err := client.requireAccess("asset_add_to_creative_sets"); err != nil {
		return err
	}
	return client.postJSON(ctx, "asset_add_to_creative_sets", "/asset/add-to-creative-sets", input, nil, options...)
}

func (client *Client) RemoveAssetFromCreativeSets(ctx context.Context, input AssetRemovalRequest, options ...socialhub.CallOption) error {
	if input.AssetID <= 0 || !validPositiveIDs(input.CreativeSetIDs, 50) {
		return invalidArgument("asset_remove_from_creative_sets", "Asset ID or Creative Set IDs are invalid")
	}
	if err := client.requireAccess("asset_remove_from_creative_sets"); err != nil {
		return err
	}
	return client.postJSON(ctx, "asset_remove_from_creative_sets", "/asset/remove-from-creative-sets", input, nil, options...)
}

func (client *Client) RemoveAssetsFromAllCreativeSets(ctx context.Context, assetIDs []int64, options ...socialhub.CallOption) error {
	if !validPositiveIDs(assetIDs, 10) {
		return invalidArgument("asset_remove_from_all_creative_sets", "Asset IDs are invalid")
	}
	if err := client.requireAccess("asset_remove_from_all_creative_sets"); err != nil {
		return err
	}
	return client.postJSON(ctx, "asset_remove_from_all_creative_sets", "/asset/remove-from-all-creative-sets", map[string]any{"asset_ids": assetIDs}, nil, options...)
}

func validAssetResponse(value Asset) bool {
	return validNumericID(value.ID) && validText(value.Name, 1024) && validAbsoluteURL(value.URL) && value.AssetType != "" && value.ResourceType != ""
}

func validUploadStatus(value UploadStatus) bool {
	if value.UploadStatus != "PENDING" && value.UploadStatus != "FINISHED" || value.Summary.Total < 0 || value.Summary.Success < 0 || value.Summary.Failed < 0 || value.Summary.Pending < 0 ||
		value.Summary.Success+value.Summary.Failed+value.Summary.Pending != value.Summary.Total {
		return false
	}
	for _, detail := range value.Details {
		if detail.FileStatus != "PENDING" && detail.FileStatus != "FAILURE" && detail.FileStatus != "SUCCESS" {
			return false
		}
		if detail.ID != "" && !validNumericID(detail.ID) || detail.URL != "" && !validAbsoluteURL(detail.URL) {
			return false
		}
	}
	return true
}
