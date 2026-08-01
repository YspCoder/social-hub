package socialhub

import (
	"context"
	"fmt"
	"os"
	"strings"
)

// SecretResolver resolves an opaque credential reference at runtime.
type SecretResolver interface {
	Resolve(context.Context, string) (string, error)
}

// EnvironmentSecretResolver supports env://NAME references. File and external
// vault resolvers can be supplied with WithSecretResolver.
type EnvironmentSecretResolver struct{}

// Resolve returns the referenced environment variable.
func (EnvironmentSecretResolver) Resolve(_ context.Context, reference string) (string, error) {
	const prefix = "env://"
	if !strings.HasPrefix(reference, prefix) {
		return "", fmt.Errorf("socialhub: unsupported secret reference scheme")
	}
	name := strings.TrimPrefix(reference, prefix)
	if name == "" {
		return "", fmt.Errorf("socialhub: empty environment secret reference")
	}
	value, exists := os.LookupEnv(name)
	if !exists || value == "" {
		return "", fmt.Errorf("socialhub: secret reference is not set")
	}
	return value, nil
}
