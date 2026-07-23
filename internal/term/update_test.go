package term

import (
	"context"
	"errors"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/moby/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateSize_GetWinsizeError(t *testing.T) {
	sq := NewSizeQueue()
	defer sq.Close()

	orig := getWinsize

	defer func() { getWinsize = orig }()

	getWinsize = func(uintptr) (*term.Winsize, error) {
		return nil, errors.New("no tty")
	}

	sq.updateSize(t.Context())

	select {
	case <-sq.resizeChan:
		t.Fatal("unexpected size sent on winsize error")
	default:
	}
}

func TestUpdateSize_LatestWinsWhenFull(t *testing.T) {
	sq := NewSizeQueue()
	defer sq.Close()

	orig := getWinsize

	defer func() { getWinsize = orig }()

	var width uint16 = 10

	getWinsize = func(uintptr) (*term.Winsize, error) {
		return &term.Winsize{Width: width, Height: 20}, nil
	}

	sq.updateSize(t.Context())

	width = 99

	sq.updateSize(t.Context())

	size := sq.Next()
	require.NotNil(t, size)
	assert.Equal(t, uint16(99), size.Width)
}

func TestMonitorSize_SIGWINCH(t *testing.T) {
	sq := NewSizeQueue()
	defer sq.Close()

	orig := getWinsize

	defer func() { getWinsize = orig }()

	var width atomic.Uint32

	width.Store(80)

	getWinsize = func(uintptr) (*term.Winsize, error) {
		return &term.Winsize{Width: uint16(width.Load()), Height: 24}, nil
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	go sq.MonitorSize(ctx)

	first := sq.Next()
	require.NotNil(t, first)
	assert.Equal(t, uint16(80), first.Width)

	width.Store(120)

	require.NoError(t, syscall.Kill(syscall.Getpid(), syscall.SIGWINCH))

	deadline := time.After(2 * time.Second)

	for {
		select {
		case size := <-sq.resizeChan:
			if size.Width == 120 {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for SIGWINCH resize")
		}
	}
}
