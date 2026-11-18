package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadStatusMissingPidFile(t *testing.T) {
	dir := t.TempDir()
	st, err := ReadStatus(dir)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if st.Running || st.PID != 0 {
		t.Fatalf("status = %+v", st)
	}
}

func TestWriteAndReadAlivePID(t *testing.T) {
	dir := t.TempDir()
	pid := os.Getpid()
	if err := WritePIDFile(dir, pid); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, pidFileName)); err != nil {
		t.Fatalf("pid file: %v", err)
	}
	st, err := ReadStatus(dir)
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if !st.Running || st.PID != pid {
		t.Fatalf("status = %+v, want pid %d running", st, pid)
	}
}
