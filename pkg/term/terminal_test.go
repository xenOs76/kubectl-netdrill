package term

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/remotecommand"
)

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
