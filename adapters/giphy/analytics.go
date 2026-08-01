package giphy

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"social-hub/pkg/socialhub"
)

// Register sends one response-provided GIPHY analytics pingback.
func (client *Client) Register(ctx context.Context, input AnalyticsRequest, options ...socialhub.CallOption) error {
	if !validOpaque(input.CustomerID, 512) {
		return invalidArgument("analytics_register", "customer ID is required")
	}
	parsed, err := url.Parse(input.TrackingURL)
	if err != nil || parsed.User != nil || parsed.Fragment != "" || parsed.Path != "/v2/pingback_simple" || normalizedURLOrigin(parsed) != client.analyticsOrigin {
		return invalidArgument("analytics_register", "tracking URL must use the configured GIPHY analytics origin")
	}
	query := parsed.Query()
	action := query.Get("action_type")
	if query.Get("analytics_response_payload") == "" || action != "SEEN" && action != "CLICK" && action != "SENT" {
		return invalidArgument("analytics_register", "tracking URL is missing GIPHY analytics parameters")
	}
	timestamp := input.Timestamp
	if timestamp.IsZero() {
		timestamp = client.clock.Now()
	}
	if timestamp.UnixMilli() <= 0 {
		return invalidArgument("analytics_register", "timestamp must be after the Unix epoch")
	}
	query.Set("customer_id", input.CustomerID)
	query.Set("ts", strconv.FormatInt(timestamp.UnixMilli(), 10))
	return client.analytics.JSON(ctx, http.MethodGet, parsed.Path, query, nil, nil, options...)
}

func normalizedURLOrigin(parsed *url.URL) string {
	if parsed == nil {
		return ""
	}
	return strings.ToLower(parsed.Scheme + "://" + parsed.Host)
}

var _ AnalyticsWorkflow = (*Client)(nil)
