package db

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakePool struct {
	pingErr error
	closed  bool
}

func (p *fakePool) Ping(context.Context) error {
	return p.pingErr
}

func (p *fakePool) Close() {
	p.closed = true
}

func TestCheckWithConnector(t *testing.T) {
	t.Parallel()

	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"
	pool := &fakePool{}

	err := CheckWithConnector(context.Background(), databaseURL, func(_ context.Context, got string) (Pool, error) {
		if got != databaseURL {
			t.Fatalf("connector database URL = %q, want %q", got, databaseURL)
		}
		return pool, nil
	})
	if err != nil {
		t.Fatalf("CheckWithConnector() error = %v, want nil", err)
	}
	if !pool.closed {
		t.Fatal("pool was not closed")
	}
}

func TestCheckWithConnectorRequiresURL(t *testing.T) {
	t.Parallel()

	err := CheckWithConnector(context.Background(), "", func(context.Context, string) (Pool, error) {
		t.Fatal("connector should not be called")
		return nil, nil
	})
	if err == nil {
		t.Fatal("CheckWithConnector(empty URL) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "database url is required") {
		t.Fatalf("CheckWithConnector(empty URL) error = %q, want required URL", err.Error())
	}
}

func TestCheckWithConnectorRedactsConnectorErrors(t *testing.T) {
	t.Parallel()

	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"

	err := CheckWithConnector(context.Background(), databaseURL, func(context.Context, string) (Pool, error) {
		return nil, errors.New("cannot connect to postgres://user:secret-password@localhost:5432/dygo")
	})
	if err == nil {
		t.Fatal("CheckWithConnector(connector error) error = nil, want error")
	}
	for _, leaked := range []string{databaseURL, "secret-password"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("CheckWithConnector() error = %q, leaked %q", err.Error(), leaked)
		}
	}
}

func TestCheckWithConnectorRedactsPingErrors(t *testing.T) {
	t.Parallel()

	const databaseURL = "postgres://user:secret-password@localhost:5432/dygo"

	err := CheckWithConnector(context.Background(), databaseURL, func(context.Context, string) (Pool, error) {
		return &fakePool{pingErr: errors.New("ping failed with secret-password")}, nil
	})
	if err == nil {
		t.Fatal("CheckWithConnector(ping error) error = nil, want error")
	}
	if strings.Contains(err.Error(), "secret-password") {
		t.Fatalf("CheckWithConnector() error = %q, leaked password", err.Error())
	}
}

func TestConnectRejectsInvalidURLWithoutLeakingValue(t *testing.T) {
	t.Parallel()

	const databaseURL = "postgres://user:secret-password@%zz"

	_, err := Connect(context.Background(), databaseURL)
	if err == nil {
		t.Fatal("Connect(invalid URL) error = nil, want error")
	}
	for _, leaked := range []string{databaseURL, "secret-password"} {
		if strings.Contains(err.Error(), leaked) {
			t.Fatalf("Connect() error = %q, leaked %q", err.Error(), leaked)
		}
	}
}
