//go:build !unix

package syncer

// alive can't check liveness without Signal(0); acquireLock falls back to lock age.
// ponytail: age-based staleness, a crashed sync blocks for up to an hour; use
// OpenProcess via x/sys/windows if that bites.
var alive = func(int) (live, known bool) { return false, false }
