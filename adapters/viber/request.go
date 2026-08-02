package viber

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"social-hub/pkg/socialhub"
)

const maxRequestBytes = 30_000

func (c *Client) request(ctx context.Context, path string, input any, output statusCarrier, options ...socialhub.CallOption) error {
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return platformError("POST "+path, socialhub.CodeInvalidArgument, socialhub.ClassPermanent, err)
		}
		if len(encoded) > maxRequestBytes {
			return invalidArgument("POST "+path, "request JSON exceeds Viber's 30 kB limit")
		}
	}
	if err := c.api.JSON(ctx, http.MethodPost, path, nil, input, output, options...); err != nil {
		return err
	}
	status, message := output.viberStatus()
	if strings.TrimSpace(message) == "" || (status == 0 && !strings.EqualFold(strings.TrimSpace(message), "ok")) {
		return platformError("POST "+path, socialhub.CodePlatformError, socialhub.ClassPermanent, nil)
	}
	return mapStatus("POST "+path, http.StatusOK, status, message)
}
