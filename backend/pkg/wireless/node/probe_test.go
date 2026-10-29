package node

import (
	"testing"
	"time"
)

func TestProbe_ParseARPReply_UsesActualReplyIdentityAndConfirmationTime(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, kind := range []string{"Unicast", "Broadcast"} {
		raw := "ARPING 192.168.50.20 from 192.168.50.10 wlan1\n" + kind + " reply from 192.168.50.20 [02:00:00:00:00:99] 1.234ms\nSent 1 probes (1 broadcast(s))\nReceived 1 response(s)\n"
		endpoint, ok := ParseARPReply(raw, "192.168.50.20", now)
		if !ok || endpoint.MAC != "02:00:00:00:00:99" || endpoint.IP != "192.168.50.20" || !endpoint.ExpiresAt.Equal(now.Add(29*time.Second)) {
			t.Fatalf("%s reply: %+v %v", kind, endpoint, ok)
		}
	}
	for _, raw := range []string{
		"Unicast reply from 192.168.50.21 [02:00:00:00:00:02] 1ms",
		"Unicast request from 192.168.50.20 [02:00:00:00:00:02] 1ms",
		"Unicast reply from 192.168.50.20 [FF:FF:FF:FF:FF:FF] 1ms",
		"Unicast reply from 192.168.50.20 [invalid] 1ms",
		"Sent 1 probes (1 broadcast(s))\nReceived 0 response(s)",
	} {
		if endpoint, ok := ParseARPReply(raw, "192.168.50.20", now); ok {
			t.Fatalf("accepted unrelated or absent reply: %q %+v", raw, endpoint)
		}
	}
}
