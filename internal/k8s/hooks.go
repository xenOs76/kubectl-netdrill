package k8s

import "sync"

// hookMu serializes reads and writes of package-level hook variables used in tests.
var hookMu sync.Mutex

// LockTestHooks serializes mutation of package-level hook variables. Call UnlockTestHooks
// after patching; do not hold the lock while invoking code under test.
func LockTestHooks() {
	hookMu.Lock()
}

// UnlockTestHooks releases LockTestHooks.
func UnlockTestHooks() {
	hookMu.Unlock()
}
