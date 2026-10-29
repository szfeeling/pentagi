package node

import (
	"testing"
	"time"
)

func TestScan_ParseScan_PreservesBSSIDChannelAndSecurity(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	result := ParseScan(`BSS 02:00:00:00:00:01(on wlan1)
	freq: 2412
	signal: -46.50 dBm
	SSID: SharedSSID
	RSN:
		 * Authentication suites: SAE
BSS 02:00:00:00:00:02(on wlan1)
	freq: 5180
	signal: -62.00 dBm
	SSID: SharedSSID
BSS invalid(on wlan1)
	SSID: ignored
`, now)
	if len(result) != 2 {
		t.Fatalf("got %d observations", len(result))
	}
	if result[0].MAC != "02:00:00:00:00:01" || result[0].Frequency != 2412 || result[0].Signal != -46.5 || result[0].Security != "SAE" || result[0].SSID != "SharedSSID" || !result[0].ObservedAt.Equal(now) {
		t.Fatalf("first AP: %+v", result[0])
	}
	if result[1].MAC != "02:00:00:00:00:02" || result[1].Frequency != 5180 || result[1].Security != "open" {
		t.Fatalf("same SSID merged distinct APs: %+v", result)
	}
}

func TestScan_ParseScan_PreservesEvidenceAgeAndDropsExpiredCacheEntries(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	result := ParseScan("BSS 02:00:00:00:00:01(on wlan1)\n last seen: 500 ms ago\n SSID: Recent\nBSS 02:00:00:00:00:02(on wlan1)\n last seen: 31000 ms ago\n SSID: Cached\n", now)
	if len(result) != 1 || result[0].MAC != "02:00:00:00:00:01" || !result[0].ObservedAt.Equal(now.Add(-500*time.Millisecond)) {
		t.Fatalf("cache was presented as fresh evidence: %+v", result)
	}
}
