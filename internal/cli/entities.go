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

	cmd.AddCommand(newEntitiesListCommand(stdout))
	cmd.AddCommand(newEntitiesValidateCommand(stdout))

	return cmd
}

func newEntitiesListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered dygo entities",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			apps, err := appregistry.New(".").Validate()
			if err != nil {
				return fmt.Errorf("validate apps: %w", err)
			}
			if len(apps) == 0 {
				if _, err := fmt.Fprintln(stdout, "No apps found."); err != nil {
					return fmt.Errorf("write entities output: %w", err)
				}
				return nil
			}

			entities, err := catalog.New(apps, fieldtype.DefaultRegistry()).Validate()
			if err != nil {
				return fmt.Errorf("validate entities: %w", err)
			}

			entitiesByApp := make(map[string][]catalog.LoadedEntity)
			for _, entity := range entities {
				entitiesByApp[entity.AppName] = append(entitiesByApp[entity.AppName], entity)
			}

			for _, app := range apps {
				if _, err := fmt.Fprintln(stdout, app.Manifest.Name); err != nil {
					return fmt.Errorf("write app name: %w", err)
				}

				appEntities := entitiesByApp[app.Manifest.Name]
				if len(appEntities) == 0 {
					if _, err := fmt.Fprintln(stdout, "  (no entities)"); err != nil {
						return fmt.Errorf("write empty app entities: %w", err)
					}
					continue
				}

				for _, entity := range appEntities {
					if _, err := fmt.Fprintf(stdout, "  - %s\n", entity.Entity.Name); err != nil {
						return fmt.Errorf("write entity name: %w", err)
					}
				}
			}

			return nil
		},
	}
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
