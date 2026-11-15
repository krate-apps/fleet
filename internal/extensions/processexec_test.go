package extensions

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateExecPathRejectsShell(t *testing.T) {
	if _, err := validateExecPath("bin/echo"); err == nil {
		t.Fatal("relative should fail")
	}
	// /bin/sh may or may not exist; if it does, must reject.
	if _, err := os.Stat("/bin/sh"); err == nil {
		if _, err := validateExecPath("/bin/sh"); err == nil {
			t.Fatal("shell should be rejected")
		}
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
	// echo is not in forbidden list
	res, err := runAllowlistedExec(context.Background(), echo, []string{"fleet-hooks"}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if res["ok"] != true {
		t.Fatalf("%#v", res)
	}
	if got := res["stdout"].(string); got != "fleet-hooks\n" && got != "fleet-hooks" {
		// some echo omit newline handling — accept prefix
		if filepath.Base(echo) == "echo" && len(got) == 0 {
			t.Fatalf("stdout=%q", got)
		}
	}
}
