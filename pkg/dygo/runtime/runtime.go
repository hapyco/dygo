// Package runtime lets compiled dygo projects run the stock CLI with app extensions.
package runtime

import (
	"context"
	"io"

	"github.com/hapyco/dygo/internal/cli"
	"github.com/hapyco/dygo/pkg/dygo"
)

// Options configures the compiled dygo runtime.
type Options struct {
	RecordHooks []dygo.RecordHookRegistrar
	Jobs        []dygo.JobRegistrar
}

// Run executes the dygo CLI with compiled app extensions.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, options Options) error {
	return cli.RunWithOptions(ctx, args, stdin, stdout, stderr, cli.Options{
		RecordHooks: options.RecordHooks,
		Jobs:        options.Jobs,
	})
}
