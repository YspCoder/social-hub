package socialhub

import (
	"strings"
	"testing"
)

func TestLoadConfigYAMLAndJSON(t *testing.T) {
	t.Parallel()
	for name, input := range map[string]string{
		"yaml": "version: 1\nplatforms:\n  - adapter: x/v2\n    accounts:\n      - id: primary\n        access_token_ref: env://X_TOKEN\n",
		"json": `{"version":1,"platforms":[{"adapter":"x/v2","accounts":[{"id":"primary","access_token_ref":"env://X_TOKEN"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			config, err := LoadConfig(strings.NewReader(input))
			if err != nil {
				t.Fatal(err)
			}
			if got := config.Platforms[0].Accounts[0].ID; got != "primary" {
				t.Fatalf("account ID = %q", got)
			}
		})
	}
}

func TestLoadConfigRejectsUnknownAndDuplicateAccounts(t *testing.T) {
	t.Parallel()
	tests := []string{
		"version: 1\nunknown: true\nplatforms:\n  - adapter: x/v2\n    accounts:\n      - id: one\n",
		"version: 1\nplatforms:\n  - adapter: x/v2\n    accounts:\n      - id: same\n      - id: same\n",
	}
	for _, input := range tests {
		if _, err := LoadConfig(strings.NewReader(input)); err == nil {
			t.Fatalf("configuration should fail:\n%s", input)
		}
	}
}

func TestDecodeSettingsIsStrict(t *testing.T) {
	t.Parallel()
	type settings struct {
		BaseURL string `yaml:"base_url"`
	}
	var value settings
	if err := DecodeSettings(map[string]any{"base_url": "https://example.com"}, &value); err != nil {
		t.Fatal(err)
	}
	if err := DecodeSettings(map[string]any{"typo": true}, &value); err == nil {
		t.Fatal("unknown adapter setting should fail")
	}
}
