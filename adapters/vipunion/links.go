package vipunion

import (
	"context"

	"social-hub/pkg/socialhub"
)

const (
	urlService         = "com.vip.adp.api.open.service.UnionUrlV2Service"
	promotionURLMethod = "genByGoodsIdWithOauth"
)

type promotionLinkOptionsWire struct {
	OpenID               string            `json:"openId"`
	RealCall             bool              `json:"realCall"`
	Platform             *LinkPlatform     `json:"platform,omitempty"`
	AdCode               string            `json:"adCode"`
	RID                  string            `json:"rid,omitempty"`
	SizeIDs              map[string]string `json:"sizeIdMap,omitempty"`
	GenerateAuthorityURL *bool             `json:"genAuthorityUrl,omitempty"`
	LandingPage          *GoodsLandingPage `json:"goodsLandingPageType,omitempty"`
	GiftCode             string            `json:"giftCode,omitempty"`
}

type promotionLinkWire struct {
	GoodsIDs             []string                 `json:"goodsIdList"`
	ChanTag              string                   `json:"chanTag"`
	RequestID            string                   `json:"requestId"`
	StatParam            string                   `json:"statParam,omitempty"`
	EvokeQuickApp        *bool                    `json:"evokeQuickApp,omitempty"`
	QueryExclusiveCoupon *bool                    `json:"queryExclusiveCoupon,omitempty"`
	GenerateShortURL     *bool                    `json:"genShortUrl,omitempty"`
	Options              promotionLinkOptionsWire `json:"urlGenByGoodsIdRequest"`
}

func (client *Client) GeneratePromotionLinks(
	ctx context.Context,
	input PromotionLinkRequest,
	options ...socialhub.CallOption,
) (PromotionLinkResult, error) {
	const operation = "generate_promotion_links"
	chanTag, openID, adCode := client.chanTag(input.ChanTag), client.openID(input.OpenID), client.adCode(input.AdCode)
	if !validPromotionLink(input, chanTag, openID, adCode) {
		return PromotionLinkResult{}, invalidArgument(operation, "goods IDs, channel identity, attribution, or link options are invalid")
	}
	requestID, forwarded, err := prepareCallOptions(operation, options)
	if err != nil {
		return PromotionLinkResult{}, err
	}
	body := promotionLinkWire{
		GoodsIDs: input.GoodsIDs, ChanTag: chanTag, RequestID: requestID,
		StatParam: input.StatParam, EvokeQuickApp: input.EvokeQuickApp,
		QueryExclusiveCoupon: input.QueryExclusiveCoupon, GenerateShortURL: input.GenerateShortURL,
		Options: promotionLinkOptionsWire{
			OpenID: openID, RealCall: input.RealCall, Platform: input.Platform, AdCode: adCode,
			RID: input.RID, SizeIDs: input.SizeIDs, GenerateAuthorityURL: input.GenerateAuthorityURL,
			LandingPage: input.LandingPage, GiftCode: input.GiftCode,
		},
	}
	var response struct {
		Links []PromotionLink `json:"urlInfoList"`
	}
	meta, err := client.doJSON(
		ctx, operation, urlService, promotionURLMethod, requestID, body, &response, forwarded...,
	)
	if err != nil {
		return PromotionLinkResult{}, err
	}
	return PromotionLinkResult{Links: response.Links, Meta: meta}, nil
}

func validPromotionLink(input PromotionLinkRequest, chanTag, openID, adCode string) bool {
	if !validGoodsIDs(input.GoodsIDs, 50) || !validChanTag(chanTag) || !validOpenID(openID) ||
		!validIdentifier(adCode, 64) || !validOptionalOpaque(input.StatParam, 256) ||
		!validOptionalOpaque(input.RID, 256) || !validOptionalOpaque(input.GiftCode, 256) ||
		!validSizeIDs(input.SizeIDs, input.GoodsIDs) {
		return false
	}
	if input.Platform != nil && (*input.Platform < LinkPlatformMobile || *input.Platform > LinkPlatformHarmony) {
		return false
	}
	return input.LandingPage == nil || *input.LandingPage == GoodsLandingDetail || *input.LandingPage == GoodsLandingIntermediate
}
