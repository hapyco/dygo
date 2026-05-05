---
name: go-cli-cobra-viper
description: Use when designing, implementing, or reviewing dygo CLI commands in Go with Cobra, and when deciding whether Viper-style configuration precedence is needed. Covers command structure, flags, config precedence, shell completion, output conventions, and CLI tests.
---

# Go CLI Development With Cobra And Viper

Use this skill for dygo CLI work: root commands, command groups, subcommands, flags, help output, diagnostics, scaffolding commands, shell completion, and future config plumbing.

## dygo Conventions

Prefer dygo's existing Cobra shape:

- `cmd/dygo/main.go` stays tiny and calls `cli.Run`.
- CLI implementation lives under `internal/cli`.
- Root construction goes through `NewRootCommand(ctx, stdin, stdout, stderr)`.
- Commands receive injected `stdin`, `stdout`, and `stderr`; do not write directly to `os.Stdout` or `os.Stderr` unless the command API requires it.
- Root command should keep `SilenceUsage: true` and `SilenceErrors: true`.
- Command functions should return `*cobra.Command` and keep command logic small.
- Use `RunE`, `PreRunE`, and `PostRunE`, not `Run`, so errors propagate.
- Every command must set `Args`.
- Use `workingRootPath()` for project-aware commands that read apps, configs, secrets, or metadata.
- Keep command output plain ASCII, stable, and easy for humans and agents to parse.

Current command naming style is resource-first:

```txt
dygo apps list
dygo apps validate
dygo entities list
dygo entities validate
dygo secrets set --env development NAME
dygo doctor
```

Keep generators late. Build manual structure first, let conventions stabilize, document the shape, then turn the final shape into `dygo generate` or `dygo new`.

## Cobra Rules

Use Cobra for multi-command CLI work.

Good command pattern:

```go
func newExampleCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "example",
		Short: "Do one clear thing",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if _, err := fmt.Fprintln(stdout, "done"); err != nil {
				return fmt.Errorf("write example output: %w", err)
			}
			return nil
		},
	}
}
```

Use persistent flags only for behavior that applies to a command and all descendants, such as a future global `--config`, `--verbose`, or `--output`.

Use local flags for command-specific options.

Prefer these helpers when applicable:

- `cobra.NoArgs`
- `cobra.ExactArgs(n)`
- `cobra.ExactValidArgs(n)`
- `ValidArgs` or `ValidArgsFunction` for known values
- `MarkFlagRequired`
- `MarkFlagsRequiredTogether`
- `MarkFlagsMutuallyExclusive`

## Viper Rules

Do not add Viper just because a command has flags.

Use Viper when dygo needs unified configuration precedence across flags, environment variables, config files, and defaults.

When Viper is introduced, keep precedence explicit:

```txt
flags > environment variables > config files > defaults
```

Use a dygo-specific environment prefix:

```txt
DYGO_
```

Bind every Cobra flag that should participate in config precedence:

```go
cmd.Flags().String("region", "us-east-1", "region")
if err := viper.BindPFlag("region", cmd.Flags().Lookup("region")); err != nil {
	return err
}
```

Prefer unmarshalling Viper config into typed dygo config structs at the boundary. Avoid passing `viper.Get...` calls deep into framework internals.

## Shell Completion

Cobra already provides a generated `completion` command. Add custom completions when values are known and useful, such as environments, app names, entity names, or field types.

Use `ValidArgsFunction` or `RegisterFlagCompletionFunc` for dynamic completions.

Keep completion functions filesystem-only and fast. They should not write config, call remote services, or perform expensive validation.

## Output And Errors

Separate user output from errors and logs:

- User-facing normal output goes to `stdout`.
- Prompts, warnings, and diagnostics may use `stderr` when appropriate.
- Secret values must stay redacted unless a command explicitly reveals them.
- Errors should be actionable and wrapped with context using `%w`.

Good error shape:

```go
return fmt.Errorf("validate apps: %w", err)
```

For validation-style commands, prefer printing all relevant problems before returning a nonzero error when that makes the next fix clearer.

## Testing

Every new CLI command needs tests.

Use the existing `Run(context.Background(), args, stdin, stdout, stderr)` test pattern with buffers.

Test at least:

- command success output
- invalid args or invalid metadata
- help or command registration when adding a new root command
- nested directory behavior for project-aware commands
- stderr behavior when warnings are expected

Run before finishing CLI work:

```sh
go test ./internal/cli
go test ./...
go vet ./...
go test -race ./...
```

For root command changes, also verify help manually:

```sh
go run ./cmd/dygo --help
```

## Anti-Patterns

Avoid:

- using `Run` instead of `RunE`
- writing directly to global stdout/stderr in command logic
- adding Viper before config precedence is actually needed
- forgetting to bind Cobra flags to Viper after Viper is introduced
- command output that changes shape unnecessarily
- hiding validation context behind a single vague error
- adding generators before manual structure has stabilized
