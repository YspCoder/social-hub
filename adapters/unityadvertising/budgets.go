package unityadvertising

import (
	"context"
	"encoding/json"

	"social-hub/pkg/socialhub"
)

type CountryBudgetGroup struct {
	Name      string        `json:"name"`
	Countries []CountryCode `json:"countries"`
	Limit     Money         `json:"limit"`
}

// CampaignBudget represents Unity's daily, per-country, or country-group union.
type CampaignBudget struct {
	Total                *Money                `json:"total,omitempty"`
	DailySpent           *Money                `json:"dailySpent,omitempty"`
	Spent                *Money                `json:"spent,omitempty"`
	Daily                *Money                `json:"daily,omitempty"`
	DailyPerCountry      map[CountryCode]Money `json:"dailyPerCountry,omitempty"`
	DailyPerCountryGroup []CountryBudgetGroup  `json:"dailyPerCountryGroup,omitempty"`
	Raw                  json.RawMessage       `json:"-"`
}

type CampaignBudgetUpdate interface {
	isCampaignBudgetUpdate()
}

type DailyBudgetUpdate struct {
	Total *Money `json:"total,omitempty"`
	Daily *Money `json:"daily,omitempty"`
}

func (DailyBudgetUpdate) isCampaignBudgetUpdate() {}

type CountryBudgetUpdate struct {
	Total           *Money                `json:"total,omitempty"`
	DailyPerCountry map[CountryCode]Money `json:"dailyPerCountry"`
}

func (CountryBudgetUpdate) isCampaignBudgetUpdate() {}

type CountryGroupBudgetUpdate struct {
	Total                *Money               `json:"total,omitempty"`
	DailyPerCountryGroup []CountryBudgetGroup `json:"dailyPerCountryGroup"`
}

func (CountryGroupBudgetUpdate) isCampaignBudgetUpdate() {}

func (client *Client) GetCampaignBudget(ctx context.Context, campaignSetID, campaignID string, options ...socialhub.CallOption) (*CampaignBudget, error) {
	path, err := client.campaignPath("campaign_budget_get", campaignSetID, campaignID)
	if err != nil {
		return nil, err
	}
	var budget CampaignBudget
	if err := client.getJSON(ctx, "campaign_budget_get", path+"/budget", nil, &budget, options...); err != nil {
		return nil, err
	}
	if !validCampaignBudget(budget) {
		return nil, platformContractError("campaign_budget_get", "Unity returned an invalid campaign budget")
	}
	return &budget, nil
}

func (client *Client) UpdateCampaignBudget(ctx context.Context, campaignSetID, campaignID string, input CampaignBudgetUpdate, options ...socialhub.CallOption) (*CampaignBudget, error) {
	path, err := client.campaignPath("campaign_budget_update", campaignSetID, campaignID)
	if err != nil {
		return nil, err
	}
	if err := validateCampaignBudgetUpdate(input); err != nil {
		return nil, err
	}
	var budget CampaignBudget
	if err := client.patchJSON(ctx, "campaign_budget_update", path+"/budget", input, &budget, options...); err != nil {
		return nil, err
	}
	if !validCampaignBudget(budget) {
		return nil, platformContractError("campaign_budget_update", "Unity returned an invalid campaign budget")
	}
	return &budget, nil
}

func validateCampaignBudgetUpdate(input CampaignBudgetUpdate) error {
	var total *Money
	switch typed := input.(type) {
	case DailyBudgetUpdate:
		total = typed.Total
		if typed.Total == nil && typed.Daily == nil {
			return invalidArgument("campaign_budget_update", "daily budget update must contain total or daily")
		}
		if typed.Daily != nil && !validMoney(*typed.Daily) {
			return invalidArgument("campaign_budget_update", "daily budget must use up to nine integer and two fractional digits")
		}
	case *DailyBudgetUpdate:
		if typed == nil {
			return invalidArgument("campaign_budget_update", "budget update is required")
		}
		return validateCampaignBudgetUpdate(*typed)
	case CountryBudgetUpdate:
		total = typed.Total
		if len(typed.DailyPerCountry) == 0 || !validCountryBudgetMap(typed.DailyPerCountry) {
			return invalidArgument("campaign_budget_update", "per-country budget must contain valid positive country limits")
		}
	case *CountryBudgetUpdate:
		if typed == nil {
			return invalidArgument("campaign_budget_update", "budget update is required")
		}
		return validateCampaignBudgetUpdate(*typed)
	case CountryGroupBudgetUpdate:
		total = typed.Total
		if !validCountryBudgetGroups(typed.DailyPerCountryGroup) {
			return invalidArgument("campaign_budget_update", "country-group budget is invalid")
		}
	case *CountryGroupBudgetUpdate:
		if typed == nil {
			return invalidArgument("campaign_budget_update", "budget update is required")
		}
		return validateCampaignBudgetUpdate(*typed)
	default:
		return invalidArgument("campaign_budget_update", "budget update type is unsupported")
	}
	if total != nil && !validTotalMoney(*total) {
		return invalidArgument("campaign_budget_update", "total budget must be zero or a positive amount with up to two fractional digits")
	}
	return nil
}

func validCampaignBudget(budget CampaignBudget) bool {
	variants := 0
	if budget.Daily != nil {
		variants++
	}
	if budget.DailyPerCountry != nil {
		variants++
	}
	if budget.DailyPerCountryGroup != nil {
		variants++
	}
	if variants != 1 {
		return false
	}
	if budget.Total != nil && !validResponseMoney(*budget.Total, 50) || budget.DailySpent != nil && !validResponseMoney(*budget.DailySpent, 15) ||
		budget.Spent != nil && !validResponseMoney(*budget.Spent, 9) || budget.Daily != nil && !validResponseMoney(*budget.Daily, 15) {
		return false
	}
	if budget.DailyPerCountry != nil && !validCountryBudgetMap(budget.DailyPerCountry) {
		return false
	}
	return budget.DailyPerCountryGroup == nil || validCountryBudgetGroups(budget.DailyPerCountryGroup)
}

func validCountryBudgetMap(values map[CountryCode]Money) bool {
	if len(values) == 0 {
		return false
	}
	for country, amount := range values {
		if !validCountry(country) || !validPositiveMoney(amount) {
			return false
		}
	}
	return true
}

func validCountryBudgetGroups(groups []CountryBudgetGroup) bool {
	if len(groups) == 0 {
		return false
	}
	names := make(map[string]struct{}, len(groups))
	countries := make(map[CountryCode]struct{})
	for _, group := range groups {
		if !countryGroupNamePattern.MatchString(group.Name) || len(group.Name) > 40 || len(group.Countries) == 0 || !validPositiveMoney(group.Limit) {
			return false
		}
		if _, exists := names[group.Name]; exists {
			return false
		}
		names[group.Name] = struct{}{}
		for _, country := range group.Countries {
			if !validCountry(country) {
				return false
			}
			if _, exists := countries[country]; exists {
				return false
			}
			countries[country] = struct{}{}
		}
	}
	return true
}

func validResponseMoney(value Money, maximumIntegerDigits int) bool {
	text := string(value)
	if text == "" {
		return false
	}
	parts := splitMoney(text)
	return len(parts[0]) >= 1 && len(parts[0]) <= maximumIntegerDigits && digitsOnly(parts[0]) &&
		(len(parts[1]) == 0 || len(parts[1]) <= 2 && digitsOnly(parts[1]))
}

func splitMoney(value string) [2]string {
	for index, character := range value {
		if character == '.' {
			return [2]string{value[:index], value[index+1:]}
		}
	}
	return [2]string{value, ""}
}

func (budget *CampaignBudget) UnmarshalJSON(data []byte) error {
	return captureRaw(data, (*campaignBudgetAlias)(budget), &budget.Raw)
}

type campaignBudgetAlias CampaignBudget
