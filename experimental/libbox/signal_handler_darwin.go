//go:build darwin && badlinkname

package libbox

/*
#include <signal.h>
#include <stdint.h>
#include <string.h>

static struct sigaction _go_sa[32];
static struct sigaction _plcrash_sa[32];
static int _saved = 0;

static int _signals[] = {SIGSEGV, SIGBUS, SIGFPE, SIGILL, SIGTRAP};
static const int _signal_count = sizeof(_signals) / sizeof(_signals[0]);

static void _save_go_handlers(void) {
	if (_saved) return;
	for (int i = 0; i < _signal_count; i++)
		sigaction(_signals[i], NULL, &_go_sa[_signals[i]]);
	_saved = 1;
}

static void _combined_handler(int sig, siginfo_t *info, void *uap) {
	// Step 1: PLCrashReporter writes .plcrash, resets all handlers to SIG_DFL,
	// and calls raise(sig) which pends (signal is blocked, no SA_NODEFER).
	if ((_plcrash_sa[sig].sa_flags & SA_SIGINFO) &&
		(uintptr_t)_plcrash_sa[sig].sa_sigaction > 1)
		_plcrash_sa[sig].sa_sigaction(sig, info, uap);

	// SIGTRAP does not rely on sigreturn -> sigpanic. Once Go's trap trampoline
	// is force-installed, we can chain into it directly after PLCrashReporter.
	if (sig == SIGTRAP &&
		(_go_sa[sig].sa_flags & SA_SIGINFO) &&
		(uintptr_t)_go_sa[sig].sa_sigaction > 1) {
		_go_sa[sig].sa_sigaction(sig, info, uap);
		return;
	}

	// Step 2: Restore Go's handler via sigaction (overwrites PLCrashReporter's SIG_DFL).
	// Do NOT call Go's handler directly — Go's preparePanic only modifies the
	// ucontext and returns. The actual crash output is written by sigpanic, which
	// only runs when the KERNEL restores the modified ucontext via sigreturn.
	// A direct C function call has no sigreturn, so sigpanic would never execute.
	sigaction(sig, &_go_sa[sig], NULL);

	// Step 3: Return. The kernel restores the original ucontext and re-executes
	// the faulting instruction. Two signals are now pending/imminent:
	//   a) PLCrashReporter's raise() (SI_USER) — Go's handler ignores it
	//      (sighandler: sigFromUser() → return).
	//   b) The re-executed fault (SEGV_MAPERR) — Go's handler processes it:
	//      preparePanic → kernel sigreturn → sigpanic → crash output written
	//      via debug.SetCrashOutput.
}

static void _reinstall_handlers(void) {
	if (!_saved) return;
	for (int i = 0; i < _signal_count; i++) {
		int sig = _signals[i];
		struct sigaction current;
		sigaction(sig, NULL, &current);
		// Only save the handler if it's not one of ours
		if (current.sa_sigaction != _combined_handler) {
			// If current handler is still Go's, PLCrashReporter wasn't installed
			if ((current.sa_flags & SA_SIGINFO) &&
				(uintptr_t)current.sa_sigaction > 1 &&
				current.sa_sigaction == _go_sa[sig].sa_sigaction)
				memset(&_plcrash_sa[sig], 0, sizeof(_plcrash_sa[sig]));
			else
				_plcrash_sa[sig] = current;
		}
		struct sigaction sa;
		memset(&sa, 0, sizeof(sa));
		sa.sa_sigaction = _combined_handler;
		sa.sa_flags = SA_SIGINFO | SA_ONSTACK;
		sigemptyset(&sa.sa_mask);
		sigaction(sig, &sa, NULL);
	}
}
*/
import "C"

import (
	"reflect"
	_ "unsafe"
)

const (
	_sigtrap = 5
	_nsig    = 32
)

//go:linkname runtimeGetsig runtime.getsig
func runtimeGetsig(i uint32) uintptr

//go:linkname runtimeSetsig runtime.setsig
func runtimeSetsig(i uint32, fn uintptr)

//go:linkname runtimeCgoSigtramp runtime.cgoSigtramp
func runtimeCgoSigtramp()

//go:linkname runtimeFwdSig runtime.fwdSig
var runtimeFwdSig [_nsig]uintptr

//go:linkname runtimeHandlingSig runtime.handlingSig
var runtimeHandlingSig [_nsig]uint32

func forceGoSIGTRAPHandler() {
	runtimeFwdSig[_sigtrap] = runtimeGetsig(_sigtrap)
	runtimeHandlingSig[_sigtrap] = 1
	runtimeSetsig(_sigtrap, reflect.ValueOf(runtimeCgoSigtramp).Pointer())
}

// PrepareCrashSignalHandlers captures Go's original synchronous signal handlers.
//
// In gomobile/c-archive embeddings, package init runs on the first Go entry.
// That means a native crash reporter installed before the first Go call would
// otherwise be captured as the "Go" handler and break handler restoration on
// SIGSEGV. Go skips SIGTRAP in c-archive mode, so install its trap trampoline
// before saving handlers. Call this before installing PLCrashReporter.
func PrepareCrashSignalHandlers() {
	forceGoSIGTRAPHandler()
	C._save_go_handlers()
}

// ReinstallCrashSignalHandlers installs a combined signal handler that chains
// PLCrashReporter (native crash report) and Go's runtime handler (Go crash log).
//
// Call PrepareCrashSignalHandlers before installing PLCrashReporter, then call
// this after PLCrashReporter has been installed.
//
// Flow on SIGSEGV:
//  1. Combined handler calls PLCrashReporter's saved handler → .plcrash written
//  2. Combined handler restores Go's handler via sigaction
//  3. Combined handler returns — kernel re-executes faulting instruction
//  4. PLCrashReporter's pending raise() (SI_USER) is ignored by Go's handler
//  5. Hardware fault → Go's handler → preparePanic → kernel sigreturn →
//     sigpanic → crash output via debug.SetCrashOutput
//
// Flow on SIGTRAP:
//  1. PrepareCrashSignalHandlers force-installs Go's cgo trap trampoline
//  2. Combined handler calls PLCrashReporter's saved handler → .plcrash written
//  3. Combined handler directly calls the saved Go trap trampoline
func ReinstallCrashSignalHandlers() {
	C._reinstall_handlers()
}
