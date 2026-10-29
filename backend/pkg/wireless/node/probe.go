package node

import (
	"context"
	"net"
	"net/netip"
	"strings"
	"time"

	"pentagi/pkg/wireless"
)

func ParseARPReply(raw, ip string, confirmedAt time.Time) (wireless.Endpoint, bool) {
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 || (fields[0] != "Unicast" && fields[0] != "Broadcast") || fields[1] != "reply" || fields[2] != "from" || fields[3] != ip {
			continue
		}
		m, err := net.ParseMAC(strings.Trim(fields[4], "[]"))
		if err != nil || len(m) != 6 || m[0]&1 != 0 {
			continue
		}
		return wireless.Endpoint{MAC: m.String(), IP: ip, ExpiresAt: confirmedAt.Add(29 * time.Second)}, true
	}
	return wireless.Endpoint{}, false
}

func (a *Agent) probeTargets(ctx context.Context, network wireless.ContextReport, subnet netip.Prefix) []wireless.Endpoint {
	a.mu.Lock()
	lease := a.lease
	targets := []wireless.VerificationTarget{}
	if lease.After(time.Now()) && len(a.targets) > 0 {
		for offset := 0; offset < len(a.targets) && len(targets) < 8; offset++ {
			i := (a.targetCursor + offset) % len(a.targets)
			target := a.targets[i]
			ip, err := netip.ParseAddr(target.IP)
			if err == nil && subnet.Contains(ip) && target.ContextID == network.ID && target.ExpiresAt.After(time.Now()) {
				targets = append(targets, target)
			}
		}
		a.targetCursor = (a.targetCursor + 8) % len(a.targets)
	}
	a.mu.Unlock()
	probeContext, stop := context.WithDeadline(ctx, lease)
	defer stop()
	results := make(chan wireless.Endpoint, len(targets))
	for _, target := range targets {
		go func() {
			probe, cancel := context.WithTimeout(probeContext, 1500*time.Millisecond)
			defer cancel()
			confirmedAt := time.Now().UTC()
			raw, err := a.Run(probe, "arping", "-c", "1", "-w", "1", "-I", a.Config.Interface, target.IP)
			if err == nil {
				if endpoint, ok := ParseARPReply(string(raw), target.IP, confirmedAt); ok {
					results <- endpoint
					return
				}
			}
			results <- wireless.Endpoint{}
		}()
	}
	endpoints := []wireless.Endpoint{}
	for range targets {
		endpoint := <-results
		if endpoint.MAC != "" {
			endpoints = append(endpoints, endpoint)
		}
	}
	return endpoints
}
