package node

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"pentagi/pkg/wireless"
)

func TestAgent_Serve_CancellationIsAcknowledgedAfterLocalCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var reports atomic.Int32
	var cleaned atomic.Bool
	receipt := make(chan wireless.JobResult, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+strings.Repeat("a", 64) {
			http.Error(w, "token missing", http.StatusUnauthorized)
			return
		}
		var report wireless.Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, "invalid report", 400)
			return
		}
		for _, result := range report.Results {
			if result.Status == "cancelled" {
				select {
				case receipt <- result:
				default:
				}
			}
		}
		status := "running"
		if reports.Add(1) > 1 {
			status = "cancel_requested"
		}
		_ = json.NewEncoder(w).Encode(wireless.Dispatch{Jobs: []wireless.Job{{ID: "00000000-0000-4000-8000-000000000001", NodeID: "00000000-0000-4000-8000-000000000002", Kind: "scan", Status: status, LeaseID: "lease-1"}}, LeaseUntil: time.Now().Add(30 * time.Second)})
	}))
	defer server.Close()
	agent := Agent{Config: Config{URL: server.URL, NodeID: "00000000-0000-4000-8000-000000000002", Token: strings.Repeat("a", 64), Interface: "wlan1", StateFile: filepath.Join(t.TempDir(), "state.json")}, Client: server.Client()}
	agent.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch name + " " + strings.Join(args, " ") {
		case "iw dev wlan1 scan flush passive":
			<-ctx.Done()
			return nil, ctx.Err()
		case "docker ps -q --filter label=pentagi.wireless.context=00000000-0000-4000-8000-000000000001":
			return []byte("container-1\n"), nil
		case "docker stop --time 1 container-1":
			cleaned.Store(true)
			return nil, nil
		case "iw dev wlan1 link":
			return []byte("Not connected.\n"), nil
		case "nmcli --wait 5 device disconnect wlan1":
			return nil, nil
		default:
			return nil, fmt.Errorf("unexpected command: %s %v", name, args)
		}
	}
	done := make(chan error, 1)
	go func() { done <- agent.Serve(ctx) }()
	select {
	case result := <-receipt:
		if !cleaned.Load() || result.LeaseID != "lease-1" || result.Message != "operation and association cleaned up" {
			t.Fatalf("receipt preceded cleanup: %+v", result)
		}
	case err := <-done:
		t.Fatalf("node stopped before acknowledging cleanup: %v", err)
	case <-time.After(30 * time.Second):
		t.Fatal("node did not acknowledge cancellation")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("node did not stop")
	}
}

func TestAgent_Serve_ReportsIdlePassiveObservations(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	observed := make(chan wireless.Observation, 1)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var report wireless.Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, "invalid report", 400)
			return
		}
		for _, observation := range report.Observations {
			select {
			case observed <- observation:
			default:
			}
		}
		_ = json.NewEncoder(w).Encode(wireless.Dispatch{LeaseUntil: time.Now().Add(30 * time.Second)})
	}))
	defer server.Close()
	agent := Agent{Config: Config{URL: server.URL, NodeID: "00000000-0000-4000-8000-000000000002", Token: strings.Repeat("a", 64), Interface: "wlan1", StateFile: filepath.Join(t.TempDir(), "state.json"), ScanInterval: 10 * time.Second}, Client: server.Client()}
	agent.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if name != "iw" || strings.Join(args, " ") != "dev wlan1 scan flush passive" {
			return nil, fmt.Errorf("unexpected command %s %v", name, args)
		}
		return []byte("BSS 02:00:00:00:00:01(on wlan1)\n freq: 2412\n signal: -45 dBm\n SSID: Lab\n"), nil
	}
	done := make(chan error, 1)
	go func() { done <- agent.Serve(ctx) }()
	select {
	case observation := <-observed:
		if observation.MAC != "02:00:00:00:00:01" || observation.Frequency != 2412 || observation.SSID != "Lab" || observation.ObservedAt.IsZero() {
			t.Fatalf("invalid idle observation: %+v", observation)
		}
	case err := <-done:
		t.Fatalf("node stopped before observation: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("idle node did not report passive observations")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("node did not stop")
	}
}

func TestAgent_Serve_RefreshesOnlyLeasedTargetsOnVerifiedRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	const contextID = "00000000-0000-4000-8000-000000000001"
	confirmed := make(chan wireless.Endpoint, 1)
	var probes atomic.Int32
	var disconnected atomic.Bool
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var report wireless.Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, "invalid report", 400)
			return
		}
		for _, network := range report.Contexts {
			for _, endpoint := range network.Endpoints {
				select {
				case confirmed <- endpoint:
				default:
				}
			}
		}
		_ = json.NewEncoder(w).Encode(wireless.Dispatch{LeaseUntil: time.Now().Add(30 * time.Second), VerificationTargets: []wireless.VerificationTarget{
			{ContextID: contextID, IP: "192.168.50.20", MAC: "02:00:00:00:00:02", ExpiresAt: time.Now().Add(30 * time.Second)},
			{ContextID: "other-context", IP: "192.168.50.21", ExpiresAt: time.Now().Add(30 * time.Second)},
			{ContextID: contextID, IP: "10.0.0.20", ExpiresAt: time.Now().Add(30 * time.Second)},
			{ContextID: contextID, IP: "192.168.50.22", ExpiresAt: time.Now().Add(-time.Second)},
		}})
	}))
	defer server.Close()
	agent := Agent{Config: Config{URL: server.URL, NodeID: "00000000-0000-4000-8000-000000000002", Token: strings.Repeat("a", 64), Interface: "wlan1", Network: "wireless-lab", StateFile: filepath.Join(t.TempDir(), "state.json")}, Client: server.Client(), active: &wireless.ContextReport{ID: contextID, BSSID: "02:00:00:00:00:01", Frequency: 2412, DaemonID: "linux-daemon", NetworkID: "network-1", Address: "192.168.50.10/24"}}
	agent.Run = func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch name + " " + strings.Join(args, " ") {
		case "iw dev wlan1 link":
			if disconnected.Load() {
				return []byte("Not connected.\n"), nil
			}
			return []byte("Connected to 02:00:00:00:00:01 (on wlan1)\n freq: 2412\n"), nil
		case "ip -j -4 address show dev wlan1":
			return []byte(`[{"addr_info":[{"family":"inet","local":"192.168.50.10","prefixlen":24,"scope":"global"}]}]`), nil
		case "docker info --format {{.ID}}":
			return []byte("linux-daemon\n"), nil
		case "docker network inspect wireless-lab":
			return []byte(`[{"Id":"network-1","Name":"wireless-lab","Driver":"ipvlan","Options":{"parent":"wlan1"},"IPAM":{"Config":[{"Subnet":"192.168.50.0/24"}]}}]`), nil
		case "ip -j -s -4 neigh show dev wlan1":
			return []byte(`[{"dst":"192.168.50.20","lladdr":"02:00:00:00:00:02","state":["REACHABLE"],"confirmed":40}]`), nil
		case "arping -c 1 -w 1 -I wlan1 192.168.50.20":
			probes.Add(1)
			return []byte("Unicast reply from 192.168.50.20 [02:00:00:00:00:99] 1.234ms\n"), nil
		case "docker ps -q --filter label=pentagi.wireless.context=" + contextID:
			return nil, nil
		case "nmcli --wait 5 device disconnect wlan1":
			disconnected.Store(true)
			return nil, nil
		default:
			t.Errorf("unscoped command: %s %v", name, args)
			return nil, fmt.Errorf("unexpected command: %s %v", name, args)
		}
	}
	done := make(chan error, 1)
	go func() { done <- agent.Serve(ctx) }()
	select {
	case endpoint := <-confirmed:
		if probes.Load() != 1 || endpoint.IP != "192.168.50.20" || endpoint.MAC != "02:00:00:00:00:99" || !endpoint.ExpiresAt.After(time.Now()) || endpoint.ExpiresAt.After(time.Now().Add(30*time.Second)) {
			t.Fatalf("probe failed to report actual identity: %+v, probes %d", endpoint, probes.Load())
		}
	case err := <-done:
		t.Fatalf("node stopped before target verification: %v", err)
	case <-time.After(25 * time.Second):
		t.Fatal("node did not verify its leased target")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) || !disconnected.Load() {
			t.Fatalf("shutdown cleanup: %v, disconnected %v", err, disconnected.Load())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("node did not stop")
	}
}
