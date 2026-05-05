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

	cmd.AddCommand(newSecretsInitCommand(stdout))
	cmd.AddCommand(newSecretsEditCommand(ctx, stdin, stdout, stderr))
	cmd.AddCommand(newSecretsValidateCommand(stdout))
	cmd.AddCommand(newSecretsRotateKeyCommand(stdout))

	return cmd
}

func newSecretsInitCommand(stdout io.Writer) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize the root master key and encrypted secrets files",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			paths, err := store.Init(force)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "initialized secrets\nkey: %s\n%s", rel(paths.MasterKeyFile), formatSecretFiles(store)); err != nil {
				return fmt.Errorf("write init output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Replace existing master key and re-encrypt existing secret files")

	return cmd
}

func newSecretsEditCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	var editor string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit decrypted secrets in an editor and re-encrypt them",
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
			return runSecretsEditor(ctx, store, env, editor, stdin, stdout, stderr)
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&editor, "editor", "", "Editor command, for example: nano or \"code --wait\"")

	return cmd
}

func newSecretsValidateCommand(stdout io.Writer) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)

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

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")

	return cmd
}

func newSecretsRotateKeyCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotate-key",
		Short: "Rotate the root master key and re-encrypt all secrets",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			store, err := newWorkingSecretsStore()
			if err != nil {
				return err
			}
			paths, err := store.RotateKey()
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "rotated secrets master key\nkey: %s\n%s", rel(paths.MasterKeyFile), formatSecretFiles(store)); err != nil {
				return fmt.Errorf("write rotate-key output: %w", err)
			}
			return nil
		},
	}

	return cmd
}

func newWorkingSecretsStore() (secrets.Store, error) {
	root, err := workingRootPath()
	if err != nil {
		return secrets.Store{}, err
	}
	return secrets.NewStore(root), nil
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

func runSecretsEditor(ctx context.Context, store secrets.Store, env secrets.Environment, editor string, stdin io.Reader, stdout, stderr io.Writer) error {
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
		if err := openEditor(ctx, editor, tempPath, stdin, stdout, stderr); err != nil {
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

func openEditor(ctx context.Context, editor string, path string, stdin io.Reader, stdout, stderr io.Writer) error {
	editor = strings.TrimSpace(editor)
	if editor == "" {
		editor = "nano"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("editor is empty")
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

func formatSecretFiles(store secrets.Store) string {
	var builder strings.Builder
	builder.WriteString("files:\n")
	for _, env := range secrets.SupportedEnvironments() {
		builder.WriteString("  ")
		builder.WriteString(rel(store.Paths(env).SecretFile))
		builder.WriteString("\n")
	}
	return builder.String()
}

func rel(path string) string {
	return relToWorkingRoot(path)
}
