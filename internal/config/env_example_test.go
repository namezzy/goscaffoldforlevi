package config_test

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/example/go-enterprise/internal/config"
)

// TestEnvExampleMatchesTheLoader keeps documentation and code from drifting.
//
// An .env.example that lists a key the loader ignores sends people chasing a
// setting that does nothing; one that omits a key the loader reads leaves a
// setting undiscoverable. Both are the kind of small, constant friction that
// nobody ever files a bug about, so it has to be enforced mechanically.
func TestEnvExampleMatchesTheLoader(t *testing.T) {
	loaderKeys := keysReadByLoader(t)
	documentedKeys := keysInEnvExample(t)

	for _, k := range loaderKeys {
		if !contains(documentedKeys, k) {
			t.Errorf("%s is read by the loader but missing from .env.example", k)
		}
	}
	for _, k := range documentedKeys {
		if !contains(loaderKeys, k) {
			t.Errorf("%s is listed in .env.example but never read by the loader", k)
		}
	}
}

var loaderCall = regexp.MustCompile(`p\.\w+\("([A-Z][A-Z0-9_]*)"`)

func keysReadByLoader(t *testing.T) []string {
	t.Helper()

	src, err := os.ReadFile("load.go")
	if err != nil {
		t.Fatalf("read load.go: %v", err)
	}

	var keys []string
	for _, m := range loaderCall.FindAllStringSubmatch(string(src), -1) {
		keys = append(keys, m[1])
	}
	if len(keys) == 0 {
		t.Fatal("found no configuration keys in load.go; the pattern is stale")
	}

	sort.Strings(keys)
	return keys
}

func keysInEnvExample(t *testing.T) []string {
	t.Helper()

	src, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}

	var keys []string
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, _, ok := strings.Cut(line, "=")
		if ok {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)
	return keys
}

// TestEnvExampleJWTSecretIsRecognisedAsInsecure ties the two halves of the
// safety check together. If the placeholder in .env.example were ever changed
// without updating InsecureJWTSecrets, the production guard would pass on a
// value that is published in the repository.
func TestEnvExampleJWTSecretIsRecognisedAsInsecure(t *testing.T) {
	src, err := os.ReadFile("../../.env.example")
	if err != nil {
		t.Fatalf("read .env.example: %v", err)
	}

	var placeholder string
	for _, line := range strings.Split(string(src), "\n") {
		if after, found := strings.CutPrefix(strings.TrimSpace(line), "AUTH_JWT_SECRET="); found {
			placeholder = after
			break
		}
	}
	if placeholder == "" {
		t.Fatal("AUTH_JWT_SECRET is not set in .env.example")
	}

	if !containsInsecure(placeholder) {
		t.Fatalf("the .env.example JWT secret %q is not in config.InsecureJWTSecrets, "+
			"so deploying it to production would pass validation", placeholder)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func containsInsecure(v string) bool {
	return contains(config.InsecureJWTSecrets, v)
}
