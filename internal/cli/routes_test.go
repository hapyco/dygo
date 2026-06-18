package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouteCommands(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	writeCLIApp(t, filepath.Join(root, "apps", "sales"), "sales")
	writeCLIEntity(t, cliEntityPath(root, "sales", "lead"), `
label: Lead
fields:
  - name: title
    label: Title
    type: text
`)
	t.Chdir(root)

	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "list",
			args: []string{"route", "list"},
			want: []string{"/lead normal sales/lead apps/sales/entities/lead/lead.entity.yml", "reserved: /api"},
		},
		{
			name: "validate",
			args: []string{"route", "validate"},
			want: []string{"routes are valid: 8 reserved routes, 1 entity routes, 0 conflicts"},
		},
		{
			name: "reserved",
			args: []string{"route", "reserved"},
			want: []string{"/api", "/login"},
		},
		{
			name: "entity route resolve",
			args: []string{"route", "resolve", "/lead"},
			want: []string{"path: /lead", "handler: entity route", "entity: sales/lead", "route: /lead"},
		},
		{
			name: "record API resolve",
			args: []string{"route", "resolve", "GET", "/api/v1/records/lead/1"},
			want: []string{"method: GET", "handler: api record", "entity: sales/lead", "action: read", "permission: read"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			err := Run(context.Background(), tt.args, strings.NewReader(""), &stdout, &stderr)
			if err != nil {
				t.Fatalf("Run(%v) error = %v, want nil", tt.args, err)
			}
			for _, want := range tt.want {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
				}
			}
		})
	}
}

func TestRouteResolveRequiresPath(t *testing.T) {
	root := t.TempDir()
	writeCLIProjectRoot(t, root)
	t.Chdir(root)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := Run(context.Background(), []string{"route", "resolve", "lead"}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Run(route resolve lead) error = nil, want path format error")
	}
	if !strings.Contains(err.Error(), `path "lead" must start with /`) {
		t.Fatalf("Run(route resolve lead) error = %q, want path format error", err.Error())
	}
}
