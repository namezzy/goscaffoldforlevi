// Package k8s_test guards the deployment manifests.
//
// It exists for the same reason as deployments/observability/dashboard_test.go
// and api/bruno/collection_test.go: none of the mistakes it catches announce
// themselves. They present as "the service is flaky", "the metrics never
// arrived", "pods keep restarting" — symptoms that send people to look at the
// application, which is the one place the cause is not.
//
// It reads the base YAML rather than the output of `kustomize build`, for two
// reasons. Shelling out to kubectl would make the test depend on a binary that
// need not exist on a CI runner, and every constraint asserted here is defined
// in the base and never overridden by an overlay. If an overlay ever starts
// overriding one, extend this test to cover that overlay — do not switch it to
// invoking kustomize, which trades a precise failure for a vague one.
package k8s_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const baseDir = "base"

// TestGracePeriodOutlastsTheShutdownBudget is the assertion most likely to be
// broken by a well-meaning edit.
//
// The application takes up to HTTP_SHUTDOWN_TIMEOUT to finish in-flight
// requests. If terminationGracePeriodSeconds is not comfortably larger, the
// kubelet sends SIGKILL while requests are still being served, and those
// requests become 502s. They appear only during a deploy, only under load, and
// never locally — the worst possible combination to debug from.
func TestGracePeriodOutlastsTheShutdownBudget(t *testing.T) {
	shutdown := shutdownTimeout(t)

	for _, name := range []string{"api-deployment.yaml", "worker-deployment.yaml"} {
		doc := loadYAML(t, name)
		grace := time.Duration(intAt(t, name, doc, "spec", "template", "spec", "terminationGracePeriodSeconds")) * time.Second

		if grace <= shutdown {
			t.Errorf("%s: terminationGracePeriodSeconds is %s, which does not exceed HTTP_SHUTDOWN_TIMEOUT (%s) from configmap.yaml.\n"+
				"The pod would be SIGKILLed while it is still draining.", name, grace, shutdown)
			continue
		}

		// A margin of "just over" is the subtle version of the same bug.
		// SIGTERM is not delivered the instant the kubelet decides to send it,
		// and under load that scheduling delay eats a thin margin.
		if margin := grace - shutdown; margin < 10*time.Second {
			t.Errorf("%s: terminationGracePeriodSeconds (%s) exceeds HTTP_SHUTDOWN_TIMEOUT (%s) by only %s.\n"+
				"Leave at least 10s for SIGTERM delivery and scheduling delay.", name, grace, shutdown, margin)
		}
	}
}

// TestProbesPointAtPathsTheServiceServes catches a probe aimed at a path that
// does not exist. The kubelet gets a 404, reads it as a failure, and restarts
// the pod — over and over. The symptom is a CrashLoopBackOff on a process that
// is working perfectly.
func TestProbesPointAtPathsTheServiceServes(t *testing.T) {
	served := pathsInOpenAPI(t)

	doc := loadYAML(t, "api-deployment.yaml")
	container := containerAt(t, "api-deployment.yaml", doc, 0)

	for _, probe := range []struct {
		field string
		want  string
	}{
		// /live must not consult a dependency. If it did, a few seconds of
		// database failover would fail liveness on every replica at once: all
		// of them killed together, all of them cold-starting together, hitting
		// the database exactly as it recovers. Readiness is where dependency
		// health belongs, because its consequence is "stop routing here"
		// rather than "destroy the pod".
		{"livenessProbe", "/live"},
		{"startupProbe", "/live"},
		{"readinessProbe", "/ready"},
	} {
		p, ok := container[probe.field].(map[string]any)
		if !ok {
			t.Errorf("api-deployment.yaml: container has no %s", probe.field)
			continue
		}
		httpGet, ok := p["httpGet"].(map[string]any)
		if !ok {
			t.Errorf("api-deployment.yaml: %s is not an httpGet probe", probe.field)
			continue
		}
		got, _ := httpGet["path"].(string)
		if got != probe.want {
			t.Errorf("api-deployment.yaml: %s probes %q, want %q", probe.field, got, probe.want)
		}
		if !served[got] {
			t.Errorf("api-deployment.yaml: %s probes %q, which api/openapi.yaml does not document.\n"+
				"A probe against an unserved path returns 404 and restarts a healthy pod.", probe.field, got)
		}
	}
}

// TestMetricsAddressIsNotLoopback catches the deployment mistake that produces
// no error at all.
//
// METRICS_ADDR defaults to 127.0.0.1:9464, which is right on a laptop and
// useless in a pod: Prometheus scrapes from somewhere else, and a loopback
// bind is reachable by nothing outside the container. Nothing logs a warning.
// The metrics simply never appear, and the natural conclusion is that the
// instrumentation was never wired up.
func TestMetricsAddressIsNotLoopback(t *testing.T) {
	data := configMapData(t)

	addr, ok := data["METRICS_ADDR"]
	if !ok {
		t.Fatal("configmap.yaml does not set METRICS_ADDR; the application default binds loopback")
	}
	for _, bad := range []string{"127.0.0.1", "localhost", "::1"} {
		if strings.Contains(addr, bad) {
			t.Errorf("configmap.yaml sets METRICS_ADDR=%q. In a pod this is reachable by nothing, "+
				"including Prometheus, and it fails by producing no data rather than an error.", addr)
		}
	}
}

// TestWorkloadsAreHardened checks the four settings that remove capability
// rather than add it. Losing one costs nothing observable: the workload keeps
// running, with more privilege than it needs, and no symptom ever points here.
func TestWorkloadsAreHardened(t *testing.T) {
	files := map[string][]string{
		"api-deployment.yaml":    {"spec", "template", "spec"},
		"worker-deployment.yaml": {"spec", "template", "spec"},
		"migrate-job.yaml":       {"spec", "template", "spec"},
	}

	for name, path := range files {
		doc := loadYAML(t, name)
		podSpec := mapAt(t, name, doc, path...)

		pod, ok := podSpec["securityContext"].(map[string]any)
		if !ok {
			t.Errorf("%s: the pod has no securityContext", name)
			continue
		}
		if pod["runAsNonRoot"] != true {
			t.Errorf("%s: pod securityContext.runAsNonRoot is not true", name)
		}
		if seccomp, ok := pod["seccompProfile"].(map[string]any); !ok || seccomp["type"] != "RuntimeDefault" {
			t.Errorf("%s: pod securityContext.seccompProfile.type is not RuntimeDefault", name)
		}

		container := containerAt(t, name, doc, 0)
		sc, ok := container["securityContext"].(map[string]any)
		if !ok {
			t.Errorf("%s: the container has no securityContext", name)
			continue
		}
		if sc["allowPrivilegeEscalation"] != false {
			t.Errorf("%s: container securityContext.allowPrivilegeEscalation is not false", name)
		}
		// Possible only because the process writes nothing to disk: logs go to
		// stdout, and there is no local cache or temporary file. If that ever
		// changes, mount an emptyDir rather than relaxing this.
		if sc["readOnlyRootFilesystem"] != true {
			t.Errorf("%s: container securityContext.readOnlyRootFilesystem is not true", name)
		}
		caps, ok := sc["capabilities"].(map[string]any)
		if !ok {
			t.Errorf("%s: container securityContext has no capabilities block", name)
			continue
		}
		dropped, _ := caps["drop"].([]any)
		if len(dropped) != 1 || dropped[0] != "ALL" {
			t.Errorf("%s: container capabilities.drop is %v, want [ALL]", name, dropped)
		}
	}
}

// TestNoManifestReferencesAMovingTag enforces decision D6.
//
// A tag like `v1` or `latest` makes the same manifest deploy different code on
// different days, and the change leaves no trace: git shows nothing, because
// nothing in git changed. Only immutable references belong in an overlay.
func TestNoManifestReferencesAMovingTag(t *testing.T) {
	moving := map[string]bool{"latest": true, "main": true, "master": true, "edge": true, "stable": true}

	overlays, err := filepath.Glob(filepath.Join("overlays", "*", "kustomization.yaml"))
	if err != nil || len(overlays) == 0 {
		t.Fatalf("no overlays found: %v", err)
	}

	for _, path := range overlays {
		// The local overlay builds its images on the developer's machine and
		// never pulls, so registry tag discipline does not apply to it.
		if strings.Contains(path, string(filepath.Separator)+"local"+string(filepath.Separator)) {
			continue
		}

		raw, err := os.ReadFile(path) //nolint:gosec // a test reading a fixed path in its own package
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var doc struct {
			Images []struct {
				Name   string `yaml:"name"`
				NewTag string `yaml:"newTag"`
			} `yaml:"images"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if len(doc.Images) == 0 {
			t.Errorf("%s: pins no images, so it would deploy the base placeholder tag", path)
		}
		for _, img := range doc.Images {
			if moving[img.NewTag] {
				t.Errorf("%s: %s is pinned to the moving tag %q. Use sha-<commit> or an exact release.",
					path, img.Name, img.NewTag)
			}
		}
	}
}

// ---------------------------------------------------------------- helpers

func loadYAML(t *testing.T, name string) map[string]any {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(baseDir, name)) //nolint:gosec // a test reading a fixed path in its own package
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return doc
}

func mapAt(t *testing.T, name string, doc map[string]any, path ...string) map[string]any {
	t.Helper()

	current := doc
	for i, key := range path {
		next, ok := current[key].(map[string]any)
		if !ok {
			t.Fatalf("%s: %s is missing or not a mapping", name, strings.Join(path[:i+1], "."))
		}
		current = next
	}
	return current
}

func intAt(t *testing.T, name string, doc map[string]any, path ...string) int {
	t.Helper()

	parent := mapAt(t, name, doc, path[:len(path)-1]...)
	value, ok := parent[path[len(path)-1]].(int)
	if !ok {
		t.Fatalf("%s: %s is missing or not an integer", name, strings.Join(path, "."))
	}
	return value
}

func containerAt(t *testing.T, name string, doc map[string]any, index int) map[string]any {
	t.Helper()

	podSpec := mapAt(t, name, doc, "spec", "template", "spec")
	containers, ok := podSpec["containers"].([]any)
	if !ok || len(containers) <= index {
		t.Fatalf("%s: has no container at index %d", name, index)
	}
	container, ok := containers[index].(map[string]any)
	if !ok {
		t.Fatalf("%s: container %d is not a mapping", name, index)
	}
	return container
}

func configMapData(t *testing.T) map[string]string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(baseDir, "configmap.yaml"))
	if err != nil {
		t.Fatalf("read configmap.yaml: %v", err)
	}
	var doc struct {
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse configmap.yaml: %v", err)
	}
	return doc.Data
}

func shutdownTimeout(t *testing.T) time.Duration {
	t.Helper()

	raw, ok := configMapData(t)["HTTP_SHUTDOWN_TIMEOUT"]
	if !ok {
		t.Fatal("configmap.yaml does not set HTTP_SHUTDOWN_TIMEOUT")
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		t.Fatalf("configmap.yaml: HTTP_SHUTDOWN_TIMEOUT=%q is not a duration: %v", raw, err)
	}
	return d
}

// pathsInOpenAPI reads the probe paths the service actually documents. The
// route-level test in cmd/api already proves the document matches the router,
// so agreeing with the document is the same as agreeing with the service.
func pathsInOpenAPI(t *testing.T) map[string]bool {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read api/openapi.yaml: %v", err)
	}
	var doc struct {
		Paths map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse api/openapi.yaml: %v", err)
	}

	paths := make(map[string]bool, len(doc.Paths))
	for p := range doc.Paths {
		paths[p] = true
	}
	return paths
}
