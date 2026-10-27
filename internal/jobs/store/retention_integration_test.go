package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The test creates and drops its own database; it never touches application tables.
func TestPostgresPruneTerminalPreservesIdempotency(t *testing.T) {
	url := os.Getenv("DYGO_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set DYGO_TEST_DATABASE_URL to run PostgreSQL regressions")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("dygo_retention_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE DATABASE "`+name+`"`); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	config := admin.Config().Copy()
	config.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if _, err := admin.Exec(ctx, `DROP DATABASE "`+name+`"`); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	_, err = pool.Exec(ctx, `CREATE TABLE job_execution (
		id bigint PRIMARY KEY, job_id bigint NOT NULL, queue text NOT NULL,
		status text NOT NULL, finished_at timestamptz, idempotency_key text,
		UNIQUE (job_id, idempotency_key)
	)`)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	old := now.Add(-31 * 24 * time.Hour)
	newer := now.Add(-29 * 24 * time.Hour)
	_, err = pool.Exec(ctx, `INSERT INTO job_execution VALUES
		(1, 1, 'default', 'succeeded', $1, NULL),
		(2, 1, 'default', 'failed', $1, NULL),
		(3, 1, 'default', 'cancelled', $1, NULL),
		(4, 1, 'default', 'succeeded', $1, 'stable-key'),
		(5, 1, 'default', 'queued', $1, NULL),
		(6, 1, 'default', 'running', $1, NULL),
		(7, 1, 'default', 'succeeded', $2, NULL),
		(8, 1, 'other', 'failed', $1, NULL),
		(9, 1, 'default', 'failed', NULL, NULL),
		(11, 1, 'default', 'failed', $1, '')`, old, newer)
	if err != nil {
		t.Fatal(err)
	}
	store, err := New(pool)
	if err != nil {
		t.Fatal(err)
	}
	cutoff := now.Add(-30 * 24 * time.Hour)
	count, err := store.PruneTerminal(ctx, []string{"default"}, cutoff, 2)
	if err != nil || count != 2 {
		t.Fatalf("first prune: count=%d err=%v", count, err)
	}
	count, err = store.PruneTerminal(ctx, []string{"default"}, cutoff, 2)
	if err != nil || count != 1 {
		t.Fatalf("second prune: count=%d err=%v", count, err)
	}
	var ids []int64
	rows, err := pool.Query(ctx, `SELECT id FROM job_execution ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if fmt.Sprint(ids) != "[4 5 6 7 8 9 11]" {
		t.Fatalf("retained ids = %v", ids)
	}
	var duplicateID int64
	err = pool.QueryRow(ctx, `INSERT INTO job_execution VALUES (10, 1, 'default', 'queued', NULL, 'stable-key') ON CONFLICT (job_id, idempotency_key) DO NOTHING RETURNING id`).Scan(&duplicateID)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("duplicate insertion error = %v, want no inserted row", err)
	}
	var existingID int64
	if err := pool.QueryRow(ctx, `SELECT id FROM job_execution WHERE job_id=1 AND idempotency_key='stable-key'`).Scan(&existingID); err != nil || existingID != 4 {
		t.Fatalf("existing execution = %d, err = %v", existingID, err)
	}
}
