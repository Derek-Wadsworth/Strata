package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const pidFileName = "strata.pid"

// PidPath is the PID file location inside dataDir.
func PidPath(dataDir string) string {
	return filepath.Join(dataDir, pidFileName)
}

// Status describes whether a daemon appears to be running.
type Status struct {
	Running bool
	PID     int
	PidFile string
}

// ReadStatus inspects the PID file and checks if that process is alive.
func ReadStatus(dataDir string) (Status, error) {
	path := PidPath(dataDir)
	st := Status{PidFile: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return st, nil
		}
		return st, fmt.Errorf("read pid file: %w", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		_ = os.Remove(path)
		return st, nil
	}
	st.PID = pid
	if processAlive(pid) {
		st.Running = true
	} else {
		_ = os.Remove(path)
		st.PID = 0
	}
	return st, nil
}

// Start spawns a detached `strata daemon` process and writes its PID file.
func Start(dataDir string) (Status, error) {
	st, err := ReadStatus(dataDir)
	if err != nil {
		return st, err
	}
	if st.Running {
		return st, fmt.Errorf("daemon already running (pid %d)", st.PID)
	}

	exe, err := os.Executable()
	if err != nil {
		return st, fmt.Errorf("resolve executable: %w", err)
	}

	cmd := exec.Command(exe, "daemon")
	configureDetached(cmd)
	// Daemon opens its own log file; keep parent stdio disconnected.
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil

	if err := cmd.Start(); err != nil {
		return st, fmt.Errorf("start daemon: %w", err)
	}
	// Release the child so the parent can exit without waiting.
	_ = cmd.Process.Release()

	if err := writePID(dataDir, cmd.Process.Pid); err != nil {
		_ = killProcess(cmd.Process.Pid)
		return st, err
	}
	return Status{Running: true, PID: cmd.Process.Pid, PidFile: PidPath(dataDir)}, nil
}

// Stop terminates a running daemon and removes the PID file.
func Stop(dataDir string) error {
	st, err := ReadStatus(dataDir)
	if err != nil {
		return err
	}
	if !st.Running {
		return fmt.Errorf("daemon is not running")
	}
	if err := killProcess(st.PID); err != nil {
		return fmt.Errorf("stop pid %d: %w", st.PID, err)
	}
	_ = os.Remove(PidPath(dataDir))
	return nil
}

// WritePIDFile writes pid for an already-running daemon process (used by daemon itself).
func WritePIDFile(dataDir string, pid int) error {
	return writePID(dataDir, pid)
}

func writePID(dataDir string, pid int) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	path := PidPath(dataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.Itoa(pid)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	return os.Rename(tmp, path)
}
