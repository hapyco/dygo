package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ListenerPool provides a dedicated PostgreSQL connection for LISTEN state.
type ListenerPool interface {
	Acquire(context.Context) (*pgxpool.Conn, error)
}

// Listener receives Job Execution queue notifications.
type Listener struct {
	conn *pgxpool.Conn
}

// NewListener starts listening for Job Execution queue notifications.
func NewListener(ctx context.Context, pool ListenerPool) (*Listener, error) {
	if pool == nil {
		return nil, fmt.Errorf("job notification pool is required")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire job notification connection: %w", err)
	}
	listener := &Listener{conn: conn}
	if _, err := conn.Exec(ctx, `LISTEN `+notificationChannel); err != nil {
		listener.Close()
		return nil, fmt.Errorf("listen for job notifications: %w", err)
	}
	return listener, nil
}

// Wait blocks until a Job Execution queue notification arrives.
func (l *Listener) Wait(ctx context.Context) (string, error) {
	if l == nil || l.conn == nil {
		return "", fmt.Errorf("job notification listener is closed")
	}
	notification, err := l.conn.Conn().WaitForNotification(ctx)
	if err != nil {
		return "", err
	}
	return notification.Payload, nil
}

// Close releases the dedicated notification connection.
func (l *Listener) Close() {
	if l == nil || l.conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, _ = l.conn.Exec(ctx, `UNLISTEN `+notificationChannel)
	l.conn.Release()
	l.conn = nil
}
