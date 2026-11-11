package main

import (
	"testing"

	"asterlab-mygo-prototype/modules/runs"
)

func TestRestoreTaskKeyPrefersKeyAndRequiresUniquePath(t *testing.T) {
	tasks := []runs.RunTask{
		{Key: "a", Path: "root/step"},
		{Key: "b", Path: "root/other"},
	}
	if got := restoreTaskKey(tasks, "b", "root/step"); got != "b" {
		t.Fatalf("task key was not preferred: %q", got)
	}
	if got := restoreTaskKey(tasks, "missing", "root/step"); got != "a" {
		t.Fatalf("unique path fallback = %q", got)
	}
	if got := restoreTaskKey(tasks, "missing", "root/missing"); got != "" {
		t.Fatalf("missing path selected %q", got)
	}
	tasks = append(tasks, runs.RunTask{Key: "c", Path: "root/step"})
	if got := restoreTaskKey(tasks, "missing", "root/step"); got != "" {
		t.Fatalf("ambiguous path selected %q", got)
	}
}
