package node

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"pentagi/pkg/wireless"

	"github.com/google/uuid"
)

type Config struct {
	URL          string
	NodeID       string
	Token        string
	Interface    string
	Network      string
	StateFile    string
	Profiles     map[string]string
	ScanInterval time.Duration
}

type Runner func(context.Context, string, ...string) ([]byte, error)

type Agent struct {
	Config       Config
	Run          Runner
	Client       *http.Client
	mu           sync.Mutex
	active       *wireless.ContextReport
	observations []wireless.Observation
	results      []wireless.JobResult
	job          *wireless.Job
	cancel       context.CancelFunc
	lease        time.Time
	busy         bool
	cleanupMu    sync.Mutex
	journalMu    sync.Mutex
	nextScan     time.Time
	targets      []wireless.VerificationTarget
	targetCursor int
}

type journal struct {
	ContextID string `json:"contextId"`
}

func Exec(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var output cappedBuffer
	cmd.Stdout, cmd.Stderr = &output, &output
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("%s failed: %w", name, err)
	}
	return output.Bytes(), nil
}

type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, errors.New("node command output exceeds limit")
	}
	return b.Buffer.Write(p)
}

func (a *Agent) Serve(ctx context.Context) error {
	parsed, err := url.Parse(a.Config.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return errors.New("an HTTPS control-plane URL is required")
	}
	if _, err := uuid.Parse(a.Config.NodeID); err != nil {
		return errors.New("invalid node ID")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,15}$`).MatchString(a.Config.Interface) || a.Config.StateFile == "" || len(a.Config.Token) != 64 {
		return errors.New("interface, state file and enrollment token are required")
	}
	if a.Run == nil {
		a.Run = Exec
	}
	if a.Client == nil {
		a.Client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	if err := os.MkdirAll(filepath.Dir(a.Config.StateFile), 0700); err != nil {
		return err
	}
	if raw, err := os.ReadFile(a.Config.StateFile); err == nil {
		var old journal
		if err := json.Unmarshal(raw, &old); err != nil {
			return err
		}
		if old.ContextID != "" {
			if err := a.cleanup(ctx, old.ContextID); err != nil {
				return fmt.Errorf("recover previous association: %w", err)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	bootID := uuid.NewString()
	go func() {
		watch := time.NewTicker(time.Second)
		defer watch.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-watch.C:
				cleanup, cancel := context.WithTimeout(ctx, 10*time.Second)
				if err := a.expire(cleanup); err != nil {
					log.Printf("wireless lease cleanup failed: %v", err)
				}
				cancel()
			}
		}
	}()
	sequence := int64(0)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	defer func() {
		recovery, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		a.mu.Lock()
		if a.cancel != nil {
			a.cancel()
		}
		active := a.active
		a.mu.Unlock()
		if active != nil {
			_ = a.cleanup(recovery, active.ID)
		}
	}()
	for {
		sequence++
		if err := a.beat(ctx, bootID, sequence); err != nil {
			log.Printf("wireless heartbeat failed: %v", err)
		}
		if err := a.expire(ctx); err != nil {
			log.Printf("wireless lease cleanup failed: %v", err)
		}
		a.observeIdle(ctx)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (a *Agent) observeIdle(ctx context.Context) {
	a.mu.Lock()
	if a.Config.ScanInterval < 10*time.Second || a.busy || a.active != nil || !a.lease.After(time.Now()) || a.nextScan.After(time.Now()) {
		a.mu.Unlock()
		return
	}
	scan, cancel := context.WithTimeout(ctx, 15*time.Second)
	a.busy, a.cancel, a.nextScan = true, cancel, time.Now().Add(a.Config.ScanInterval)
	a.mu.Unlock()
	go func() {
		defer cancel()
		raw, err := a.Run(scan, "iw", "dev", a.Config.Interface, "scan", "flush", "passive")
		observations := []wireless.Observation{}
		if err == nil {
			observations = ParseScan(string(raw), time.Now().UTC())
		}
		a.mu.Lock()
		a.observations = append(a.observations, observations...)
		a.busy, a.cancel = false, nil
		a.mu.Unlock()
		if err != nil && ctx.Err() == nil {
			log.Printf("wireless passive observation failed: %v", err)
		}
	}()
}

func (a *Agent) beat(ctx context.Context, bootID string, sequence int64) error {
	a.mu.Lock()
	a.observations = slices.DeleteFunc(a.observations, func(o wireless.Observation) bool { return !o.ObservedAt.Add(25 * time.Second).After(time.Now()) })
	report := wireless.Report{NodeID: a.Config.NodeID, BootID: bootID, Sequence: sequence, Capabilities: []string{"scan"}, Observations: slices.Clone(a.observations), Results: slices.Clone(a.results)}
	if len(report.Observations) > 500 {
		report.Observations = report.Observations[:500]
	}
	if len(a.Config.Profiles) > 0 && a.Config.Network != "" {
		report.Capabilities = append(report.Capabilities, "connect", "disconnect")
	}
	active := a.active
	a.mu.Unlock()
	if active != nil {
		probe, cancel := context.WithTimeout(ctx, 4*time.Second)
		current, err := a.inspect(probe, *active)
		cancel()
		if err != nil {
			current = *active
			current.Connected = false
			current.Endpoints = nil
		}
		if current.Address != active.Address || current.NetworkID != active.NetworkID || current.DaemonID != active.DaemonID || current.Frequency != active.Frequency {
			current.Connected = false
			current.Endpoints = nil
		}
		report.Contexts = []wireless.ContextReport{current}
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.Config.URL, "/")+"/api/v1/wireless/node/report", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+a.Config.Token)
	request.Header.Set("Content-Type", "application/json")
	response, err := a.Client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("control plane returned HTTP %d", response.StatusCode)
	}
	var dispatch wireless.Dispatch
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&dispatch); err != nil {
		return err
	}
	a.mu.Lock()
	a.lease = time.Now().Add(20 * time.Second)
	a.targets = slices.Clone(dispatch.VerificationTargets)
	if len(report.Observations) <= len(a.observations) {
		a.observations = a.observations[len(report.Observations):]
	}
	if len(report.Results) <= len(a.results) {
		a.results = a.results[len(report.Results):]
	}
	if active != nil && len(report.Contexts) > 0 && !report.Contexts[0].Connected && a.active == active {
		a.lease = time.Time{}
	}
	a.mu.Unlock()
	for _, job := range dispatch.Jobs {
		a.accept(ctx, job)
	}
	if active != nil && slices.Contains(dispatch.RevokeContexts, active.ID) {
		cleanup, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if err := a.cleanup(cleanup, active.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *Agent) accept(ctx context.Context, job wireless.Job) {
	a.mu.Lock()
	if a.busy {
		if a.job != nil && a.job.ID == job.ID && (job.Status == "cancel_requested" || job.Status == "unknown") {
			a.cancel()
		}
		a.mu.Unlock()
		return
	}
	if slices.ContainsFunc(a.results, func(r wireless.JobResult) bool { return r.ID == job.ID && r.LeaseID == job.LeaseID }) {
		a.mu.Unlock()
		return
	}
	jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	a.cancel, a.job, a.busy = cancel, &job, true
	a.mu.Unlock()
	go func() {
		defer cancel()
		status, message := "succeeded", "operation completed"
		err := a.execute(jobCtx, job)
		if err != nil {
			status, message = "failed", err.Error()
		}
		if err != nil && job.Kind == "connect" && jobCtx.Err() == nil {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			cleanupErr := a.cleanup(cleanup, job.ID)
			stop()
			if cleanupErr != nil {
				a.mu.Lock()
				a.results = append(a.results, wireless.JobResult{ID: job.ID, LeaseID: job.LeaseID, Status: "unknown", Message: "connection cleanup requires reconciliation"})
				a.busy, a.cancel, a.job = false, nil, nil
				a.mu.Unlock()
				return
			}
		}
		if jobCtx.Err() != nil || job.Status == "cancel_requested" || job.Status == "unknown" {
			cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
			cleanupErr := a.cleanup(cleanup, job.ID)
			stop()
			if cleanupErr != nil {
				a.mu.Lock()
				a.results = append(a.results, wireless.JobResult{ID: job.ID, LeaseID: job.LeaseID, Status: "unknown", Message: "cancellation cleanup requires reconciliation"})
				a.busy, a.cancel, a.job = false, nil, nil
				a.mu.Unlock()
				return
			} else {
				status, message = "cancelled", "operation and association cleaned up"
			}
		}
		a.mu.Lock()
		a.results = append(a.results, wireless.JobResult{ID: job.ID, LeaseID: job.LeaseID, Status: status, Message: message})
		a.busy, a.cancel, a.job = false, nil, nil
		a.mu.Unlock()
	}()
}

func (a *Agent) execute(ctx context.Context, job wireless.Job) error {
	if job.Status == "cancel_requested" || job.Status == "unknown" {
		return context.Canceled
	}
	switch job.Kind {
	case "scan":
		a.mu.Lock()
		connected := a.active != nil
		a.mu.Unlock()
		if connected {
			return errors.New("disconnect before changing the radio's observation schedule")
		}
		raw, err := a.Run(ctx, "iw", "dev", a.Config.Interface, "scan", "flush", "passive")
		if err != nil {
			return err
		}
		observations := ParseScan(string(raw), time.Now().UTC())
		a.mu.Lock()
		a.observations = append(a.observations, observations...)
		a.mu.Unlock()
		return nil
	case "connect":
		profile, ok := a.Config.Profiles[job.BSSID]
		if !ok {
			return errors.New("no saved profile is configured for this BSSID")
		}
		if _, err := uuid.Parse(profile); err != nil {
			return errors.New("profile must be a NetworkManager connection UUID")
		}
		a.mu.Lock()
		existing := a.active
		a.mu.Unlock()
		if existing != nil {
			return errors.New("disconnect the previous association first")
		}
		pinned, err := a.Run(ctx, "nmcli", "-g", "802-11-wireless.bssid", "connection", "show", "uuid", profile)
		if err != nil || !strings.EqualFold(strings.TrimSpace(string(pinned)), job.BSSID) {
			return errors.New("saved profile must be pinned to the exact BSSID")
		}
		if err := a.save(job.ID); err != nil {
			return err
		}
		if _, err := a.Run(ctx, "nmcli", "--wait", "15", "connection", "up", "uuid", profile, "ifname", a.Config.Interface); err != nil {
			return err
		}
		current, err := a.inspect(ctx, wireless.ContextReport{ID: job.ID, JobID: job.ID, LeaseID: job.LeaseID, BSSID: job.BSSID})
		if err != nil {
			return err
		}
		a.mu.Lock()
		a.active = &current
		a.mu.Unlock()
		return nil
	case "disconnect":
		a.mu.Lock()
		active := a.active
		a.mu.Unlock()
		if active != nil {
			return a.cleanup(ctx, active.ID)
		}
		return nil
	default:
		return errors.New("unsupported node operation")
	}
}

func (a *Agent) inspect(ctx context.Context, current wireless.ContextReport) (wireless.ContextReport, error) {
	expected := current
	link, err := a.Run(ctx, "iw", "dev", a.Config.Interface, "link")
	if err != nil {
		return current, err
	}
	if !strings.Contains(strings.ToLower(string(link)), "connected to "+current.BSSID) {
		return current, errors.New("associated BSSID does not match")
	}
	for _, line := range strings.Split(string(link), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "freq:" {
			current.Frequency, _ = strconv.Atoi(fields[1])
		}
	}
	if current.Frequency == 0 {
		return current, errors.New("association frequency is unavailable")
	}
	addresses, err := a.Run(ctx, "ip", "-j", "-4", "address", "show", "dev", a.Config.Interface)
	if err != nil {
		return current, err
	}
	var interfaces []struct {
		Addresses []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
			Prefix int    `json:"prefixlen"`
			Scope  string `json:"scope"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(addresses, &interfaces); err != nil {
		return current, err
	}
	current.Address = ""
	for _, iface := range interfaces {
		for _, address := range iface.Addresses {
			if address.Family == "inet" && address.Scope == "global" {
				current.Address = fmt.Sprintf("%s/%d", address.Local, address.Prefix)
			}
		}
	}
	if current.Address == "" {
		return current, errors.New("wireless interface has no IPv4 address")
	}
	daemon, err := a.Run(ctx, "docker", "info", "--format", "{{.ID}}")
	if err != nil {
		return current, err
	}
	network, err := a.Run(ctx, "docker", "network", "inspect", a.Config.Network)
	if err != nil {
		return current, err
	}
	var networks []struct {
		ID      string `json:"Id"`
		Name    string
		Driver  string
		Options map[string]string
		IPAM    struct{ Config []struct{ Subnet string } }
	}
	if err := json.Unmarshal(network, &networks); err != nil || len(networks) != 1 {
		return current, errors.New("invalid Docker network inspection")
	}
	n := networks[0]
	if n.Options["parent"] != a.Config.Interface || (n.Driver != "ipvlan" && n.Driver != "macvlan") {
		return current, errors.New("runtime network must use the dedicated wireless interface")
	}
	prefix, _ := netip.ParsePrefix(current.Address)
	if !slices.ContainsFunc(n.IPAM.Config, func(config struct{ Subnet string }) bool {
		p, err := netip.ParsePrefix(config.Subnet)
		return err == nil && p.Masked() == prefix.Masked()
	}) {
		return current, errors.New("runtime subnet does not match the wireless association")
	}
	current.DaemonID, current.NetworkID, current.Network, current.Interface, current.Connected = strings.TrimSpace(string(daemon)), n.ID, n.Name, a.Config.Interface, true
	if expected.NetworkID != "" && (expected.DaemonID != current.DaemonID || expected.NetworkID != current.NetworkID || expected.Address != current.Address || expected.Frequency != current.Frequency) {
		return current, errors.New("wireless runtime identity changed")
	}
	confirmedAt := time.Now().UTC()
	neighbors, err := a.Run(ctx, "ip", "-j", "-s", "-4", "neigh", "show", "dev", a.Config.Interface)
	if err != nil {
		return current, err
	}
	var entries []struct {
		IP        string   `json:"dst"`
		MAC       string   `json:"lladdr"`
		State     []string `json:"state"`
		Confirmed *uint64  `json:"confirmed"`
	}
	if err := json.Unmarshal(neighbors, &entries); err != nil {
		return current, err
	}
	current.Endpoints = a.probeTargets(ctx, current, prefix)
	for _, entry := range entries {
		ip, parseErr := netip.ParseAddr(entry.IP)
		if parseErr == nil && prefix.Contains(ip) && slices.Contains(entry.State, "REACHABLE") && entry.MAC != "" && entry.Confirmed != nil && *entry.Confirmed < 29 {
			current.Endpoints = append(current.Endpoints, wireless.Endpoint{MAC: entry.MAC, IP: entry.IP, ExpiresAt: confirmedAt.Add(30*time.Second - time.Duration(*entry.Confirmed+1)*time.Second)})
		}
	}
	return current, nil
}

func (a *Agent) expire(ctx context.Context) error {
	a.mu.Lock()
	expired := !a.lease.After(time.Now())
	active, job := a.active, a.job
	if expired && a.cancel != nil {
		a.cancel()
	}
	a.mu.Unlock()
	if expired && active != nil {
		return a.cleanup(ctx, active.ID)
	}
	if expired && job != nil && job.Kind == "connect" {
		return a.cleanup(ctx, job.ID)
	}
	return nil
}

func (a *Agent) cleanup(ctx context.Context, id string) error {
	a.cleanupMu.Lock()
	defer a.cleanupMu.Unlock()
	if _, err := uuid.Parse(id); err != nil {
		return errors.New("invalid context journal")
	}
	containers, err := a.Run(ctx, "docker", "ps", "-q", "--filter", "label=pentagi.wireless.context="+id)
	if err != nil {
		return err
	}
	ids := strings.Fields(string(containers))
	if len(ids) > 0 {
		if _, err := a.Run(ctx, "docker", append([]string{"stop", "--time", "1"}, ids...)...); err != nil {
			return err
		}
	}
	if _, err := a.Run(ctx, "nmcli", "--wait", "5", "device", "disconnect", a.Config.Interface); err != nil {
		state, inspectErr := a.Run(ctx, "nmcli", "-g", "GENERAL.STATE", "device", "show", a.Config.Interface)
		if inspectErr != nil || (!strings.HasPrefix(strings.TrimSpace(string(state)), "30 ") && !strings.HasPrefix(strings.TrimSpace(string(state)), "20 ")) {
			return err
		}
	}
	link, err := a.Run(ctx, "iw", "dev", a.Config.Interface, "link")
	if err != nil {
		return err
	}
	if strings.Contains(string(link), "Connected to ") {
		return errors.New("wireless interface remains associated after cleanup")
	}
	if err := a.save(""); err != nil {
		return err
	}
	a.mu.Lock()
	a.active = nil
	a.targets = nil
	a.mu.Unlock()
	return nil
}

func (a *Agent) save(id string) error {
	a.journalMu.Lock()
	defer a.journalMu.Unlock()
	raw, err := json.Marshal(journal{ContextID: id})
	if err != nil {
		return err
	}
	file, err := os.OpenFile(a.Config.StateFile+".tmp", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(a.Config.StateFile+".tmp", a.Config.StateFile); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(a.Config.StateFile))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
