package cli

import (
	"fmt"
	"io"

	appregistry "github.com/dygo-dev/dygo/internal/app/registry"
	"github.com/dygo-dev/dygo/internal/entity/catalog"
	"github.com/dygo-dev/dygo/internal/entity/fieldtype"
	"github.com/spf13/cobra"
)

func newEntitiesCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entities",
		Short: "Manage dygo entities",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newEntitiesValidateCommand(stdout))

	return cmd
}

func newEntitiesValidateCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate discovered dygo entities",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			apps, err := appregistry.New(".").Validate()
			if err != nil {
				return fmt.Errorf("validate apps: %w", err)
			}

			entities, err := catalog.New(apps, fieldtype.DefaultRegistry()).Validate()
			if err != nil {
				return fmt.Errorf("validate entities: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "%d entities are valid\n", len(entities)); err != nil {
				return fmt.Errorf("write entities validation output: %w", err)
			}
			return nil
		},
	}
}
