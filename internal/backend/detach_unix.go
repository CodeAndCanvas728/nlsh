//go:build !windows

package backend

import "syscall"

// detachAttributes puts a spawned apfel in its own session so that it
// survives nlsh exiting and is not killed by a Ctrl-C in the terminal.
func detachAttributes() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
