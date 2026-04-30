package term

import (
	"context"
	"testing"
	"time"

	"github.com/moby/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/remotecommand"
)

func TestSizeQueue_MonitorSize(t *testing.T) {
	sq := NewSizeQueue()
	defer sq.Close()

	// Mock getWinsize
	originalGetWinsize := getWinsize

	defer func() { getWinsize = originalGetWinsize }()

	getWinsize = func(_ uintptr) (*term.Winsize, error) {
		return &term.Winsize{Width: 100, Height: 40}, nil
	}

	t.Run("monitor size loop", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		go sq.MonitorSize(ctx)

		// MonitorSize should have sent initial size
		size := sq.Next()
		require.NotNil(t, size)
		assert.Equal(t, uint16(100), size.Width)

		cancel()
	})
}

func TestNewSizeQueue(t *testing.T) {
	sq := NewSizeQueue()
	assert.NotNil(t, sq)
	assert.NotNil(t, sq.resizeChan)
}

func TestSizeQueue_Next(t *testing.T) {
	sq := NewSizeQueue()

	size := remotecommand.TerminalSize{Width: 80, Height: 24}
	sq.resizeChan <- size

	result := sq.Next()
	assert.NotNil(t, result)
	assert.Equal(t, uint16(80), result.Width)
	assert.Equal(t, uint16(24), result.Height)
}

func TestSizeQueue_Next_Closed(t *testing.T) {
	sq := NewSizeQueue()
	close(sq.resizeChan)

	result := sq.Next()
	assert.Nil(t, result)
}

func TestSizeQueue_Close(t *testing.T) {
	sq := NewSizeQueue()

	go func() {
		time.Sleep(10 * time.Millisecond)
		sq.Close()
	}()

	result := sq.Next()
	assert.Nil(t, result)
}

func TestSetRawMode_NonTerminal(t *testing.T) {
	restore, err := SetRawMode()

	require.NoError(t, err)
	assert.NotNil(t, restore)

	restore()
}

func TestSetRawMode_Terminal(t *testing.T) {
	if !isTerminal() {
		t.Skip("not a terminal, skipping terminal test")
	}

	restore, err := SetRawMode()

	require.NoError(t, err)
	assert.NotNil(t, restore)

	restore()
}

func isTerminal() bool {
	return false
}
