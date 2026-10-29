package node

import (
	"bufio"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"pentagi/pkg/wireless"
)

func ParseScan(raw string, observedAt time.Time) []wireless.Observation {
	result := []wireless.Observation{}
	scanner := bufio.NewScanner(strings.NewReader(raw))
	var current *wireless.Observation
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "BSS ") {
			value := strings.Split(strings.Fields(line)[1], "(")[0]
			m, err := net.ParseMAC(value)
			if err != nil || len(m) != 6 || m[0]&1 != 0 {
				current = nil
				continue
			}
			result = append(result, wireless.Observation{Kind: "ap", MAC: m.String(), BSSID: m.String(), Security: "open", ObservedAt: observedAt})
			current = &result[len(result)-1]
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "SSID:"):
			ssid := strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
			if decoded, err := strconv.Unquote(`"` + strings.ReplaceAll(ssid, `"`, `\"`) + `"`); err == nil {
				ssid = decoded
			}
			if len(ssid) <= 32 && utf8.ValidString(ssid) {
				current.SSID = ssid
			}
		case strings.HasPrefix(line, "freq:"):
			current.Frequency, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "freq:")))
		case strings.HasPrefix(line, "last seen:"):
			fields := strings.Fields(strings.TrimPrefix(line, "last seen:"))
			if len(fields) >= 2 && fields[1] == "ms" {
				milliseconds, err := strconv.ParseInt(fields[0], 10, 64)
				if err == nil && milliseconds >= 0 && milliseconds <= 86400000 {
					current.ObservedAt = observedAt.Add(-time.Duration(milliseconds) * time.Millisecond)
				} else {
					current.ObservedAt = time.Time{}
				}
			} else {
				current.ObservedAt = time.Time{}
			}
		case strings.HasPrefix(line, "signal:"):
			fields := strings.Fields(strings.TrimPrefix(line, "signal:"))
			if len(fields) > 0 {
				current.Signal, _ = strconv.ParseFloat(fields[0], 64)
			}
		case strings.HasPrefix(line, "RSN:"):
			current.Security = "RSN"
		case strings.Contains(line, "Authentication suites:"):
			current.Security = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		case strings.HasPrefix(line, "WPA:"):
			current.Security = "WPA"
		case strings.HasPrefix(line, "capability:") && strings.Contains(line, "Privacy"):
			current.Security = "protected"
		}
	}
	result = slices.DeleteFunc(result, func(o wireless.Observation) bool { return !o.ObservedAt.Add(wireless.LeaseDuration).After(observedAt) })
	if len(result) > 500 {
		result = result[:500]
	}
	return result
}
