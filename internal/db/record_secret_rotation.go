package db

import (
	"context"
	"fmt"

	"github.com/hapyco/dygo/internal/recordsecret"
	"github.com/jackc/pgx/v5"
)

// RecordKeyLock serializes offline key commands. It is deliberately distinct
// from schema and worker locks. Operators must stop all runtime processes.
const RecordKeyLock int64 = 0x6479676f736563

// RotateRecordSecrets rewrites storage only: no business Hooks, timestamps, or
// Activity changes. Rotation changes encryption, not the business value.
// Each batch commits independently and old keys remain available after failure.
func RotateRecordSecrets(ctx context.Context, conn *pgx.Conn, ring recordsecret.Ring) (int, error) {
	reader := NewMetadataReader(conn)
	entities, err := reader.ListEntities(ctx)
	if err != nil {
		return 0, fmt.Errorf("load metadata for Record key rotation failed: %w", err)
	}
	total := 0
	for _, entity := range entities {
		meta, err := reader.GetEntityMetaByIdentity(ctx, entity.App.Name, entity.Key)
		if err != nil {
			return total, fmt.Errorf("load Record secret metadata failed: %w", err)
		}
		layout, err := newRecordLayout(meta)
		if err != nil {
			return total, err
		}
		for _, field := range layout.Fields {
			if field.Type != "secret" {
				continue
			}
			var after int64
			for {
				tx, err := conn.Begin(ctx)
				if err != nil {
					return total, fmt.Errorf("begin Record key rotation batch failed: %w", err)
				}
				count, last, err := rotateSecretBatch(ctx, tx, layout, field, ring, after)
				if err != nil {
					_ = tx.Rollback(ctx)
					return total, err
				}
				if err = tx.Commit(ctx); err != nil {
					return total, fmt.Errorf("commit Record key rotation batch failed: %w", err)
				}
				total += count
				if last == after {
					break
				}
				after = last
			}
		}
	}
	return total, nil
}
func rotateSecretBatch(ctx context.Context, tx pgx.Tx, layout recordLayout, field recordField, ring recordsecret.Ring, after int64) (int, int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf("SELECT id, %s FROM %s WHERE id > $1 AND %s IS NOT NULL ORDER BY id LIMIT 100 FOR UPDATE", quoteIdent(field.Column), quoteIdent(layout.Table), quoteIdent(field.Column)), after)
	if err != nil {
		return 0, after, fmt.Errorf("read Record key rotation batch failed: %w", err)
	}
	type item struct {
		id         int64
		ciphertext string
	}
	items := []item{}
	for rows.Next() {
		var row item
		if err := rows.Scan(&row.id, &row.ciphertext); err != nil {
			rows.Close()
			return 0, after, fmt.Errorf("read Record secret failed: %w", err)
		}
		items = append(items, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, after, fmt.Errorf("read Record key rotation batch failed: %w", err)
	}
	count := 0
	for _, row := range items {
		value, err := ring.Decrypt(secretBinding(layout, field), row.ciphertext)
		if err != nil {
			return count, after, err
		}
		id, err := recordsecret.CipherKey(row.ciphertext)
		if err != nil {
			return count, after, err
		}
		if id != ring.Active {
			ciphertext, err := ring.Encrypt(secretBinding(layout, field), value)
			if err != nil {
				return count, after, err
			}
			if _, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s SET %s=$1 WHERE id=$2", quoteIdent(layout.Table), quoteIdent(field.Column)), ciphertext, row.id); err != nil {
				return count, after, fmt.Errorf("write rotated Record secret failed: %w", err)
			}
			count++
		}
		after = row.id
	}
	return count, after, nil
}
