package outbrain

import (
	"context"
	"net/url"

	"social-hub/pkg/socialhub"
)

func (client *Client) ListBudgets(ctx context.Context, detachedOnly bool, options ...socialhub.CallOption) ([]Budget, error) {
	var envelope struct {
		Budgets []Budget `json:"budgets"`
		Count   int      `json:"count"`
	}
	query := make(url.Values)
	if detachedOnly {
		query.Set("detachedOnly", "true")
	}
	path := "marketers/" + url.PathEscape(client.marketerID) + "/budgets"
	if err := client.getJSON(ctx, "list_budgets", path, query, &envelope, options...); err != nil {
		return nil, err
	}
	if envelope.Count != len(envelope.Budgets) {
		return nil, platformContractError("list_budgets", "budget count does not match results")
	}
	for _, budget := range envelope.Budgets {
		if !validBudgetResponse(budget) {
			return nil, platformContractError("list_budgets", "invalid Budget response")
		}
	}
	return envelope.Budgets, nil
}

func (client *Client) GetBudget(ctx context.Context, budgetID string, options ...socialhub.CallOption) (Budget, error) {
	if !validPathID(budgetID) {
		return Budget{}, invalidArgument("get_budget", "budget ID is invalid")
	}
	if _, err := client.ensureBudgetOwned(ctx, budgetID, options...); err != nil {
		return Budget{}, err
	}
	var budget Budget
	if err := client.getJSON(ctx, "get_budget", "budgets/"+url.PathEscape(budgetID), nil, &budget, options...); err != nil {
		return Budget{}, err
	}
	if budget.ID != budgetID || !validBudgetResponse(budget) {
		return Budget{}, platformContractError("get_budget", "Budget response does not match request")
	}
	return budget, nil
}

func (client *Client) CreateBudget(ctx context.Context, input CreateBudgetRequest, options ...socialhub.CallOption) (Budget, error) {
	if !validCreateBudget(input) {
		return Budget{}, invalidArgument("create_budget", "invalid Budget fields or current minimum amount")
	}
	var budget Budget
	path := "marketers/" + url.PathEscape(client.marketerID) + "/budgets"
	if err := client.postJSON(ctx, "create_budget", path, input, &budget, options...); err != nil {
		return Budget{}, err
	}
	if !validBudgetResponse(budget) || budget.Name != input.Name || budget.Type != input.Type || budget.Pacing != input.Pacing {
		return Budget{}, platformContractError("create_budget", "Budget response does not match request")
	}
	return budget, nil
}

func (client *Client) UpdateBudget(ctx context.Context, budgetID string, input UpdateBudgetRequest, options ...socialhub.CallOption) (Budget, error) {
	if !validPathID(budgetID) || !validUpdateBudget(input) {
		return Budget{}, invalidArgument("update_budget", "budget ID or update fields are invalid")
	}
	current, err := client.ensureBudgetOwned(ctx, budgetID, options...)
	if err != nil {
		return Budget{}, err
	}
	if !validBudgetUpdateAgainstCurrent(current, input) {
		return Budget{}, invalidArgument("update_budget", "updated Budget would violate the current minimum or pacing requirements")
	}
	var budget Budget
	if err := client.putJSON(ctx, "update_budget", "budgets/"+url.PathEscape(budgetID), input, &budget, options...); err != nil {
		return Budget{}, err
	}
	if budget.ID != budgetID || !validBudgetResponse(budget) {
		return Budget{}, platformContractError("update_budget", "Budget response does not match request")
	}
	return budget, nil
}

func (client *Client) ensureBudgetOwned(ctx context.Context, budgetID string, options ...socialhub.CallOption) (Budget, error) {
	budgets, err := client.ListBudgets(ctx, false, options...)
	if err != nil {
		return Budget{}, err
	}
	for _, budget := range budgets {
		if budget.ID == budgetID {
			return budget, nil
		}
	}
	return Budget{}, platformError("validate_budget_owner", socialhub.CodePermissionDenied, socialhub.ClassUserAction, nil)
}

func validBudgetResponse(budget Budget) bool {
	return validPathID(budget.ID) && validText(budget.Name, 1024) && budget.Amount > 0 &&
		validBudgetType(budget.Type) && validPacingType(budget.Pacing)
}

func validCreateBudget(input CreateBudgetRequest) bool {
	if !validText(input.Name, 1024) || !validDate(input.StartDate) ||
		!validPacingType(input.Pacing) || !validBudgetType(input.Type) || !validBudgetMinimum(input) ||
		!validPositive(input.DailyTarget) {
		return false
	}
	if input.EndDate != "" && !validDateWindow(input.StartDate, input.EndDate) {
		return false
	}
	if input.Pacing == PacingDailyTarget {
		return input.DailyTarget != nil
	}
	return input.DailyTarget == nil
}

func validUpdateBudget(input UpdateBudgetRequest) bool {
	if input.Name == nil && input.Amount == nil && input.EndDate == nil && input.Pacing == nil && input.DailyTarget == nil {
		return false
	}
	if !validOptionalText(input.Name, 1024) || !validPositive(input.Amount) || !validOptionalDate(input.EndDate) ||
		!validPositive(input.DailyTarget) {
		return false
	}
	return input.Pacing == nil || validPacingType(*input.Pacing)
}

func validBudgetUpdateAgainstCurrent(current Budget, input UpdateBudgetRequest) bool {
	candidate := CreateBudgetRequest{
		Name: current.Name, Amount: current.Amount, StartDate: current.StartDate, EndDate: current.EndDate,
		Pacing: current.Pacing, Type: current.Type,
	}
	if current.DailyTarget > 0 {
		candidate.DailyTarget = &current.DailyTarget
	}
	if input.Name != nil {
		candidate.Name = *input.Name
	}
	if input.Amount != nil {
		candidate.Amount = *input.Amount
	}
	if input.EndDate != nil {
		candidate.EndDate = *input.EndDate
	}
	if input.Pacing != nil {
		candidate.Pacing = *input.Pacing
	}
	if input.DailyTarget != nil {
		candidate.DailyTarget = input.DailyTarget
	}
	return validCreateBudget(candidate)
}
