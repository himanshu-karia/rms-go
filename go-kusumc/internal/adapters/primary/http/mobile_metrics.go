package http

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

var mobileAuthRequestOtpTotal atomic.Int64
var mobileAuthVerifyTotal atomic.Int64
var mobileAuthRefreshTotal atomic.Int64
var mobileAuthLogoutTotal atomic.Int64
var mobileAssignmentsTotal atomic.Int64
var mobileIngestAcceptedTotal atomic.Int64
var mobileIngestDuplicateTotal atomic.Int64
var mobileIngestRejectedTotal atomic.Int64
var mobileCommandStatusTotal atomic.Int64

func incMobileAuthRequestOtp() { mobileAuthRequestOtpTotal.Add(1) }
func incMobileAuthVerify()     { mobileAuthVerifyTotal.Add(1) }
func incMobileAuthRefresh()    { mobileAuthRefreshTotal.Add(1) }
func incMobileAuthLogout()     { mobileAuthLogoutTotal.Add(1) }
func incMobileAssignments()    { mobileAssignmentsTotal.Add(1) }
func addMobileIngestAccepted(v int) {
	if v > 0 {
		mobileIngestAcceptedTotal.Add(int64(v))
	}
}
func addMobileIngestDuplicate(v int) {
	if v > 0 {
		mobileIngestDuplicateTotal.Add(int64(v))
	}
}
func addMobileIngestRejected(v int) {
	if v > 0 {
		mobileIngestRejectedTotal.Add(int64(v))
	}
}
func incMobileCommandStatus() { mobileCommandStatusTotal.Add(1) }

type mobileEndpointStat struct {
	Requests       int64
	Errors         int64
	LatencyLE100ms int64
	LatencyLE300ms int64
	LatencyLE1000  int64
	LatencyGT1000  int64
}

var mobileEndpointStats = struct {
	mu sync.Mutex
	m  map[string]*mobileEndpointStat
}{m: map[string]*mobileEndpointStat{}}

func recordMobileEndpoint(endpoint string, elapsed time.Duration, isError bool) {
	if endpoint == "" {
		endpoint = "unknown"
	}
	mobileEndpointStats.mu.Lock()
	defer mobileEndpointStats.mu.Unlock()
	st, ok := mobileEndpointStats.m[endpoint]
	if !ok {
		st = &mobileEndpointStat{}
		mobileEndpointStats.m[endpoint] = st
	}
	st.Requests++
	if isError {
		st.Errors++
	}
	ms := elapsed.Milliseconds()
	switch {
	case ms <= 100:
		st.LatencyLE100ms++
	case ms <= 300:
		st.LatencyLE300ms++
	case ms <= 1000:
		st.LatencyLE1000++
	default:
		st.LatencyGT1000++
	}
}

func snapshotMobileEndpointStats() map[string]mobileEndpointStat {
	mobileEndpointStats.mu.Lock()
	defer mobileEndpointStats.mu.Unlock()
	out := make(map[string]mobileEndpointStat, len(mobileEndpointStats.m))
	for k, v := range mobileEndpointStats.m {
		out[k] = *v
	}
	return out
}

func renderMobileEndpointMetrics() string {
	stats := snapshotMobileEndpointStats()
	if len(stats) == 0 {
		return ""
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := ""
	out += "# HELP mobile_endpoint_requests_total Total mobile API requests by endpoint\n"
	out += "# TYPE mobile_endpoint_requests_total counter\n"
	for _, k := range keys {
		out += fmt.Sprintf("mobile_endpoint_requests_total{endpoint=\"%s\"} %d\n", k, stats[k].Requests)
	}

	out += "# HELP mobile_endpoint_errors_total Total mobile API errors by endpoint\n"
	out += "# TYPE mobile_endpoint_errors_total counter\n"
	for _, k := range keys {
		out += fmt.Sprintf("mobile_endpoint_errors_total{endpoint=\"%s\"} %d\n", k, stats[k].Errors)
	}

	out += "# HELP mobile_endpoint_latency_bucket Mobile API latency buckets by endpoint\n"
	out += "# TYPE mobile_endpoint_latency_bucket counter\n"
	for _, k := range keys {
		st := stats[k]
		out += fmt.Sprintf("mobile_endpoint_latency_bucket{endpoint=\"%s\",le=\"100\"} %d\n", k, st.LatencyLE100ms)
		out += fmt.Sprintf("mobile_endpoint_latency_bucket{endpoint=\"%s\",le=\"300\"} %d\n", k, st.LatencyLE300ms)
		out += fmt.Sprintf("mobile_endpoint_latency_bucket{endpoint=\"%s\",le=\"1000\"} %d\n", k, st.LatencyLE1000)
		out += fmt.Sprintf("mobile_endpoint_latency_bucket{endpoint=\"%s\",le=\"+Inf\"} %d\n", k, st.LatencyGT1000)
	}

	return out
}
