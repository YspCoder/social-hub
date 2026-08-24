package pddunion

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"

	"social-hub/pkg/socialhub"
)

const promotionLinkMethod = "pdd.ddk.goods.promotion.url.generate"

func (client *Client) GeneratePromotionLinks(
	ctx context.Context,
	input PromotionLinkRequest,
	options ...socialhub.CallOption,
) (PromotionLinkResult, error) {
	const operation = "generate_promotion_links"
	pid, err := client.requiredPID(operation, input.PID)
	if err != nil {
		return PromotionLinkResult{}, err
	}
	if !validPromotionLink(input) {
		return PromotionLinkResult{}, invalidArgument(operation, "goods sign, custom parameters, search ID, or link options are invalid")
	}
	goodsSigns, err := json.Marshal([]string{input.GoodsSign})
	if err != nil {
		return PromotionLinkResult{}, platformError(operation, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
	}
	values := make(url.Values)
	values.Set("p_id", pid)
	values.Set("goods_sign_list", string(goodsSigns))
	values.Set("generate_short_url", strconv.FormatBool(input.GenerateShortURL))
	setBool(values, "multi_group", input.MultiGroup)
	setString(values, "custom_parameters", input.CustomParameters)
	setBool(values, "pull_new", input.PullNew)
	setBool(values, "generate_weapp_webview", input.GenerateWeAppWebView)
	setBool(values, "generate_we_app", input.GenerateWeApp)
	setBool(values, "generate_qq_app", input.GenerateQQApp)
	setBool(values, "generate_schema_url", input.GenerateSchemaURL)
	setBool(values, "generate_weiboapp_webview", input.GenerateWeiboAppWebView)
	setString(values, "search_id", input.SearchID)
	var response struct {
		Links []PromotionLink `json:"goods_promotion_url_list"`
	}
	meta, err := client.doForm(ctx, operation, promotionLinkMethod, "goods_promotion_url_generate_response", values, &response, options...)
	if err != nil {
		return PromotionLinkResult{}, err
	}
	if len(response.Links) == 0 {
		return PromotionLinkResult{}, platformContractError(operation, "Pinduoduo promotion response omitted links")
	}
	return PromotionLinkResult{Links: response.Links, Meta: meta}, nil
}

func validPromotionLink(input PromotionLinkRequest) bool {
	return validOpaque(input.GoodsSign, 1024) && validCustomParameters(input.CustomParameters) &&
		validOptionalOpaque(input.SearchID, 1024)
}
