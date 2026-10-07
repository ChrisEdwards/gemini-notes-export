//go:build unix

package syncer

import (
	"errors"
	"os"
	"syscall"
)

// pidAlive reports whether pid is a running process. known is always true on Unix.
var alive = pidAlive

func pidAlive(pid int) (live, known bool) {
	if pid <= 0 {
		return false, true
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false, true
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM), true
}
