//go:build windows

package backend

import "syscall"

// detachAttributes puts a spawned apfel in its own process group so that it
// survives nlsh exiting and is not killed by a Ctrl-C in the console.
func detachAttributes() *syscall.SysProcAttr {
	const (
		detachedProcess       = 0x00000008
		createNewProcessGroup = 0x00000200
	)
	return &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
}
