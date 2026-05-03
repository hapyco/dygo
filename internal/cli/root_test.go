package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantStdout string
		wantStderr string
		wantErr    bool
	}{
		{
			name:       "prints help without args",
			args:       nil,
			wantStdout: "Usage:",
		},
		{
			name:       "prints version",
			args:       []string{"version"},
			wantStdout: "dygo dev",
		},
		{
			name:       "prints default serve address",
			args:       []string{"serve"},
			wantStdout: "127.0.0.1:6790",
		},
		{
			name:    "rejects unknown command",
			args:    []string{"missing"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer

			err := Run(context.Background(), tt.args, &stdout, &stderr)
			if tt.wantErr && err == nil {
				t.Fatal("Run() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Run() error = %v, want nil", err)
			}
			if tt.wantStdout != "" && !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Fatalf("stdout = %q, want substring %q", stdout.String(), tt.wantStdout)
			}
			if tt.wantStderr != "" && !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr = %q, want substring %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}
