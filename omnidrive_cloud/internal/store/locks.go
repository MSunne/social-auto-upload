package store

import (
	"context"
	"fmt"
)

func advisoryLockKey(scope string, parts ...string) string {
	key := scope
	for _, part := range parts {
		key += ":" + part
	}
	return key
}

func (s *Store) WithAdvisoryLock(ctx context.Context, key string, fn func() error) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("store pool is not initialized")
	}

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, key); err != nil {
		return err
	}
	defer func() {
		_, _ = conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, key)
	}()

	return fn()
}
