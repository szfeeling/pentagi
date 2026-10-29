//go:build postgres

package wireless

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

func TestStore_Update_PersistsAcrossServicesAndSerializesConcurrentWriters(t *testing.T) {
	dsn := os.Getenv("PENTAGI_TEST_DSN")
	if dsn == "" {
		t.Fatal("PENTAGI_TEST_DSN is required")
	}
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := "wireless_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := admin.ExecContext(t.Context(), `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.ExecContext(context.Background(), `DROP SCHEMA `+schema+` CASCADE`)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	params := parsed.Query()
	params.Set("search_path", schema)
	parsed.RawQuery = params.Encode()
	db, err := sql.Open("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(t.Context(), `CREATE TABLE users (id BIGINT PRIMARY KEY); INSERT INTO users VALUES (1), (2); CREATE TABLE privileges (role_id BIGINT, name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), `CREATE TABLE flows (id BIGINT PRIMARY KEY, user_id BIGINT REFERENCES users(id), status TEXT);
		INSERT INTO flows VALUES (1,1,'created'),(2,1,'running'),(3,1,'waiting'),(4,1,'finished'),(5,1,'failed'),(6,2,'running')`); err != nil {
		t.Fatal(err)
	}
	active, err := (SQLStore{DB: db}).ActiveFlows(t.Context(), 1)
	if err != nil || len(active) != 3 || !active[1] || !active[2] || !active[3] {
		t.Fatalf("active user flows: %+v %v", active, err)
	}
	raw, err := os.ReadFile("../../migrations/sql/20261020_120000_wireless_state.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.Split(string(raw), "-- +goose Down")[0]
	if _, err := db.ExecContext(t.Context(), up); err != nil {
		t.Fatal(err)
	}
	first := New(SQLStore{DB: db})
	area, err := first.Command(t.Context(), 1, Command{Kind: "area.create", Name: "Persistent lab"})
	if err != nil {
		t.Fatal(err)
	}
	node, err := first.Command(t.Context(), 1, Command{Kind: "node.create", Name: "Persistent sensor", AreaID: area.ID})
	if err != nil {
		t.Fatal(err)
	}
	restarted := New(SQLStore{DB: db})
	state, err := restarted.Snapshot(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 2 || len(state.Areas) != 1 || state.Areas[0].Name != "Persistent lab" || state.Nodes[0].TokenHash != "" {
		t.Fatalf("state not restored or credential leaked: %+v", state)
	}
	uid, err := (SQLStore{DB: db}).Owner(t.Context(), node.ID, tokenHash(node.Token))
	if err != nil || uid != 1 {
		t.Fatalf("durable credential lookup: %d %v", uid, err)
	}
	var wg sync.WaitGroup
	failures := make(chan error, 20)
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			failures <- (SQLStore{DB: db}).Update(t.Context(), 1, func(state *Snapshot) error { state.Events = append(state.Events, Event{ID: fmt.Sprint(i)}); return nil })
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	state, err = restarted.Snapshot(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 22 || len(state.Events) != 22 {
		t.Fatalf("lost concurrent updates: revision %d, events %d", state.Revision, len(state.Events))
	}
	err = (SQLStore{DB: db}).Update(t.Context(), 1, func(state *Snapshot) error { state.Areas = nil; return ErrConflict })
	if err != ErrConflict {
		t.Fatalf("rollback error: %v", err)
	}
	state, err = restarted.Snapshot(t.Context(), 1)
	if err != nil || len(state.Areas) != 1 || state.Revision != 22 {
		t.Fatalf("failed transaction was committed: %+v %v", state, err)
	}
	other, err := restarted.Snapshot(t.Context(), 2)
	if err != nil || other.Revision != 0 || len(other.Nodes) != 0 {
		t.Fatalf("cross-user state: %+v %v", other, err)
	}
	if _, err := db.ExecContext(t.Context(), strings.Split(string(raw), "-- +goose Down")[1]); err != nil {
		t.Fatal(err)
	}
}
