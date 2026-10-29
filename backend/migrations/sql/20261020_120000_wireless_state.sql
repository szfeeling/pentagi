-- +goose Up
CREATE TABLE wireless_state (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    state JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX wireless_state_nodes_idx ON wireless_state USING gin ((state->'nodes'));
INSERT INTO privileges (role_id, name) VALUES
    (1, 'wireless.admin'), (2, 'wireless.view'), (2, 'wireless.edit');

-- +goose Down
DELETE FROM privileges WHERE name IN ('wireless.admin', 'wireless.view', 'wireless.edit');
DROP TABLE wireless_state;
