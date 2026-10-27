package store

import (
	"context"
	"fmt"
	"time"
)

// PruneTerminal removes a bounded batch of old terminal executions without
// idempotency keys. Keyed rows must remain to preserve duplicate protection.
func (s Store) PruneTerminal(ctx context.Context, queueNames []string, cutoff time.Time, limit int) (int, error) {
	if len(queueNames) == 0 || cutoff.IsZero() || limit < 1 {
		return 0, fmt.Errorf("prune terminal executions requires queues, a cutoff, and a positive limit")
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin job execution pruning: %w", err)
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `
WITH candidates AS (
    SELECT id FROM "job_execution"
    WHERE queue = ANY($1) AND status IN ('succeeded', 'failed', 'cancelled')
      AND finished_at < $2 AND idempotency_key IS NULL
    ORDER BY finished_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT $3
)
DELETE FROM "job_execution" WHERE id IN (SELECT id FROM candidates)`, queueNames, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("prune job executions: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit job execution pruning: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
