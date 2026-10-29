package wireless

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

type Store interface {
	Load(context.Context, int64) (Snapshot, error)
	Update(context.Context, int64, func(*Snapshot) error) error
	Owner(context.Context, string, string) (int64, error)
	ActiveFlows(context.Context, int64) (map[int64]bool, error)
}

func (s SQLStore) ActiveFlows(ctx context.Context, uid int64) (map[int64]bool, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM flows WHERE user_id = $1 AND status IN ('created', 'running', 'waiting')`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	active := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		active[id] = true
	}
	return active, rows.Err()
}

type SQLStore struct{ DB *sql.DB }

func (s SQLStore) Load(ctx context.Context, uid int64) (Snapshot, error) {
	var raw []byte
	err := s.DB.QueryRowContext(ctx, `SELECT state FROM wireless_state WHERE user_id = $1`, uid).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return Snapshot{}, nil
	}
	if err != nil {
		return Snapshot{}, err
	}
	var state Snapshot
	err = json.Unmarshal(raw, &state)
	return state, err
}

func (s SQLStore) Update(ctx context.Context, uid int64, fn func(*Snapshot) error) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO wireless_state (user_id, state) VALUES ($1, '{}') ON CONFLICT DO NOTHING`, uid); err != nil {
		return err
	}
	var raw []byte
	if err = tx.QueryRowContext(ctx, `SELECT state FROM wireless_state WHERE user_id = $1 FOR UPDATE`, uid).Scan(&raw); err != nil {
		return err
	}
	var state Snapshot
	if err = json.Unmarshal(raw, &state); err != nil {
		return err
	}
	if err = fn(&state); err != nil {
		return err
	}
	state.Revision++
	if raw, err = json.Marshal(state); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE wireless_state SET state = $2, updated_at = NOW() WHERE user_id = $1`, uid, raw); err != nil {
		return err
	}
	return tx.Commit()
}

func (s SQLStore) Owner(ctx context.Context, id, hash string) (int64, error) {
	filter, err := json.Marshal([]map[string]string{{"id": id, "tokenHash": hash}})
	if err != nil {
		return 0, err
	}
	var uid int64
	err = s.DB.QueryRowContext(ctx, `SELECT user_id FROM wireless_state WHERE state->'nodes' @> $1::jsonb`, filter).Scan(&uid)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrUnauthorized
	}
	return uid, err
}
