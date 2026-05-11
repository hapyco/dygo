package runtime

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dygo-dev/dygo/pkg/sdk"
)

func TestRunAppliesRecordHookRegistrars(t *testing.T) {
	t.Parallel()

	called := false
	var stdout bytes.Buffer
	err := Run(context.Background(), []string{"version"}, strings.NewReader(""), &stdout, &bytes.Buffer{}, Options{
		RecordHooks: []sdk.RecordHookRegistrar{
			func(sdk.RecordHookRegistry) error {
				called = true
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("Run(version) error = %v, want nil", err)
	}
	if !called {
		t.Fatal("record hook registrar was not called")
	}
	if stdout.String() != "dygo dev\n" {
		t.Fatalf("version stdout = %q, want dygo dev", stdout.String())
	}
}

func TestRunReturnsRecordHookRegistrarErrors(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), []string{"version"}, strings.NewReader(""), &bytes.Buffer{}, &bytes.Buffer{}, Options{
		RecordHooks: []sdk.RecordHookRegistrar{
			func(sdk.RecordHookRegistry) error {
				return errors.New("boom")
			},
		},
	})
	if err == nil {
		t.Fatal("Run(version) error = nil, want registrar error")
	}
	if !strings.Contains(err.Error(), "configure record hooks") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Run(version) error = %q, want hook configuration context", err.Error())
	}
}
