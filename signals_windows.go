//go:build windows

package udsrpc

import (
	"os"
	"os/signal"
)

// HandleSignals on Windows installs handlers only for os.Interrupt
// (Ctrl+C). SIGHUP-style reload is not available; onReload is ignored.
// Returns a stop function that detaches the handler.
func HandleSignals(onShutdown, onReload func()) (stop func()) {
	_ = onReload // unsupported on Windows
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	done := make(chan struct{})
	go func() {
		select {
		case <-ch:
			if onShutdown != nil {
				onShutdown()
			}
			signal.Stop(ch)
		case <-done:
			signal.Stop(ch)
		}
	}()

	return func() { close(done) }
}
