package jdunion

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

const promotionCommonGetMethod = "jd.union.open.promotion.common.get"

func (client *Client) CreatePromotion(
	ctx context.Context,
	input PromotionRequest,
	options ...socialhub.CallOption,
) (*Promotion, error) {
	const operation = "create_promotion"
	siteID, err := client.siteID(operation, input.SiteID)
	if err != nil {
		return nil, err
	}
	if !validPromotion(input) {
		return nil, invalidArgument(operation, "material, attribution identifiers, coupon, command, or scene are invalid")
	}
	command := uint64(0)
	if input.GenerateCommand {
		command = 1
	}
	request := struct {
		MaterialID    string `json:"materialId"`
		SiteID        string `json:"siteId"`
		PositionID    uint64 `json:"positionId,omitempty"`
		SubUnionID    string `json:"subUnionId,omitempty"`
		ExternalID    string `json:"ext1,omitempty"`
		PID           string `json:"pid,omitempty"`
		CouponURL     string `json:"couponUrl,omitempty"`
		GiftCouponKey string `json:"giftCouponKey,omitempty"`
		ChannelID     uint64 `json:"channelId,omitempty"`
		RID           string `json:"rid,omitempty"`
		Command       uint64 `json:"command,omitempty"`
		SceneID       uint64 `json:"sceneId"`
	}{
		MaterialID: input.MaterialID, SiteID: siteID, PositionID: input.PositionID,
		SubUnionID: input.SubUnionID, ExternalID: input.ExternalID, PID: input.PID,
		CouponURL: input.CouponURL, GiftCouponKey: input.GiftCouponKey,
		ChannelID: input.ChannelID, RID: input.RID, Command: command, SceneID: uint64(input.SceneID),
	}
	result, meta, err := client.doMethod(ctx, operation, promotionCommonGetMethod, "promotionCodeReq", request, "getResult", options...)
	if err != nil {
		return nil, err
	}
	data, err := decodeObjectOrString(result.Data)
	if err != nil || len(data) == 0 || string(data) == "null" {
		return nil, platformContractError(operation, "JD promotion response omitted data")
	}
	var wire struct {
		ClickURL string `json:"clickURL"`
		JCommand string `json:"jCommand"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, platformError(operation, socialhub.CodePlatformError, socialhub.ClassPermanent, err)
	}
	if wire.ClickURL == "" {
		return nil, platformContractError(operation, "JD promotion response omitted clickURL")
	}
	return &Promotion{
		ClickURL: wire.ClickURL, JCommand: wire.JCommand, Meta: meta, Raw: append(json.RawMessage(nil), data...),
	}, nil
}

func validPromotion(input PromotionRequest) bool {
	if !validOpaque(input.MaterialID, 8192) || input.SceneID != PromotionSceneAffiliate && input.SceneID != PromotionSceneMainSite ||
		!validASCIITag(input.SubUnionID, 80) || !validASCIITag(input.ExternalID, 40) || !validPID(input.PID) ||
		input.SubUnionID != "" && input.PID != "" ||
		!validOptionalHTTPURL(input.CouponURL) || !validOptionalIdentifier(input.GiftCouponKey, 256) ||
		!validOptionalIdentifier(input.RID, 256) {
		return false
	}
	return true
}
