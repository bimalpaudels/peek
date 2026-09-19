package runner

import (
	"context"
)

// Runner represents a language-specific block execution engine.
type Runner interface {
	// ExecuteBlocks executes upstream blocks silently, then executes targetBlock with output capture.
	ExecuteBlocks(ctx context.Context, filePath string, upstreamBlocks []string, targetBlock string, maxLines int) ([]string, error)
}

