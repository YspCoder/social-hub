package taobaounion

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const materialSearchMethod = "taobao.tbk.dg.material.optional"

func (client *Client) SearchMaterials(
	ctx context.Context,
	input MaterialSearchRequest,
	options ...socialhub.CallOption,
) (MaterialSearchResult, error) {
	const operation = "search_materials"
	adzoneID, err := client.adzoneID(operation, input.AdzoneID)
	if err != nil {
		return MaterialSearchResult{}, err
	}
	if !validMaterialSearch(input) {
		return MaterialSearchResult{}, invalidArgument(operation, "material filters, pagination, platform, or scene are invalid")
	}
	values := make(url.Values)
	values.Set("adzone_id", adzoneID)
	setString(values, "q", input.Query)
	if len(input.CategoryIDs) > 0 {
		values.Set("cat", strings.Join(input.CategoryIDs, ","))
	}
	setInt(values, "material_id", input.MaterialID)
	setInt(values, "page_no", input.PageNo)
	setInt(values, "page_size", input.PageSize)
	setInt(values, "platform", int64(input.Platform))
	setInt(values, "start_price", input.StartPrice)
	setInt(values, "end_price", input.EndPrice)
	setInt(values, "start_tk_rate", input.StartCommissionRate)
	setInt(values, "end_tk_rate", input.EndCommissionRate)
	setBool(values, "has_coupon", input.HasCoupon)
	setBool(values, "is_tmall", input.IsTmall)
	setBool(values, "is_overseas", input.IsOverseas)
	setString(values, "sort", input.Sort)
	setString(values, "relation_id", input.RelationID)
	setString(values, "special_id", input.SpecialID)
	setString(values, "page_result_key", input.PageResultKey)
	setString(values, "biz_scene_id", input.BizSceneID)
	setString(values, "promotion_type", input.PromotionType)
	var response struct {
		ResultList    []Material `json:"result_list"`
		TotalResults  ExactValue `json:"total_results"`
		PageResultKey string     `json:"page_result_key"`
	}
	meta, err := client.doForm(ctx, operation, materialSearchMethod, values, &response, options...)
	if err != nil {
		return MaterialSearchResult{}, err
	}
	return MaterialSearchResult{
		Materials: response.ResultList, TotalResults: response.TotalResults,
		PageResultKey: response.PageResultKey, Meta: meta,
	}, nil
}

func validMaterialSearch(input MaterialSearchRequest) bool {
	if !validOptionalText(input.Query, 512) || input.MaterialID < 0 || input.PageNo < 0 ||
		input.PageSize < 0 || input.PageSize > 100 || !validPlatform(input.Platform) ||
		input.StartPrice < 0 || input.EndPrice < 0 || input.StartCommissionRate < 0 || input.EndCommissionRate < 0 ||
		input.StartPrice > 0 && input.EndPrice > 0 && input.EndPrice < input.StartPrice ||
		input.StartCommissionRate > 0 && input.EndCommissionRate > 0 && input.EndCommissionRate < input.StartCommissionRate ||
		!validOptionalText(input.Sort, 128) || !validOptionalID(input.RelationID, 128) ||
		!validOptionalID(input.SpecialID, 128) || input.PageResultKey != "" && !validOpaque(input.PageResultKey, 16_384) ||
		input.BizSceneID != "" && input.BizSceneID != "1" && input.BizSceneID != "2" ||
		!validPromotionType(input.PromotionType) || len(input.CategoryIDs) > 10 {
		return false
	}
	seen := make(map[string]struct{}, len(input.CategoryIDs))
	for _, categoryID := range input.CategoryIDs {
		if !validNumericID(categoryID, 20) {
			return false
		}
		if _, duplicate := seen[categoryID]; duplicate {
			return false
		}
		seen[categoryID] = struct{}{}
	}
	return true
}
