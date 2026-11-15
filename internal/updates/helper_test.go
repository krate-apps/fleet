package updates

import (
	"strings"
	"testing"
)

func TestHelperCommand(t *testing.T) {
	got := HelperCommand("/opt/alice/fleet", "v1.2.3", "http://127.0.0.1:8080/api/v1/health")
	if !strings.Contains(got, "ln -sfn") {
		t.Fatalf("expected ln -sfn in %q", got)
	}
	if !strings.Contains(got, "'releases/v1.2.3'") {
		t.Fatalf("expected quoted release path in %q", got)
	}
	if !strings.Contains(got, "'/opt/alice/fleet/current'") {
		t.Fatalf("expected current path in %q", got)
	}
	if strings.Contains(got, "fleet-switch-release") {
		t.Fatalf("legacy helper must not appear: %q", got)
	}
	if !strings.Contains(got, "restart") {
		t.Fatalf("expected restart hint in %q", got)
	}
}

func TestHelperCommandEmpty(t *testing.T) {
	got := HelperCommand("", "", "")
	if !strings.Contains(got, "releases/<version>") {
		t.Fatalf("placeholder form expected: %q", got)
	}
}

func TestShellQuote(t *testing.T) {
	if shellQuote("") != "''" {
		t.Fatalf("empty: %q", shellQuote(""))
	}
	if shellQuote("a'b") != `'a'\''b'` {
		t.Fatalf("quote escape: %q", shellQuote("a'b"))
	}
}
