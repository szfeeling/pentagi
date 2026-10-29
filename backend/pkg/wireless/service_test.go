package wireless

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type memoryStore struct {
	mu       sync.Mutex
	states   map[int64][]byte
	inactive map[int64]bool
}

func (m *memoryStore) ActiveFlows(ctx context.Context, uid int64) (map[int64]bool, error) {
	state, err := m.Load(ctx, uid)
	if err != nil {
		return nil, err
	}
	active := make(map[int64]bool)
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, flow := range state.Flows {
		active[flow.FlowID] = !m.inactive[flow.FlowID]
	}
	return active, nil
}

func (m *memoryStore) Load(ctx context.Context, uid int64) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var state Snapshot
	if raw := m.states[uid]; raw != nil {
		if err := json.Unmarshal(raw, &state); err != nil {
			return Snapshot{}, err
		}
	}
	return state, nil
}

func (m *memoryStore) Update(ctx context.Context, uid int64, fn func(*Snapshot) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var state Snapshot
	if raw := m.states[uid]; raw != nil {
		if err := json.Unmarshal(raw, &state); err != nil {
			return err
		}
	}
	if err := fn(&state); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	state.Revision++
	raw, err := json.Marshal(state)
	if err == nil {
		m.states[uid] = raw
	}
	return err
}

func (m *memoryStore) Owner(ctx context.Context, id, hash string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for uid, raw := range m.states {
		var state Snapshot
		if err := json.Unmarshal(raw, &state); err != nil {
			return 0, err
		}
		for _, node := range state.Nodes {
			if node.ID == id && node.TokenHash == hash {
				return uid, nil
			}
		}
	}
	return 0, ErrUnauthorized
}

type scenario struct {
	s      *Service
	now    time.Time
	token  string
	nodeID string
	areaID string
	job    Job
	report Report
}

func setupScenario(t *testing.T) *scenario {
	t.Helper()
	x := &scenario{now: time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)}
	x.s = New(&memoryStore{states: make(map[int64][]byte)})
	x.s.now = func() time.Time { return x.now }
	area, err := x.s.Command(t.Context(), 1, Command{Kind: "area.create", Name: "Lab", BSSIDs: []string{"02:00:00:00:00:01"}, CIDRs: []string{"192.168.50.0/24"}})
	if err != nil {
		t.Fatal(err)
	}
	x.areaID = area.ID
	node, err := x.s.Command(t.Context(), 1, Command{Kind: "node.create", Name: "Sensor", AreaID: area.ID})
	if err != nil {
		t.Fatal(err)
	}
	x.nodeID, x.token = node.ID, node.Token
	x.report = Report{NodeID: node.ID, BootID: "boot-1", Sequence: 1, Capabilities: []string{"scan", "connect", "disconnect"}, Observations: []Observation{{Kind: "ap", MAC: "02:00:00:00:00:01", SSID: "Lab", Frequency: 2412, Signal: -45, ObservedAt: x.now}}}
	if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
		t.Fatal(err)
	}
	x.report.Observations = nil
	return x
}

func (x *scenario) connected(t *testing.T) Binding {
	t.Helper()
	job, err := x.s.Command(t.Context(), 1, Command{Kind: "job.start", NodeID: x.nodeID, JobKind: "connect", BSSID: "02:00:00:00:00:01", RequestID: "connect-1"})
	if err != nil {
		t.Fatal(err)
	}
	x.report.Sequence++
	dispatch, err := x.s.Report(t.Context(), x.token, x.report)
	if err != nil || len(dispatch.Jobs) != 1 {
		t.Fatalf("dispatch: %+v, %v", dispatch, err)
	}
	x.job = dispatch.Jobs[0]
	if x.job.ID != job.ID || x.job.Status != "running" || x.job.LeaseID == "" {
		t.Fatalf("invalid lease: %+v", x.job)
	}
	x.report.Sequence++
	x.report.Contexts = []ContextReport{{ID: job.ID, JobID: job.ID, LeaseID: x.job.LeaseID, BSSID: "02:00:00:00:00:01", DaemonID: "linux-daemon", Network: "wireless-lab", NetworkID: "network-1", Interface: "wlan1", Address: "192.168.50.10/24", Connected: true, Endpoints: []Endpoint{{MAC: "02:00:00:00:00:02", IP: "192.168.50.20", ExpiresAt: x.now.Add(30 * time.Second)}}}}
	x.report.Results = []JobResult{{ID: job.ID, LeaseID: x.job.LeaseID, Status: "succeeded"}}
	if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
		t.Fatal(err)
	}
	x.report.Results = nil
	state, err := x.s.Snapshot(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Contexts) != 1 || state.Contexts[0].Status != "ready" || state.Contexts[0].Epoch != 1 || len(state.Endpoints) != 1 || state.Endpoints[0].Status != "verified" || state.Endpoints[0].Revision != 1 || state.Jobs[0].Status != "succeeded" {
		t.Fatalf("unexpected connected state: %+v", state)
	}
	return Binding{ContextID: job.ID, Epoch: 1, EndpointID: state.Endpoints[0].ID, EndpointRevision: 1}
}

func TestService_Report_IdentityChangesInvalidatePinnedFlows(t *testing.T) {
	x := setupScenario(t)
	binding := x.connected(t)
	if err := x.s.Bind(t.Context(), 1, 42, binding); err != nil {
		t.Fatal(err)
	}
	if network, err := x.s.Check(t.Context(), 1, binding); err != nil || network.NetworkID != "network-1" {
		t.Fatalf("valid binding: %+v %v", network, err)
	}
	if _, err := x.s.Check(t.Context(), 2, binding); !errors.Is(err, ErrEnvironmentChanged) {
		t.Fatalf("cross-user binding: %v", err)
	}
	x.report.Sequence++
	x.report.Contexts[0].Endpoints[0].IP = "192.168.50.21"
	if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
		t.Fatal(err)
	}
	if _, err := x.s.Check(t.Context(), 1, binding); !errors.Is(err, ErrEnvironmentChanged) {
		t.Fatalf("old IP binding remained executable: %v", err)
	}
	state, err := x.s.Snapshot(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Endpoints[0].IP != "192.168.50.21" || state.Endpoints[0].Revision != 2 || state.Flows[0].Status != "blocked" {
		t.Fatalf("identity change not recorded: %+v", state)
	}
	public, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(public), "tokenHash") || strings.Contains(string(public), x.token) {
		t.Fatal("public state leaked a node credential")
	}
	binding.EndpointRevision = 2
	if _, err := x.s.Check(t.Context(), 1, binding); err != nil {
		t.Fatal(err)
	}
	x.now = x.now.Add(31 * time.Second)
	if _, err := x.s.Check(t.Context(), 1, binding); !errors.Is(err, ErrEnvironmentChanged) {
		t.Fatalf("expired context accepted: %v", err)
	}
	x.report.Sequence++
	if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
		t.Fatal(err)
	}
	if _, err := x.s.Check(t.Context(), 1, binding); !errors.Is(err, ErrEnvironmentChanged) {
		t.Fatalf("heartbeat revived an expired context: %v", err)
	}
}

func TestService_Report_VerifiesOnlyCurrentActiveFlowTargets(t *testing.T) {
	x := setupScenario(t)
	binding := x.connected(t)
	for _, id := range []int64{41, 42} {
		if err := x.s.Bind(t.Context(), 1, id, binding); err != nil {
			t.Fatal(err)
		}
	}
	x.report.Sequence++
	dispatch, err := x.s.Report(t.Context(), x.token, x.report)
	if err != nil || len(dispatch.VerificationTargets) != 1 {
		t.Fatalf("deduplicated active target: %+v %v", dispatch, err)
	}
	target := dispatch.VerificationTargets[0]
	if target.ContextID != binding.ContextID || target.IP != "192.168.50.20" || target.MAC != "02:00:00:00:00:02" || !target.ExpiresAt.Equal(dispatch.LeaseUntil) {
		t.Fatalf("incorrect target scope: %+v", target)
	}
	store := x.s.store.(*memoryStore)
	store.mu.Lock()
	store.inactive = map[int64]bool{41: true, 42: true}
	store.mu.Unlock()
	x.report.Sequence++
	dispatch, err = x.s.Report(t.Context(), x.token, x.report)
	if err != nil || len(dispatch.VerificationTargets) != 0 {
		t.Fatalf("inactive flows requested probes: %+v %v", dispatch, err)
	}
	store.mu.Lock()
	store.inactive = nil
	store.mu.Unlock()
	x.report.Contexts[0].Endpoints[0].IP = "192.168.50.21"
	x.report.Sequence++
	dispatch, err = x.s.Report(t.Context(), x.token, x.report)
	if err != nil || len(dispatch.VerificationTargets) != 0 {
		t.Fatalf("changed client binding requested probes: %+v %v", dispatch, err)
	}
}

func TestService_Report_HeartbeatDoesNotRenewCachedClientEvidence(t *testing.T) {
	x := setupScenario(t)
	binding := x.connected(t)
	x.now = x.now.Add(20 * time.Second)
	x.report.Sequence++
	if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
		t.Fatal(err)
	}
	x.now = x.now.Add(11 * time.Second)
	if _, err := x.s.Check(t.Context(), 1, binding); !errors.Is(err, ErrEnvironmentChanged) {
		t.Fatalf("cached client evidence was renewed by a heartbeat: %v", err)
	}
	state, err := x.s.Snapshot(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if state.Contexts[0].Status != "ready" || state.Endpoints[0].Status != "stale" {
		t.Fatalf("client cache expiry was not distinguished from network expiry: %+v", state)
	}
}

func TestService_Report_RejectsReplayStaleEvidenceAndRevokedNodes(t *testing.T) {
	x := setupScenario(t)
	if _, err := x.s.Report(t.Context(), x.token, x.report); !errors.Is(err, ErrConflict) {
		t.Fatalf("replay accepted: %v", err)
	}
	x.report.Sequence++
	x.report.Observations = []Observation{{Kind: "ap", MAC: "02:00:00:00:00:01", ObservedAt: x.now.Add(-31 * time.Second)}}
	if _, err := x.s.Report(t.Context(), x.token, x.report); err == nil || !strings.Contains(err.Error(), "observation is expired") {
		t.Fatalf("stale evidence accepted: %v", err)
	}
	if _, err := x.s.Command(t.Context(), 1, Command{Kind: "node.revoke", ID: x.nodeID}); err != nil {
		t.Fatal(err)
	}
	x.report.Observations = nil
	if _, err := x.s.Report(t.Context(), x.token, x.report); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("revoked node accepted: %v", err)
	}
}

func TestService_Report_NewObserverKeepsAreaAssetIdentity(t *testing.T) {
	x := setupScenario(t)
	before, err := x.s.Snapshot(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	node, err := x.s.Command(t.Context(), 1, Command{Kind: "node.create", Name: "Second observer", AreaID: x.areaID})
	if err != nil {
		t.Fatal(err)
	}
	x.now = x.now.Add(time.Second)
	_, err = x.s.Report(t.Context(), node.Token, Report{NodeID: node.ID, BootID: "second-boot", Sequence: 1, Observations: []Observation{{Kind: "ap", MAC: "02:00:00:00:00:01", SSID: "Lab", Frequency: 5180, Signal: -50, ObservedAt: x.now}}})
	if err != nil {
		t.Fatal(err)
	}
	after, err := x.s.Snapshot(t.Context(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Assets) != 1 || after.Assets[0].ID != before.Assets[0].ID || after.Assets[0].NodeID != node.ID || after.Assets[0].Frequency != 5180 || len(after.Observations) != 2 {
		t.Fatalf("asset identity or observation history lost: %+v", after)
	}
}

func TestService_Report_AmbiguousClientClaimsBlockEveryCandidate(t *testing.T) {
	for _, duplicate := range []Endpoint{{MAC: "02:00:00:00:00:02", IP: "192.168.50.21"}, {MAC: "02:00:00:00:00:03", IP: "192.168.50.20"}} {
		t.Run(duplicate.MAC+duplicate.IP, func(t *testing.T) {
			x := setupScenario(t)
			binding := x.connected(t)
			x.report.Sequence++
			duplicate.ExpiresAt = x.now.Add(30 * time.Second)
			x.report.Contexts[0].Endpoints = append(x.report.Contexts[0].Endpoints, duplicate)
			if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
				t.Fatal(err)
			}
			if _, err := x.s.Check(t.Context(), 1, binding); !errors.Is(err, ErrEnvironmentChanged) {
				t.Fatalf("ambiguous client remained executable: %v", err)
			}
			state, err := x.s.Snapshot(t.Context(), 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, endpoint := range state.Endpoints {
				if endpoint.Status == "verified" {
					t.Fatalf("ambiguous candidate was selected: %+v", endpoint)
				}
			}
		})
	}
}

func TestService_Report_ContextChangesNeverReviveAnOldEpoch(t *testing.T) {
	for _, change := range []string{"channel", "address", "runtime", "restart"} {
		t.Run(change, func(t *testing.T) {
			x := setupScenario(t)
			binding := x.connected(t)
			x.report.Sequence++
			switch change {
			case "channel":
				x.report.Contexts[0].Frequency = 5180
			case "address":
				x.report.Contexts[0].Address = "192.168.50.11/24"
			case "runtime":
				x.report.Contexts[0].NetworkID = "network-2"
			case "restart":
				x.report.BootID, x.report.Sequence = "boot-2", 1
			}
			if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
				t.Fatal(err)
			}
			if _, err := x.s.Check(t.Context(), 1, binding); !errors.Is(err, ErrEnvironmentChanged) {
				t.Fatalf("old epoch accepted: %v", err)
			}
			state, err := x.s.Snapshot(t.Context(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if state.Contexts[0].Epoch != 2 || state.Contexts[0].Status != "invalid" {
				t.Fatalf("context not invalidated exactly once: %+v", state.Contexts[0])
			}
			if change == "restart" {
				x.report.BootID, x.report.Sequence = "boot-1", 50
				if _, err := x.s.Report(t.Context(), x.token, x.report); !errors.Is(err, ErrConflict) {
					t.Fatalf("old boot replay accepted: %v", err)
				}
			}
		})
	}
}

func TestService_Command_CancelWaitsForCleanupAndUnknownJobsBlockRetries(t *testing.T) {
	x := setupScenario(t)
	start := Command{Kind: "job.start", NodeID: x.nodeID, JobKind: "scan", RequestID: "scan-request-1"}
	job, err := x.s.Command(t.Context(), 1, start)
	if err != nil {
		t.Fatal(err)
	}
	duplicate, err := x.s.Command(t.Context(), 1, start)
	if err != nil || duplicate.ID != job.ID {
		t.Fatalf("idempotent start: %+v %v", duplicate, err)
	}
	x.report.Sequence++
	dispatch, err := x.s.Report(t.Context(), x.token, x.report)
	if err != nil {
		t.Fatal(err)
	}
	lease := dispatch.Jobs[0].LeaseID
	x.now = x.now.Add(31 * time.Second)
	x.report.Sequence++
	if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
		t.Fatal(err)
	}
	start.RequestID = "scan-request-2"
	if _, err := x.s.Command(t.Context(), 1, start); err == nil || !strings.Contains(err.Error(), "unreconciled") {
		t.Fatalf("unknown job retried: %v", err)
	}
	if _, err := x.s.Command(t.Context(), 1, Command{Kind: "job.cancel", ID: job.ID}); err != nil {
		t.Fatal(err)
	}
	x.report.Sequence++
	x.report.Results = []JobResult{{ID: job.ID, LeaseID: lease, Status: "succeeded"}}
	dispatch, err = x.s.Report(t.Context(), x.token, x.report)
	if err != nil || len(dispatch.Jobs) != 1 || dispatch.Jobs[0].Status != "cancel_requested" {
		t.Fatalf("cancellation was not redelivered for cleanup: %+v %v", dispatch, err)
	}
	x.report.Sequence++
	x.report.Results[0].Status = "cancelled"
	if _, err := x.s.Report(t.Context(), x.token, x.report); err != nil {
		t.Fatal(err)
	}
	if _, err := x.s.Command(t.Context(), 1, start); err != nil {
		t.Fatalf("cleaned operation blocked new work: %v", err)
	}
}
