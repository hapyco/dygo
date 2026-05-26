package cli

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/hapyco/dygo/internal/permissions"
	"github.com/hapyco/dygo/internal/project"
	routeplan "github.com/hapyco/dygo/internal/routes"
	"github.com/spf13/cobra"
)

func newRouteCommand(stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route",
		Short: "Inspect and validate dygo routes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newRouteListCommand(stdout))
	cmd.AddCommand(newRouteValidateCommand(stdout))
	cmd.AddCommand(newRouteResolveCommand(stdout))
	cmd.AddCommand(newRouteReservedCommand(stdout))

	return cmd
}

func newRouteListCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List routeable Entities and reserved root slugs",
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
			for _, route := range routeplan.Entries(metadata.Entities) {
				if _, err := fmt.Fprintf(stdout, "/%s %s %s/%s %s\n", route.Slug, route.Kind, route.AppName, route.EntityName, relToWorkingRoot(route.Path)); err != nil {
					return fmt.Errorf("write route list output: %w", err)
				}
			}
			if _, err := fmt.Fprintln(stdout, "reserved: "+strings.Join(routeplan.PrefixedReservedSlugs(), ", ")); err != nil {
				return fmt.Errorf("write route list output: %w", err)
			}
			return nil
		},
	}
}

func newRouteValidateCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate route conflicts and reserved-route usage",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			root, err := workingRootPath()
			if err != nil {
				return err
			}
			metadata, err := project.LoadMetadata(root)
			if err != nil {
				return fmt.Errorf("validate routes: %w", err)
			}
			if _, err := fmt.Fprintf(stdout, "routes are valid: %d routeable entities, %d reserved slugs\n", len(routeplan.Entries(metadata.Entities)), len(routeplan.ReservedSlugs())); err != nil {
				return fmt.Errorf("write route validate output: %w", err)
			}
			return nil
		},
	}
}

func newRouteResolveCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "resolve [<method>] <path>",
		Short: "Explain which route handles a path",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 && len(args) != 2 {
				return fmt.Errorf("accepts 1 or 2 arg(s), received %d", len(args))
			}
			return nil
		},
		RunE: func(_ *cobra.Command, args []string) error {
			method := ""
			rawPath := args[0]
			if len(args) == 2 {
				method = strings.ToUpper(strings.TrimSpace(args[0]))
				rawPath = args[1]
				if method == "" {
					return fmt.Errorf("method is required")
				}
			}
			normalized, err := normalizeResolvePath(rawPath)
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
			return writeRouteResolution(stdout, method, normalized, routeplan.Entries(metadata.Entities))
		},
	}
}

func newRouteReservedCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "reserved",
		Short: "List framework-reserved route slugs",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			for _, slug := range routeplan.PrefixedReservedSlugs() {
				if _, err := fmt.Fprintln(stdout, slug); err != nil {
					return fmt.Errorf("write reserved route output: %w", err)
				}
			}
			return nil
		},
	}
}

func normalizeResolvePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(value, "/") {
		return "", fmt.Errorf("path %q must start with /", value)
	}
	clean := path.Clean(value)
	if clean == "." {
		return "/", nil
	}
	return clean, nil
}

func writeRouteResolution(stdout io.Writer, method string, requestPath string, routes []routeplan.Entry) error {
	if _, err := fmt.Fprintf(stdout, "path: %s\n", requestPath); err != nil {
		return fmt.Errorf("write route resolve output: %w", err)
	}
	if method != "" {
		if _, err := fmt.Fprintf(stdout, "method: %s\n", method); err != nil {
			return fmt.Errorf("write route resolve output: %w", err)
		}
	}
	parts := strings.Split(strings.Trim(requestPath, "/"), "/")
	if requestPath == "/health" {
		return writeStaticRoute(stdout, "health", "framework health check")
	}
	if len(parts) >= 1 && parts[0] == "api" {
		return writeAPIResolution(stdout, method, parts, routes)
	}
	if len(parts) == 1 {
		if route, ok := findRouteBySlug(routes, parts[0]); ok {
			return writeEntityRoute(stdout, "entity route", route, "")
		}
	}
	if len(parts) >= 1 && routeplan.IsReservedSlug(parts[0]) {
		return writeStaticRoute(stdout, "studio", "framework-reserved Studio route")
	}
	return writeStaticRoute(stdout, "studio", "Studio fallback")
}

func writeAPIResolution(stdout io.Writer, method string, parts []string, routes []routeplan.Entry) error {
	if len(parts) >= 4 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "records" {
		route, ok := findRouteBySlug(routes, parts[3])
		if !ok {
			return writeStaticRoute(stdout, "api record", "unknown Entity route slug")
		}
		action := recordAction(method, parts[4:])
		return writeEntityRoute(stdout, "api record", route, action)
	}
	if len(parts) >= 5 && parts[0] == "api" && parts[1] == "v1" && parts[2] == "entities" && parts[4] == "meta" {
		route, ok := findRouteBySlug(routes, parts[3])
		if !ok {
			return writeStaticRoute(stdout, "api metadata", "unknown Entity route slug")
		}
		return writeEntityRoute(stdout, "api metadata", route, string(permissions.ActionRead))
	}
	if len(parts) >= 3 && parts[0] == "api" && parts[1] == "v1" {
		switch parts[2] {
		case "auth":
			return writeStaticRoute(stdout, "api auth", "authentication endpoint")
		case "apps", "entities", "platform":
			return writeStaticRoute(stdout, "api metadata", "framework metadata endpoint")
		}
	}
	return writeStaticRoute(stdout, "api", "framework API route")
}

func writeEntityRoute(stdout io.Writer, handler string, route routeplan.Entry, action string) error {
	if _, err := fmt.Fprintf(stdout, "handler: %s\n", handler); err != nil {
		return fmt.Errorf("write route resolve output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "entity: %s/%s\n", route.AppName, route.EntityName); err != nil {
		return fmt.Errorf("write route resolve output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "route: /%s\n", route.Slug); err != nil {
		return fmt.Errorf("write route resolve output: %w", err)
	}
	if action != "" {
		if _, err := fmt.Fprintf(stdout, "action: %s\npermission: %s\n", action, action); err != nil {
			return fmt.Errorf("write route resolve output: %w", err)
		}
	}
	return nil
}

func writeStaticRoute(stdout io.Writer, handler string, detail string) error {
	if _, err := fmt.Fprintf(stdout, "handler: %s\n", handler); err != nil {
		return fmt.Errorf("write route resolve output: %w", err)
	}
	if _, err := fmt.Fprintf(stdout, "detail: %s\n", detail); err != nil {
		return fmt.Errorf("write route resolve output: %w", err)
	}
	return nil
}

func findRouteBySlug(routes []routeplan.Entry, slug string) (routeplan.Entry, bool) {
	for _, route := range routes {
		if route.Slug == slug {
			return route, true
		}
	}
	return routeplan.Entry{}, false
}

func recordAction(method string, _ []string) string {
	switch method {
	case http.MethodGet:
		return string(permissions.ActionRead)
	case http.MethodPost:
		return string(permissions.ActionCreate)
	case http.MethodPatch:
		return string(permissions.ActionUpdate)
	case http.MethodDelete:
		return string(permissions.ActionDelete)
	case "":
		return ""
	default:
		return "unknown"
	}
}
