package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/dygo-dev/dygo/internal/auth"
	"github.com/dygo-dev/dygo/internal/db"
	"github.com/dygo-dev/dygo/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type defaultAdminSetupRunner struct{}

func (defaultAdminSetupRunner) SetupAdmin(ctx context.Context, databaseURL string, input auth.SetupAdminInput) (auth.User, error) {
	pool, err := db.OpenRuntimePool(ctx, databaseURL)
	if err != nil {
		return auth.User{}, err
	}
	defer pool.Close()
	return auth.NewService(pool).SetupAdmin(ctx, input)
}

func newSetupCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, setup adminSetupRunner) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up dygo runtime accounts",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(newSetupAdminCommand(ctx, stdin, stdout, stderr, setup))
	return cmd
}

func newSetupAdminCommand(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, setup adminSetupRunner) *cobra.Command {
	envName := string(secrets.EnvironmentDevelopment)
	email := ""
	fullName := ""
	passwordStdin := false

	cmd := &cobra.Command{
		Use:   "admin",
		Short: "Create the first Administrator account",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			env, _, databaseURL, err := databaseInputs(envName)
			if err != nil {
				return err
			}
			reader := bufio.NewReader(stdin)
			inputEmail, err := promptValue(reader, stderr, "Admin email", email)
			if err != nil {
				return err
			}
			inputFullName, err := promptValue(reader, stderr, "Admin full name", fullName)
			if err != nil {
				return err
			}
			password, err := adminPassword(stdin, reader, stderr, passwordStdin)
			if err != nil {
				return err
			}

			user, err := setup.SetupAdmin(ctx, databaseURL, auth.SetupAdminInput{
				Email:    inputEmail,
				FullName: inputFullName,
				Password: password,
			})
			if err != nil {
				return fmt.Errorf("setup administrator account: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "administrator account ready: %s (%s)\n", user.Email, env); err != nil {
				return fmt.Errorf("write setup admin output: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", envName, "Environment: development, staging, or production")
	cmd.Flags().StringVar(&email, "email", email, "Administrator email")
	cmd.Flags().StringVar(&fullName, "full-name", fullName, "Administrator full name")
	cmd.Flags().BoolVar(&passwordStdin, "password-stdin", passwordStdin, "Read the Administrator password from stdin")
	return cmd
}

func promptValue(reader *bufio.Reader, stderr io.Writer, label string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value != "" {
		return value, nil
	}
	if _, err := fmt.Fprintf(stderr, "%s: ", label); err != nil {
		return "", fmt.Errorf("write prompt: %w", err)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read %s: %w", strings.ToLower(label), err)
	}
	return strings.TrimSpace(line), nil
}

func adminPassword(stdin io.Reader, reader *bufio.Reader, stderr io.Writer, passwordStdin bool) (string, error) {
	if passwordStdin {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", fmt.Errorf("read admin password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}
	if _, err := fmt.Fprint(stderr, "Admin password: "); err != nil {
		return "", fmt.Errorf("write password prompt: %w", err)
	}
	if file, ok := stdin.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		password, err := term.ReadPassword(int(file.Fd()))
		if _, printErr := fmt.Fprintln(stderr); printErr != nil && err == nil {
			return "", fmt.Errorf("write password prompt newline: %w", printErr)
		}
		if err != nil {
			return "", fmt.Errorf("read admin password: %w", err)
		}
		return string(password), nil
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read admin password: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}
