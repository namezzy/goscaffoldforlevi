package observability_test

import (
	"encoding/json"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/namezzy/goscaffoldforlevi/internal/telemetry"
)

// A dashboard is the one artefact that fails silently. A panel querying a
// metric that does not exist renders as an empty graph, which is
// indistinguishable from a metric that is genuinely zero — so a typo in a
// dashboard is discovered during the incident it was built for, by someone
// concluding that nothing is wrong.
//
// This test parses the dashboard, extracts every metric name it queries, and
// requires each one to exist in the registry the application actually
// exposes.
func TestEveryMetricTheDashboardQueriesExists(t *testing.T) {
	exported := exportedMetricNames(t)

	for _, name := range metricsReferencedByDashboard(t) {
		if !exported[name] {
			t.Errorf("the dashboard queries %q, which the application does not export.\nexported: %v",
				name, sortedKeys(exported))
		}
	}
}

// The reverse direction is a warning rather than a failure: a metric with no
// panel is not broken, it is unexplained. Listing them keeps the gap visible
// instead of letting the dashboard quietly fall behind the instrumentation.
func TestEveryExportedMetricIsOnTheDashboard(t *testing.T) {
	// Process and Go runtime collectors are registered by the client library
	// and are not this dashboard's job.
	ignored := regexp.MustCompile(`^(go_|process_|promhttp_)`)

	queried := map[string]bool{}
	for _, name := range metricsReferencedByDashboard(t) {
		queried[name] = true
	}

	var missing []string
	for name := range exportedMetricNames(t) {
		if ignored.MatchString(name) || queried[name] {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)

	if len(missing) > 0 {
		t.Logf("exported but not on any dashboard panel: %v", missing)
	}
}

func metricsReferencedByDashboard(t *testing.T) []string {
	t.Helper()

	raw, err := os.ReadFile("grafana/dashboards/service-overview.json")
	if err != nil {
		t.Fatalf("read dashboard: %v", err)
	}

	var dash struct {
		Panels []struct {
			Targets []struct {
				Expr string `json:"expr"`
			} `json:"targets"`
		} `json:"panels"`
	}
	if err := json.Unmarshal(raw, &dash); err != nil {
		t.Fatalf("parse dashboard: %v", err)
	}

	// Everything that can hold an identifier which is not a metric name is
	// removed before matching: quoted strings, label selectors, range
	// durations, and the label lists of by/without/on/ignoring clauses. What
	// is left and is not followed by an opening parenthesis is a metric name.
	strip := []*regexp.Regexp{
		regexp.MustCompile(`"[^"]*"`),
		regexp.MustCompile(`\{[^}]*\}`),
		regexp.MustCompile(`\[[^\]]*\]`),
		regexp.MustCompile(`\b(by|without|on|ignoring|group_left|group_right)\s*\([^)]*\)`),
	}
	identifier := regexp.MustCompile(`[a-zA-Z_:][a-zA-Z0-9_:]*\s*\(?`)
	keywords := map[string]bool{
		"sum": true, "avg": true, "min": true, "max": true, "count": true,
		"stddev": true, "stdvar": true, "topk": true, "bottomk": true,
		"quantile": true, "count_values": true, "group": true,
		"and": true, "or": true, "unless": true,
	}

	seen := map[string]bool{}
	var names []string
	for _, p := range dash.Panels {
		if len(p.Targets) == 0 {
			t.Errorf("a dashboard panel has no query at all")
		}
		for _, target := range p.Targets {
			expr := target.Expr
			for _, re := range strip {
				expr = re.ReplaceAllString(expr, " ")
			}
			for _, match := range identifier.FindAllString(expr, -1) {
				if strings.HasSuffix(strings.TrimSpace(match), "(") {
					continue
				}
				name := strings.TrimSpace(match)
				if keywords[name] || seen[name] {
					continue
				}
				seen[name] = true
				names = append(names, name)
			}
		}
	}

	if len(names) == 0 {
		t.Fatal("no metric names were extracted from the dashboard; the parser is broken")
	}
	return names
}

func exportedMetricNames(t *testing.T) map[string]bool {
	t.Helper()

	m := telemetry.NewMetrics()

	// Touch every instrument, because a CounterVec with no observed label
	// combination exports nothing and would look like a missing metric.
	m.HTTPRequest("GET", "/v1/users", 200, 0.01)
	m.HTTPRequest("GET", "/v1/users", 500, 0.01)
	m.RateLimitRejected("ip")
	m.RedisOperation("get", 0.001, nil)
	m.MessagingPublished("user.created.v1")
	m.MessagingConsumed("user.created.v1", "worker", 0.01, nil)
	m.MessagingRetry("user.created.v1", "worker")
	m.MessagingDeadLettered("user.created.v1", "worker")
	m.MessagingDuplicate("user.created.v1", "worker")
	m.UserRegistered()
	m.LoginAttempt(true)
	m.LoginAttempt(false)
	m.RegisterPoolCollector(fixedPoolStats{})
	m.MessagingOutboxPending(0)

	families, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather() = %v", err)
	}

	names := map[string]bool{}
	for _, f := range families {
		base := f.GetName()
		names[base] = true
		// Histograms are queried through their derived series.
		if f.GetType().String() == "HISTOGRAM" {
			names[base+"_bucket"] = true
			names[base+"_sum"] = true
			names[base+"_count"] = true
		}
	}
	return names
}

type fixedPoolStats struct{}

func (fixedPoolStats) TotalConns() int32    { return 4 }
func (fixedPoolStats) IdleConns() int32     { return 3 }
func (fixedPoolStats) AcquiredConns() int32 { return 1 }

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
