package term

import (
	"errors"
	"testing"

	"github.com/moby/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetRawMode_TerminalSuccess(t *testing.T) {
	origIs := isTerminalFn
	origMake := makeRawFn
	origRestore := restoreTerminalFn

	defer func() {
		isTerminalFn = origIs
		makeRawFn = origMake
		restoreTerminalFn = origRestore
	}()

	state := &term.State{}

	var restored bool

	isTerminalFn = func(uintptr) bool { return true }
	makeRawFn = func(uintptr) (*term.State, error) { return state, nil }
	restoreTerminalFn = func(uintptr, *term.State) error {
		restored = true

		return nil
	}

	restore, err := SetRawMode()
	require.NoError(t, err)
	require.NotNil(t, restore)
	restore()
	assert.True(t, restored)
}

func TestSetRawMode_MakeRawError(t *testing.T) {
	origIs := isTerminalFn
	origMake := makeRawFn

	defer func() {
		isTerminalFn = origIs
		makeRawFn = origMake
	}()

	isTerminalFn = func(uintptr) bool { return true }
	makeRawFn = func(uintptr) (*term.State, error) {
		return nil, errors.New("make raw fail")
	}

	_, err := SetRawMode()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "make raw fail")
}
