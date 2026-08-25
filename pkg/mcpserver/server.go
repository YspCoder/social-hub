// Package mcpserver exposes configured social-hub accounts through the Model
// Context Protocol without exposing platform credentials.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"social-hub/pkg/socialhub"
)

const serverInstructions = "Use socialhub_list_targets before selecting an adapter and account_id. Read capabilities before calling an account operation. Mutation tools are registered only when deployment policy allows them; obtain explicit user confirmation before any write or delete. Never request or pass credentials. Capability groups do not guarantee every method is supported. Respect retry_after_ms."

// Operation identifies a mutation that a deployer may explicitly enable.
type Operation string

const (
	OperationPublishPost    Operation = "publish_post"
	OperationDeletePost     Operation = "delete_post"
	OperationAddReaction    Operation = "add_reaction"
	OperationRemoveReaction Operation = "remove_reaction"
	OperationCreateComment  Operation = "create_comment"
	OperationDeleteComment  Operation = "delete_comment"
	OperationSendMessage    Operation = "send_message"
)

var writeOperations = map[Operation]struct{}{
	OperationPublishPost:   {},
	OperationAddReaction:   {},
	OperationCreateComment: {},
	OperationSendMessage:   {},
}

var destructiveOperations = map[Operation]struct{}{
	OperationDeletePost:     {},
	OperationRemoveReaction: {},
	OperationDeleteComment:  {},
}

// Policy controls which mutation tools are registered. Read tools are always
// registered. The zero value is read-only.
type Policy struct {
	AllowedWrites      []Operation
	AllowedDestructive []Operation
}

func (p Policy) validate() error {
	for _, operation := range p.AllowedWrites {
		if _, ok := writeOperations[operation]; !ok {
			return fmt.Errorf("mcpserver: %q is not an additive write operation", operation)
		}
	}
	for _, operation := range p.AllowedDestructive {
		if _, ok := destructiveOperations[operation]; !ok {
			return fmt.Errorf("mcpserver: %q is not a destructive operation", operation)
		}
	}
	return nil
}

func (p Policy) allows(operation Operation) bool {
	operations := p.AllowedWrites
	if _, destructive := destructiveOperations[operation]; destructive {
		operations = p.AllowedDestructive
	}
	for _, allowed := range operations {
		if operation == allowed {
			return true
		}
	}
	return false
}

type targetKey struct {
	adapter string
	account socialhub.AccountID
}

// Service owns initialized adapters and lazily-created account clients.
type Service struct {
	mu       sync.Mutex
	policy   Policy
	adapters map[string]socialhub.Adapter
	clients  map[targetKey]socialhub.Client
	targets  map[targetKey]TargetInfo
	closed   bool
}

// New initializes every configured adapter. Adapter packages must be imported
// by the caller so their init functions can register them with social-hub.
func New(ctx context.Context, config socialhub.Config, policy Policy, options ...socialhub.Option) (*Service, error) {
	if err := policy.validate(); err != nil {
		return nil, err
	}
	service := &Service{
		policy:   policy,
		adapters: make(map[string]socialhub.Adapter, len(config.Platforms)),
		clients:  make(map[targetKey]socialhub.Client),
		targets:  make(map[targetKey]TargetInfo),
	}
	for _, adapterConfig := range config.Platforms {
		if _, duplicate := service.adapters[adapterConfig.Adapter]; duplicate {
			_ = service.Close()
			return nil, fmt.Errorf("mcpserver: duplicate adapter configuration %q", adapterConfig.Adapter)
		}
		adapter, err := socialhub.Open(ctx, adapterConfig.Adapter, adapterConfig, options...)
		if err != nil {
			_ = service.Close()
			return nil, fmt.Errorf("mcpserver: initialize adapter %q: %w", adapterConfig.Adapter, err)
		}
		service.adapters[adapterConfig.Adapter] = adapter
		metadata := adapter.Metadata()
		product := adapterConfig.Product
		if product == "" {
			product = metadata.Product
		}
		for _, account := range adapterConfig.Accounts {
			key := targetKey{adapter: adapterConfig.Adapter, account: account.ID}
			service.targets[key] = TargetInfo{
				Adapter:    adapterConfig.Adapter,
				AccountID:  string(account.ID),
				Product:    product,
				APIVersion: metadata.APIVersion,
				DocURL:     metadata.DocURL,
			}
		}
	}
	return service, nil
}

// MCPServer creates a protocol server backed by the service.
func (s *Service) MCPServer(version string) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "social-hub", Version: version},
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	s.registerReadTools(server)
	s.registerMutationTools(server)
	return server
}

func (s *Service) targetList() []TargetInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	targets := make([]TargetInfo, 0, len(s.targets))
	for _, target := range s.targets {
		targets = append(targets, target)
	}
	sort.Slice(targets, func(i, j int) bool {
		if targets[i].Adapter == targets[j].Adapter {
			return targets[i].AccountID < targets[j].AccountID
		}
		return targets[i].Adapter < targets[j].Adapter
	})
	return targets
}

func (s *Service) client(ctx context.Context, target TargetRef) (socialhub.Client, error) {
	if target.Adapter == "" || target.AccountID == "" {
		return nil, invalidArgument("target", "adapter and account_id are required")
	}
	key := targetKey{adapter: target.Adapter, account: socialhub.AccountID(target.AccountID)}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, &socialhub.Error{Code: socialhub.CodeConflict, Class: socialhub.ClassPermanent, Op: "mcp_client"}
	}
	if _, configured := s.targets[key]; !configured {
		return nil, &socialhub.Error{Code: socialhub.CodeNotFound, Class: socialhub.ClassPermanent, Op: "target"}
	}
	if client := s.clients[key]; client != nil {
		return client, nil
	}
	client, err := s.adapters[target.Adapter].Client(ctx, key.account)
	if err != nil {
		return nil, err
	}
	s.clients[key] = client
	return client, nil
}

// Close releases every lazily-created client, then each initialized adapter.
func (s *Service) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	clients := make([]socialhub.Client, 0, len(s.clients))
	for _, client := range s.clients {
		clients = append(clients, client)
	}
	adapters := make([]socialhub.Adapter, 0, len(s.adapters))
	for _, adapter := range s.adapters {
		adapters = append(adapters, adapter)
	}
	s.mu.Unlock()

	var closeErrors []error
	for _, client := range clients {
		if err := client.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	for _, adapter := range adapters {
		if err := adapter.Close(); err != nil {
			closeErrors = append(closeErrors, err)
		}
	}
	return errors.Join(closeErrors...)
}

func invalidArgument(op, message string) error {
	return &socialhub.Error{
		Code:            socialhub.CodeInvalidArgument,
		Class:           socialhub.ClassPermanent,
		Op:              op,
		PlatformMessage: message,
	}
}
