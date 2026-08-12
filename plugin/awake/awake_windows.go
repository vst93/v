//go:build windows

package plugin_awake

import (
	"fmt"
	"runtime"
	"syscall"
)

const (
	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
)

func startPreventSleep() (func() error, error) {
	type result struct{ err error }
	ready := make(chan result, 1)
	release := make(chan chan result)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		kernel32 := syscall.NewLazyDLL("kernel32.dll")
		setThreadExecutionState := kernel32.NewProc("SetThreadExecutionState")
		ret, _, callErr := setThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired | esDisplayRequired))
		if ret == 0 {
			if callErr != syscall.Errno(0) {
				ready <- result{fmt.Errorf("failed to prevent sleep with SetThreadExecutionState: %w", callErr)}
				return
			}
			ready <- result{fmt.Errorf("failed to prevent sleep with SetThreadExecutionState")}
			return
		}
		ready <- result{}

		done := <-release
		ret, _, callErr = setThreadExecutionState.Call(uintptr(esContinuous))
		if ret == 0 && callErr != syscall.Errno(0) {
			done <- result{fmt.Errorf("failed to restore sleep policy: %w", callErr)}
		} else if ret == 0 {
			done <- result{fmt.Errorf("failed to restore sleep policy")}
		} else {
			done <- result{}
		}
	}()

	if initialized := <-ready; initialized.err != nil {
		return nil, initialized.err
	}
	return func() error {
		done := make(chan result, 1)
		release <- done
		return (<-done).err
	}, nil
}
