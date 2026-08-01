package socialhub

import (
	"bytes"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Config is the serializable, multi-platform SDK configuration.
type Config struct {
	Version   int                    `json:"version" yaml:"version"`
	Defaults  DefaultsConfig         `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Platforms []AdapterConfig        `json:"platforms" yaml:"platforms"`
	Stores    map[string]StoreConfig `json:"stores,omitempty" yaml:"stores,omitempty"`
}

// DefaultsConfig contains cross-platform defaults.
type DefaultsConfig struct {
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// AdapterConfig configures one platform product and its accounts.
type AdapterConfig struct {
	Adapter  string          `json:"adapter" yaml:"adapter"`
	Product  string          `json:"product,omitempty" yaml:"product,omitempty"`
	Accounts []AccountConfig `json:"accounts" yaml:"accounts"`
	Settings map[string]any  `json:"settings,omitempty" yaml:"settings,omitempty"`
}

// AccountConfig configures one platform account. Credential values are always
// references and are resolved only at runtime.
type AccountConfig struct {
	ID             AccountID      `json:"id" yaml:"id"`
	ClientID       string         `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	AppID          string         `json:"app_id,omitempty" yaml:"app_id,omitempty"`
	SecretRef      string         `json:"secret_ref,omitempty" yaml:"secret_ref,omitempty"`
	AccessTokenRef string         `json:"access_token_ref,omitempty" yaml:"access_token_ref,omitempty"`
	TokenStore     string         `json:"token_store,omitempty" yaml:"token_store,omitempty"`
	Webhook        WebhookConfig  `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	Approval       ApprovalConfig `json:"approval,omitempty" yaml:"approval,omitempty"`
	Settings       map[string]any `json:"settings,omitempty" yaml:"settings,omitempty"`
}

// WebhookConfig contains secret references used for callback verification.
type WebhookConfig struct {
	SecretRef string `json:"secret_ref,omitempty" yaml:"secret_ref,omitempty"`
	TokenRef  string `json:"token_ref,omitempty" yaml:"token_ref,omitempty"`
	AESKeyRef string `json:"aes_key_ref,omitempty" yaml:"aes_key_ref,omitempty"`
}

// ApprovalConfig records externally granted account capabilities.
type ApprovalConfig struct {
	AccountType string   `json:"account_type,omitempty" yaml:"account_type,omitempty"`
	Scopes      []string `json:"scopes,omitempty" yaml:"scopes,omitempty"`
}

// StoreConfig configures a named external state store.
type StoreConfig struct {
	Type      string `json:"type" yaml:"type"`
	Address   string `json:"address,omitempty" yaml:"address,omitempty"`
	SecretRef string `json:"secret_ref,omitempty" yaml:"secret_ref,omitempty"`
}

// LoadConfig strictly decodes YAML or JSON configuration.
func LoadConfig(reader io.Reader) (Config, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("socialhub: decode config: %w", err)
	}
	if config.Version != 1 {
		return Config{}, fmt.Errorf("socialhub: unsupported config version %d", config.Version)
	}
	if len(config.Platforms) == 0 {
		return Config{}, fmt.Errorf("socialhub: at least one platform is required")
	}
	for i := range config.Platforms {
		if err := config.Platforms[i].Validate(); err != nil {
			return Config{}, fmt.Errorf("socialhub: platforms[%d]: %w", i, err)
		}
	}
	return config, nil
}

// Validate checks the fields shared by all adapters.
func (c AdapterConfig) Validate() error {
	if c.Adapter == "" {
		return fmt.Errorf("adapter is required")
	}
	if len(c.Accounts) == 0 {
		return fmt.Errorf("at least one account is required")
	}
	seen := make(map[AccountID]struct{}, len(c.Accounts))
	for i, account := range c.Accounts {
		if account.ID == "" {
			return fmt.Errorf("accounts[%d].id is required", i)
		}
		if _, exists := seen[account.ID]; exists {
			return fmt.Errorf("duplicate account id %q", account.ID)
		}
		seen[account.ID] = struct{}{}
	}
	return nil
}

// Account returns a configured account by ID.
func (c AdapterConfig) Account(id AccountID) (AccountConfig, bool) {
	for _, account := range c.Accounts {
		if account.ID == id {
			return account, true
		}
	}
	return AccountConfig{}, false
}

// DecodeSettings strictly decodes adapter-specific settings into a typed value.
func DecodeSettings(settings map[string]any, target any) error {
	encoded, err := yaml.Marshal(settings)
	if err != nil {
		return fmt.Errorf("socialhub: encode adapter settings: %w", err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(encoded))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("socialhub: decode adapter settings: %w", err)
	}
	return nil
}
