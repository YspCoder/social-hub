package taobaounion

import (
	"context"
	"encoding/json"
	"net/url"

	"social-hub/pkg/socialhub"
)

const (
	linkConversionMethod = "taobao.tbk.dg.general.link.convert"
	taoPasswordMethod    = "taobao.tbk.tpwd.create"
)

func (client *Client) ConvertLinks(
	ctx context.Context,
	input LinkConversionRequest,
	options ...socialhub.CallOption,
) (LinkConversionResult, error) {
	const operation = "convert_links"
	adzoneID, err := client.adzoneID(operation, input.AdzoneID)
	if err != nil {
		return LinkConversionResult{}, err
	}
	if !validLinkConversion(input) {
		return LinkConversionResult{}, invalidArgument(operation, "link-conversion items, materials, or scene are invalid")
	}
	values := make(url.Values)
	values.Set("adzone_id", adzoneID)
	if len(input.Items) > 0 {
		encoded, err := json.Marshal(input.Items)
		if err != nil {
			return LinkConversionResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		values.Set("item_dto", string(encoded))
	}
	if len(input.Materials) > 0 {
		encoded, err := json.Marshal(input.Materials)
		if err != nil {
			return LinkConversionResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		values.Set("material_dto", string(encoded))
	}
	setString(values, "biz_scene_id", input.BizSceneID)
	setString(values, "promotion_type", input.PromotionType)
	setString(values, "relation_id", input.RelationID)
	setString(values, "special_id", input.SpecialID)
	var response struct {
		Data *struct {
			ItemLinks     []ItemLinkResult     `json:"item_url_list"`
			MaterialLinks []MaterialLinkResult `json:"material_url_list"`
			ShopLinks     []json.RawMessage    `json:"shop_url_list"`
			EventLinks    []json.RawMessage    `json:"event_url_list"`
		} `json:"data"`
		BusinessError   ExactValue `json:"biz_error_desc"`
		BusinessMessage string     `json:"result_msg"`
	}
	meta, err := client.doForm(ctx, operation, linkConversionMethod, values, &response, options...)
	if err != nil {
		return LinkConversionResult{}, err
	}
	if response.Data == nil {
		return LinkConversionResult{}, platformContractError(operation, "TOP link-conversion response omitted data")
	}
	return LinkConversionResult{
		ItemLinks: response.Data.ItemLinks, MaterialLinks: response.Data.MaterialLinks,
		ShopLinks: response.Data.ShopLinks, EventLinks: response.Data.EventLinks,
		BusinessError: response.BusinessError, BusinessMessage: response.BusinessMessage, Meta: meta,
	}, nil
}

func (client *Client) CreateTaoPassword(
	ctx context.Context,
	input TaoPasswordRequest,
	options ...socialhub.CallOption,
) (*TaoPassword, error) {
	const operation = "create_tao_password"
	if !validHTTPURL(input.URL) {
		return nil, invalidArgument(operation, "url must be an absolute HTTP(S) affiliate URL")
	}
	values := make(url.Values)
	values.Set("url", input.URL)
	var response struct {
		Data *TaoPassword `json:"data"`
	}
	meta, err := client.doForm(ctx, operation, taoPasswordMethod, values, &response, options...)
	if err != nil {
		return nil, err
	}
	if response.Data == nil || response.Data.PasswordSimple == "" && response.Data.Model == "" {
		return nil, platformContractError(operation, "TOP Tao Password response omitted password data")
	}
	response.Data.Meta = meta
	return response.Data, nil
}

func validLinkConversion(input LinkConversionRequest) bool {
	if len(input.Items) == 0 && len(input.Materials) == 0 || !validBizScene(input.BizSceneID, false) ||
		!validPromotionType(input.PromotionType) ||
		input.RelationID != "" && !validNumericID(input.RelationID, 20) ||
		input.SpecialID != "" && !validNumericID(input.SpecialID, 20) {
		return false
	}
	for _, item := range input.Items {
		if !validOptionalID(item.ItemID, 256) || item.ItemID == "" || !validOptionalID(item.CouponID, 256) ||
			!validOptionalText(item.ExternalID, 512) || item.GeneralPlan != "" && item.GeneralPlan != "1" ||
			item.SKUID < 0 || item.ManagePublisherID < 0 || item.TargetCoupon < 0 || item.TargetCoupon > 1 {
			return false
		}
	}
	for _, material := range input.Materials {
		if !validOpaque(material.MaterialURL, 8192) || !validOptionalID(material.CouponID, 256) ||
			material.TargetCoupon < 0 || material.TargetCoupon > 1 {
			return false
		}
	}
	return true
}
