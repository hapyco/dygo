package db

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultPingTimeout = 5 * time.Second

// Pool is the minimal PostgreSQL pool behavior dygo needs for connectivity checks.
type Pool interface {
	Ping(context.Context) error
	Close()
}

// Connector opens a PostgreSQL pool for one database URL.
type Connector func(context.Context, string) (Pool, error)

// Check connects to PostgreSQL and verifies the connection can be pinged.
func Check(ctx context.Context, databaseURL string) error {
	return CheckWithConnector(ctx, databaseURL, Connect)
}

// CheckWithConnector verifies database connectivity through an injected connector.
func CheckWithConnector(ctx context.Context, databaseURL string, connect Connector) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	databaseURL = strings.TrimSpace(databaseURL)
	if databaseURL == "" {
		return fmt.Errorf("database url is required")
	}
	if connect == nil {
		return fmt.Errorf("database connector is required")
	}

	checkCtx, cancel := context.WithTimeout(ctx, defaultPingTimeout)
	defer cancel()

	pool, err := connect(checkCtx, databaseURL)
	if err != nil {
		return sanitizeError("connect to postgres", databaseURL, err)
	}
	if pool == nil {
		return fmt.Errorf("connect to postgres: database pool is nil")
	}
	defer pool.Close()

	if err := pool.Ping(checkCtx); err != nil {
		return sanitizeError("ping postgres", databaseURL, err)
	}
	return nil
}

// Connect opens a pgx PostgreSQL connection pool.
func Connect(ctx context.Context, databaseURL string) (Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid postgres database url")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return pool, nil
}

func sanitizeError(message string, databaseURL string, err error) error {
	if err == nil {
		return errors.New(message)
	}
	detail := err.Error()
	for _, target := range redactionTargets(databaseURL) {
		detail = strings.ReplaceAll(detail, target, "<redacted>")
	}
	if strings.TrimSpace(detail) == "" {
		return errors.New(message)
	}
	return fmt.Errorf("%s: %s", message, detail)
}

func redactionTargets(databaseURL string) []string {
	var targets []string
	if databaseURL != "" {
		targets = append(targets, databaseURL)
	}
	parsed, err := url.Parse(databaseURL)
	if err == nil && parsed.User != nil {
		if password, ok := parsed.User.Password(); ok && password != "" {
			targets = append(targets, password)
		}
	}
	return targets
}
