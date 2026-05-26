package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/hapyco/dygo/internal/entity/catalog"
	"github.com/hapyco/dygo/internal/project"
	"github.com/hapyco/dygo/internal/shape"
	"github.com/spf13/cobra"
)

type entityRelation struct {
	From      catalog.LoadedEntity
	FieldName string
	Kind      string
	TargetApp string
	Target    string
}

func newEntityCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entity",
		Short: "Manage dygo entities",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newEntitiesListCommand(stdout))
	cmd.AddCommand(newEntitiesValidateCommand(stdout))
	cmd.AddCommand(newEntityShowCommand(stdout))
	cmd.AddCommand(newEntityGraphCommand(stdout))

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

func newEntityShowCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "show <app>/<entity>",
		Short: "Show resolved metadata for one dygo Entity",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			ref, err := shape.ParseAppRef(args[0])
			if err != nil {
				return err
			}
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			metadata, err := project.LoadMetadata(root)
			if err != nil {
				return err
			}
			entity, ok := findEntity(metadata.Entities, ref)
			if !ok {
				return fmt.Errorf("entity %q was not found", args[0])
			}
			return writeEntityShow(stdout, entity)
		},
	}
}

func newEntityGraphCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "graph [<app>|<app>/<entity>]",
		Short: "Print dygo Entity relationships",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			metadata, err := project.LoadMetadata(root)
			if err != nil {
				return err
			}
			entities, err := graphScope(metadata.Entities, args)
			if err != nil {
				return err
			}
			relations := entityRelations(metadata.Entities)
			return writeEntityGraph(stdout, entities, relations)
		},
	}
}

func writeEntityShow(stdout io.Writer, entity catalog.LoadedEntity) error {
	if _, err := fmt.Fprintf(stdout, "entity: %s/%s\n", entity.AppName, entity.Entity.Name); err != nil {
		return fmt.Errorf("write entity show output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "kind: %s\n", entityKind(entity)); err != nil {
		return fmt.Errorf("write entity show output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "path: %s\n", relToWorkingRoot(entity.Path)); err != nil {
		return fmt.Errorf("write entity show output: %w", err)
	}
	if route := entity.RouteSlug(); route != "" {
		if _, err := fmt.Fprintf(stdout, "route: /%s\n", route); err != nil {
			return fmt.Errorf("write entity show output: %w", err)
		}
	} else if _, err := fmt.Fprintln(stdout, "route: (none)"); err != nil {
		return fmt.Errorf("write entity show output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "naming: %s\n", entity.Entity.EffectiveNaming().Strategy); err != nil {
		return fmt.Errorf("write entity show output: %w", err)
	}
	if _, err := fmt.Fprintln(stdout, "fields:"); err != nil {
		return fmt.Errorf("write entity show output: %w", err)
	}
	if len(entity.Entity.Fields) == 0 {
		if _, err := fmt.Fprintln(stdout, "  (none)"); err != nil {
			return fmt.Errorf("write entity show output: %w", err)
		}
		return nil
	}
	for _, field := range entity.Entity.Fields {
		target := ""
		if field.Options.Entity != "" {
			targetApp := field.Options.App
			if targetApp == "" {
				targetApp = entity.AppName
			}
			target = fmt.Sprintf(" -> %s/%s", targetApp, field.Options.Entity)
		}
		if _, err := fmt.Fprintf(stdout, "  - %s: %s%s\n", field.Name, field.Type, target); err != nil {
			return fmt.Errorf("write entity show output: %w", err)
		}
	}
	return nil
}

func writeEntityGraph(stdout io.Writer, entities []catalog.LoadedEntity, relations []entityRelation) error {
	if len(entities) == 0 {
		if _, err := fmt.Fprintln(stdout, "No entities found."); err != nil {
			return fmt.Errorf("write entity graph output: %w", err)
		}
		return nil
	}
	for _, entity := range entities {
		if _, err := fmt.Fprintf(stdout, "%s/%s (%s)\n", entity.AppName, entity.Entity.Name, entityKind(entity)); err != nil {
			return fmt.Errorf("write entity graph output: %w", err)
		}
		wroteRelation := false
		for _, relation := range relations {
			if relation.From.Key() != entity.Key() {
				continue
			}
			wroteRelation = true
			if _, err := fmt.Fprintf(stdout, "  -> %s %s -> %s/%s\n", relation.Kind, relation.FieldName, relation.TargetApp, relation.Target); err != nil {
				return fmt.Errorf("write entity graph output: %w", err)
			}
		}
		for _, relation := range relations {
			if relation.TargetApp != entity.AppName || relation.Target != entity.Entity.Name {
				continue
			}
			wroteRelation = true
			if _, err := fmt.Fprintf(stdout, "  <- %s %s/%s.%s\n", relation.Kind, relation.From.AppName, relation.From.Entity.Name, relation.FieldName); err != nil {
				return fmt.Errorf("write entity graph output: %w", err)
			}
		}
		if !wroteRelation {
			if _, err := fmt.Fprintln(stdout, "  (no relationships)"); err != nil {
				return fmt.Errorf("write entity graph output: %w", err)
			}
		}
	}
	return nil
}

func graphScope(entities []catalog.LoadedEntity, args []string) ([]catalog.LoadedEntity, error) {
	if len(args) == 0 {
		return entities, nil
	}
	scope := strings.TrimSpace(args[0])
	if strings.Contains(scope, "/") {
		ref, err := shape.ParseAppRef(scope)
		if err != nil {
			return nil, err
		}
		entity, ok := findEntity(entities, ref)
		if !ok {
			return nil, fmt.Errorf("entity %q was not found", scope)
		}
		return []catalog.LoadedEntity{entity}, nil
	}
	var scoped []catalog.LoadedEntity
	for _, entity := range entities {
		if entity.AppName == scope {
			scoped = append(scoped, entity)
		}
	}
	if len(scoped) == 0 {
		return nil, fmt.Errorf("app %q has no discovered entities", scope)
	}
	return scoped, nil
}

func findEntity(entities []catalog.LoadedEntity, ref shape.AppRef) (catalog.LoadedEntity, bool) {
	for _, entity := range entities {
		if entity.AppName == ref.App && entity.Entity.Name == ref.Name {
			return entity, true
		}
	}
	return catalog.LoadedEntity{}, false
}

func entityRelations(entities []catalog.LoadedEntity) []entityRelation {
	var relations []entityRelation
	for _, entity := range entities {
		for _, field := range entity.Entity.Fields {
			if field.Type != "link" && field.Type != "collection" {
				continue
			}
			if field.Options.Entity == "" {
				continue
			}
			targetApp := field.Options.App
			if targetApp == "" {
				targetApp = entity.AppName
			}
			relations = append(relations, entityRelation{
				From:      entity,
				FieldName: field.Name,
				Kind:      field.Type,
				TargetApp: targetApp,
				Target:    field.Options.Entity,
			})
		}
	}
	return relations
}

func entityKind(entity catalog.LoadedEntity) string {
	switch {
	case entity.IsCollection():
		return "collection"
	case entity.Entity.IsSingle:
		return "single"
	default:
		return "normal"
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
