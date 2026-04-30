package term

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/term"
	"k8s.io/client-go/tools/remotecommand"
)

// getWinsize is a function variable for term.GetWinsize to allow mocking in tests.
var getWinsize = term.GetWinsize

// SizeQueue implements remotecommand.TerminalSizeQueue
type SizeQueue struct {
	resizeChan chan remotecommand.TerminalSize
}

// NewSizeQueue creates a new SizeQueue.
func NewSizeQueue() *SizeQueue {
	return &SizeQueue{
		resizeChan: make(chan remotecommand.TerminalSize, 1),
	}
}

// Next returns the next terminal size from the queue.
func (s *SizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-s.resizeChan
	if !ok {
		return nil
	}

	return &size
}

// MonitorSize watches for SIGWINCH signals and updates the size queue.
func (s *SizeQueue) MonitorSize(ctx context.Context) {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGWINCH)
	defer signal.Stop(sigChan)

	// Send initial size
	s.updateSize(ctx)

	for {
		select {
		case <-sigChan:
			s.updateSize(ctx)
		case <-ctx.Done():
			return
		}
	}
}

// updateSize gets the current terminal size and sends it to the resize channel.
// It is non-blocking and preserves "latest" semantics if the channel is full.
func (s *SizeQueue) updateSize(ctx context.Context) {
	size, err := getWinsize(os.Stdin.Fd())
	if err != nil {
		return
	}

	ts := remotecommand.TerminalSize{
		Width:  size.Width,
		Height: size.Height,
	}

	select {
	case s.resizeChan <- ts:
	case <-ctx.Done():
	default:
		// Channel is full, consume old value to preserve latest semantics
		select {
		case <-s.resizeChan:
		default:
		}
		// Now send the latest size
		select {
		case s.resizeChan <- ts:
		case <-ctx.Done():
		}
	}
}

// Close stops monitoring and closes the queue.
func (s *SizeQueue) Close() {
	close(s.resizeChan)
}

// RawModeSetter is a function variable for SetRawMode to allow mocking in tests.
var RawModeSetter = SetRawMode

// SetRawMode puts the terminal in raw mode and returns a function to restore it.
func SetRawMode() (func(), error) {
	stdInFd := os.Stdin.Fd()
	if !term.IsTerminal(stdInFd) {
		return func() {}, nil
	}

	state, err := term.MakeRaw(stdInFd)
	if err != nil {
		return nil, err
	}

	return func() {
		_ = term.RestoreTerminal(stdInFd, state)
	}, nil
}
