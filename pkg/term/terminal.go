package term

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/moby/term"
	"k8s.io/client-go/tools/remotecommand"
)

// SizeQueue implements remotecommand.TerminalSizeQueue
type SizeQueue struct {
	resizeChan chan remotecommand.TerminalSize
}

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
func (s *SizeQueue) MonitorSize() {
	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGWINCH)
	defer signal.Stop(sigChan)

	// Send initial size
	s.updateSize()

	for range sigChan {
		s.updateSize()
	}
}

func (s *SizeQueue) updateSize() {
	size, err := term.GetWinsize(os.Stdin.Fd())
	if err != nil {
		return
	}

	s.resizeChan <- remotecommand.TerminalSize{
		Width:  size.Width,
		Height: size.Height,
	}
}

// Close stops monitoring and closes the queue.
func (s *SizeQueue) Close() {
	close(s.resizeChan)
}

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
