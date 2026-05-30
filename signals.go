//go:build !windows

package udsrpc

import (
	"os"
	"os/signal"
	"syscall"
)

// HandleSignals installs a SIGTERM/SIGINT/SIGHUP handler in a new
// goroutine and returns a stop function that cancels the handler.
//
//   - SIGTERM, SIGINT → onShutdown is called once, the handler returns.
//   - SIGHUP          → onReload is called and the handler keeps running.
//
// Either callback may be nil. Calling the returned stop function
// detaches the signal handler — it does NOT invoke onShutdown.
func HandleSignals(onShutdown, onReload func()) (stop func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	done := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-ch:
				switch sig {
				case syscall.SIGTERM, syscall.SIGINT:
					if onShutdown != nil {
						onShutdown()
					}
					signal.Stop(ch)
					return
				case syscall.SIGHUP:
					if onReload != nil {
						onReload()
					}
				}
			case <-done:
				signal.Stop(ch)
				return
			}
		}
	}()

	return func() { close(done) }
}
