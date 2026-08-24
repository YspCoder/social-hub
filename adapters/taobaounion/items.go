package taobaounion

import (
	"context"
	"net/url"
	"strings"

	"social-hub/pkg/socialhub"
)

const itemInfoMethod = "taobao.tbk.item.info.get"

func (client *Client) GetItems(
	ctx context.Context,
	input ItemInfoRequest,
	options ...socialhub.CallOption,
) (ItemInfoResult, error) {
	const operation = "get_items"
	if !validItemInfo(input) {
		return ItemInfoResult{}, invalidArgument(operation, "item IDs, platform, IP, or scene are invalid")
	}
	values := make(url.Values)
	values.Set("num_iids", strings.Join(input.ItemIDs, ","))
	setInt(values, "platform", int64(input.Platform))
	setString(values, "ip", input.IP)
	setString(values, "biz_scene_id", input.BizSceneID)
	setString(values, "promotion_type", input.PromotionType)
	setString(values, "relation_id", input.RelationID)
	setString(values, "manage_item_pub_id", input.ManageItemPubID)
	var response struct {
		Results []Item `json:"results"`
	}
	meta, err := client.doForm(ctx, operation, itemInfoMethod, values, &response, options...)
	if err != nil {
		return ItemInfoResult{}, err
	}
	return ItemInfoResult{Items: response.Results, Meta: meta}, nil
}

func validItemInfo(input ItemInfoRequest) bool {
	if len(input.ItemIDs) == 0 || len(input.ItemIDs) > 40 || !validPlatform(input.Platform) || !validIP(input.IP) ||
		!validBizScene(input.BizSceneID, true) || !validPromotionType(input.PromotionType) ||
		!validOptionalID(input.RelationID, 128) ||
		input.ManageItemPubID != "" && !validNumericID(input.ManageItemPubID, 20) {
		return false
	}
	seen := make(map[string]struct{}, len(input.ItemIDs))
	for _, itemID := range input.ItemIDs {
		if !validOptionalID(itemID, 256) || itemID == "" {
			return false
		}
		if _, duplicate := seen[itemID]; duplicate {
			return false
		}
		seen[itemID] = struct{}{}
	}
	return true
}
