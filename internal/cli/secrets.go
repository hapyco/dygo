package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func newSecretsCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage encrypted dygo secrets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newSecretsInitCommand(stdout, stderr))
	cmd.AddCommand(newSecretsSetCommand(stdin, stdout))
	cmd.AddCommand(newSecretsGetCommand(stdout))
	cmd.AddCommand(newSecretsShowCommand(stdout))
	cmd.AddCommand(newSecretsListCommand(stdout))
	cmd.AddCommand(newSecretsEditCommand(ctx, stdin, stdout, stderr))
	cmd.AddCommand(newSecretsRemoveCommand(stdin, stdout))
	cmd.AddCommand(newSecretsValidateCommand(stdout))
	cmd.AddCommand(newSecretsRotateKeyCommand(stdout, stderr))

	return cmd
}

func newSecretsInitCommand(stdout, stderr io.Writer) *cobra.Command {
	var envName string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize encrypted secrets for an environment",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			paths, err := store.Init(env, force)
			if err != nil {
				return err
			}
			if env == secrets.EnvironmentProduction {
				if _, err := fmt.Fprintln(stderr, "warning: production private keys should usually come from CI/deployment secret storage"); err != nil {
					return fmt.Errorf("write warning: %w", err)
				}
			}
			if _, err := fmt.Fprintf(stdout, "initialized %s secrets\nkey: %s\nrecipient: %s\nfile: %s\n", env, rel(paths.KeyFile), rel(paths.RecipientFile), rel(paths.SecretFile)); err != nil {
				return fmt.Errorf("write init output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&force, "force", false, "Replace existing key, recipient, and encrypted file")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsSetCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	var envName string
	var value string
	var fromFile string

	cmd := &cobra.Command{
		Use:   "set NAME",
		Short: "Set an encrypted secret value",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			valueChanged := cmd.Flags().Changed("value")
			fromFileChanged := cmd.Flags().Changed("from-file")
			if valueChanged && fromFileChanged {
				return fmt.Errorf("--value and --from-file cannot be used together")
			}
			secretValue := value
			if fromFileChanged {
				data, err := os.ReadFile(fromFile)
				if err != nil {
					return fmt.Errorf("read secret value file: %w", err)
				}
				secretValue = string(data)
			}
			if !valueChanged && !fromFileChanged {
				var err error
				secretValue, err = readSecretValue(stdin, stdout)
				if err != nil {
					return err
				}
			}

			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			if err := store.Set(env, args[0], secretValue); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "set %s in %s\n", args[0], env); err != nil {
				return fmt.Errorf("write set output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.Flags().StringVar(&value, "value", "", "Secret value")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read secret value from file")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsGetCommand(stdout io.Writer) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "get NAME",
		Short: "Print a raw secret value for scripts",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			secret, err := store.Get(env, args[0])
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, secret.Value); err != nil {
				return fmt.Errorf("write secret value: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsShowCommand(stdout io.Writer) *cobra.Command {
	var envName string
	var reveal bool

	cmd := &cobra.Command{
		Use:   "show NAME",
		Short: "Show one secret with redaction by default",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			secret, err := store.Get(env, args[0])
			if err != nil {
				return err
			}
			value := secrets.Redact(secret.Value)
			if reveal {
				value = secret.Value
			}
			if _, err := fmt.Fprintf(stdout, "%s=%s\nupdated_at=%s\n", args[0], value, secret.UpdatedAt); err != nil {
				return fmt.Errorf("write secret output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&reveal, "reveal", false, "Print the raw secret value")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsListCommand(stdout io.Writer) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List encrypted secrets with redacted values",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			entries, err := store.List(env)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if _, err := fmt.Fprintf(stdout, "%s=%s updated_at=%s\n", entry.Name, secrets.Redact(entry.Secret.Value), entry.Secret.UpdatedAt); err != nil {
					return fmt.Errorf("write list output: %w", err)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsEditCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit decrypted secrets in $EDITOR and re-encrypt them",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			return runSecretsEditor(ctx, store, env, stdin, stdout, stderr)
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsRemoveCommand(stdin io.Reader, stdout io.Writer) *cobra.Command {
	var envName string
	var yes bool

	cmd := &cobra.Command{
		Use:   "remove NAME",
		Short: "Remove an encrypted secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			if !yes {
				ok, err := confirm(stdin, stdout, fmt.Sprintf("Remove %s from %s? [y/N] ", args[0], env))
				if err != nil {
					return err
				}
				if !ok {
					if _, err := fmt.Fprintln(stdout, "remove canceled"); err != nil {
						return fmt.Errorf("write remove output: %w", err)
					}
					return nil
				}
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			if err := store.Remove(env, args[0]); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "removed %s from %s\n", args[0], env); err != nil {
				return fmt.Errorf("write remove output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsValidateCommand(stdout io.Writer) *cobra.Command {
	var envName string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate encrypted secrets and config references",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			if err := store.Validate(env); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "%s secrets are valid\n", env); err != nil {
				return fmt.Errorf("write validate output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newSecretsRotateKeyCommand(stdout, stderr io.Writer) *cobra.Command {
	var envName string
	var force bool

	cmd := &cobra.Command{
		Use:   "rotate-key",
		Short: "Rotate the local age key and committed recipient",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, err := secrets.ParseEnvironment(envName)
			if err != nil {
				return err
			}
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			paths, err := store.RotateKey(env, force)
			if err != nil {
				return err
			}
			if env == secrets.EnvironmentProduction {
				if _, err := fmt.Fprintln(stderr, "warning: production private keys should usually come from CI/deployment secret storage"); err != nil {
					return fmt.Errorf("write warning: %w", err)
				}
			}
			if _, err := fmt.Fprintf(stdout, "rotated %s secrets key\nkey: %s\nrecipient: %s\nfile: %s\n", env, rel(paths.KeyFile), rel(paths.RecipientFile), rel(paths.SecretFile)); err != nil {
				return fmt.Errorf("write rotate-key output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment: development, staging, or production")
	cmd.Flags().BoolVar(&force, "force", false, "Create a new empty encrypted file if the existing file cannot be loaded")
	cmd.MarkFlagRequired("env")

	return cmd
}

func newWorkingSecretsStore() (secrets.Store, error) {
	root, err := workingRootPath()
	if err != nil {
		return secrets.Store{}, err
	}
	return secrets.NewStore(root), nil
}

func readSecretValue(stdin io.Reader, stdout io.Writer) (string, error) {
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		if _, err := fmt.Fprint(stdout, "Value: "); err != nil {
			return "", fmt.Errorf("write prompt: %w", err)
		}
		data, err := term.ReadPassword(int(file.Fd()))
		if _, writeErr := fmt.Fprintln(stdout); writeErr != nil {
			return "", fmt.Errorf("write prompt newline: %w", writeErr)
		}
		if err != nil {
			return "", fmt.Errorf("read secret value: %w", err)
		}
		return string(data), nil
	}

	reader := bufio.NewReader(stdin)
	value, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read secret value: %w", err)
	}
	return strings.TrimRight(value, "\r\n"), nil
}

func confirm(stdin io.Reader, stdout io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(stdout, prompt); err != nil {
		return false, fmt.Errorf("write confirmation prompt: %w", err)
	}
	reader := bufio.NewReader(stdin)
	answer, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func runSecretsEditor(ctx context.Context, store secrets.Store, env secrets.Environment, stdin io.Reader, stdout, stderr io.Writer) error {
	plaintext, err := store.Plaintext(env)
	if err != nil {
		return err
	}

	paths := store.Paths(env)
	if err := os.MkdirAll(paths.TempDir, 0o700); err != nil {
		return fmt.Errorf("create secrets temp dir: %w", err)
	}
	tempFile, err := os.CreateTemp(paths.TempDir, string(env)+"-*.yaml")
	if err != nil {
		return fmt.Errorf("create secrets temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.Write(plaintext); err != nil {
		tempFile.Close()
		return fmt.Errorf("write secrets temp file: %w", err)
	}
	if err := tempFile.Chmod(0o600); err != nil {
		tempFile.Close()
		return fmt.Errorf("secure secrets temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close secrets temp file: %w", err)
	}

	for {
		if err := openEditor(ctx, tempPath, stdin, stdout, stderr); err != nil {
			return err
		}
		data, err := os.ReadFile(tempPath)
		if err != nil {
			return fmt.Errorf("read edited secrets temp file: %w", err)
		}
		if err := store.SavePlaintext(env, data); err == nil {
			if _, err := fmt.Fprintf(stdout, "updated %s secrets\n", env); err != nil {
				return fmt.Errorf("write edit output: %w", err)
			}
			return nil
		} else {
			if _, writeErr := fmt.Fprintf(stderr, "invalid secrets document: %v\n", err); writeErr != nil {
				return fmt.Errorf("write validation error: %w", writeErr)
			}
		}
		retry, err := confirm(stdin, stdout, "Edit again? [y/N] ")
		if err != nil {
			return err
		}
		if !retry {
			return fmt.Errorf("edited secrets were not saved")
		}
	}
}

func openEditor(ctx context.Context, path string, stdin io.Reader, stdout, stderr io.Writer) error {
	editor := strings.TrimSpace(os.Getenv("EDITOR"))
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("EDITOR is empty")
	}
	args := append(parts[1:], path)
	cmd := exec.CommandContext(ctx, parts[0], args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run editor %q: %w", editor, err)
	}
	return nil
}

func rel(path string) string {
	return relToWorkingRoot(path)
}
