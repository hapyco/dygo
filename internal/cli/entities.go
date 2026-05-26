package cli

import (
	"fmt"
	"io"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/project"
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
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			metadata, err := project.LoadMetadata(root)
			if err != nil {
				return err
			}
			if len(metadata.Apps) == 0 {
				if _, err := fmt.Fprintln(stdout, "No apps found."); err != nil {
					return fmt.Errorf("write entities output: %w", err)
				}
				return nil
			}

			entitiesByApp := make(map[string][]catalog.LoadedEntity)
			for _, entity := range metadata.Entities {
				entitiesByApp[entity.AppName] = append(entitiesByApp[entity.AppName], entity)
			}

			for _, app := range metadata.Apps {
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
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			metadata, err := project.LoadMetadata(root)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(stdout, "%d entities are valid\n", len(metadata.Entities)); err != nil {
				return fmt.Errorf("write entities validation output: %w", err)
			}
			return nil
		},
	}
}
