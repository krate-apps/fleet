package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateExecPathRejectsShell(t *testing.T) {
	SetProcessExecPolicy(nil, nil)
	if _, err := validateExecPath("bin/echo"); err == nil {
		t.Fatal("relative should fail")
	}
	if _, err := os.Stat("/bin/sh"); err == nil {
		SetProcessExecPolicy([]string{"/bin/sh"}, nil)
		if _, err := validateExecPath("/bin/sh"); err == nil {
			t.Fatal("shell should be rejected even when listed")
		}
	}
}

func TestValidateExecPathRequiresAllowlist(t *testing.T) {
	echo := ""
	for _, c := range []string{"/bin/echo", "/usr/bin/echo"} {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			echo = c
			break
		}
	}
	if echo == "" {
		t.Skip("no echo binary")
	}
	SetProcessExecPolicy(nil, nil)
	if _, err := validateExecPath(echo); err == nil {
		t.Fatal("empty allowlist must deny")
	}
	SetProcessExecPolicy([]string{echo}, nil)
	got, err := validateExecPath(echo)
	if err != nil {
		t.Fatal(err)
	}
	if got == "" {
		t.Fatal("expected resolved path")
	}
}

func TestRunAllowlistedExecEcho(t *testing.T) {
	echo := ""
	for _, c := range []string{"/bin/echo", "/usr/bin/echo"} {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			echo = c
			break
		}
	}
	if echo == "" {
		t.Skip("no echo binary")
	}
	SetProcessExecPolicy([]string{echo}, nil)
	res, err := runAllowlistedExec(context.Background(), echo, []string{"fleet-hooks"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true {
		t.Fatalf("%#v", res)
	}
	if got := res["stdout"].(string); got != "fleet-hooks\n" && got != "fleet-hooks" {
		if filepath.Base(echo) == "echo" && len(got) == 0 {
			t.Fatalf("stdout=%q", got)
		}
	}
}
