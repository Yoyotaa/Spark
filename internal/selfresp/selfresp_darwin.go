//go:build darwin && !bindings

package selfresp

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation
#import <Foundation/Foundation.h>
#include <spawn.h>
#include <stdlib.h>
#include <unistd.h>

extern char **environ;

// Private but stable (macOS 10.14+), used by Chromium and Qt for the same
// purpose. Takes a pointer to the attributes (verified: passing the value
// returns success but changes nothing).
int responsibility_spawnattrs_setdisclaim(posix_spawnattr_t *attrs, int disclaim)
	__attribute__((weak_import));

static int sparkSpawnDisclaimed(const char *path, char **argv, pid_t *pid) {
	if (responsibility_spawnattrs_setdisclaim == NULL) {
		return -1;
	}
	posix_spawnattr_t attr;
	if (posix_spawnattr_init(&attr) != 0) {
		return -1;
	}
	if (responsibility_spawnattrs_setdisclaim(&attr, 1) != 0) {
		posix_spawnattr_destroy(&attr);
		return -1;
	}
	int rc = posix_spawn(pid, path, NULL, &attr, argv, environ);
	posix_spawnattr_destroy(&attr);
	return rc;
}

static int sparkInBundle(void) {
	return [[NSBundle mainBundle] bundleIdentifier] != nil ? 1 : 0;
}

// Also private, and the ground truth: which process macOS holds
// responsible for this one (its own pid when it already is).
pid_t responsibility_get_pid_responsible_for_pid(pid_t pid) __attribute__((weak_import));

static int sparkResponsibleForSelf(void) {
	if (responsibility_get_pid_responsible_for_pid == NULL) {
		return 1; // cannot tell: do not relaunch
	}
	return responsibility_get_pid_responsible_for_pid(getpid()) == getpid() ? 1 : 0;
}
*/
import "C"

import (
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"
)

// launchedByAnotherApp: another app (the terminal's host: GoLand,
// Terminal...) is responsible for this process. Asked to macOS directly:
// terminals do not reliably pass hints such as __CFBundleIdentifier.
func launchedByAnotherApp() bool {
	return C.sparkInBundle() == 1 && C.sparkResponsibleForSelf() == 0
}

// Relaunch re-executes Spark as its own responsible process when needed.
// The caller must exit with the returned code when relaunched is true: this
// process then only stays as a thin parent so `wails dev` keeps its handle.
func Relaunch() (code int, relaunched bool) {
	if os.Getenv(envDisclaimed) != "" {
		watchParent()
		return 0, false
	}
	if !launchedByAnotherApp() {
		return 0, false
	}
	exe, err := os.Executable()
	if err != nil {
		return 0, false
	}
	os.Setenv(envDisclaimed, "1")

	cpath := C.CString(exe)
	defer C.free(unsafe.Pointer(cpath))
	argv := make([]*C.char, len(os.Args)+1)
	for i, a := range os.Args {
		argv[i] = C.CString(a)
		defer C.free(unsafe.Pointer(argv[i]))
	}
	var pid C.pid_t
	if C.sparkSpawnDisclaimed(cpath, (**C.char)(unsafe.Pointer(&argv[0])), &pid) != 0 {
		os.Unsetenv(envDisclaimed)
		return 0, false // run in this process, as before
	}

	// Forward termination requests to the real app.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for s := range sigs {
			_ = syscall.Kill(int(pid), s.(syscall.Signal))
		}
	}()
	var status syscall.WaitStatus
	for {
		_, err := syscall.Wait4(int(pid), &status, 0, nil)
		if err != syscall.EINTR {
			break
		}
	}
	if status.Signaled() {
		return 128 + int(status.Signal()), true
	}
	return status.ExitStatus(), true
}

// watchParent ends the relaunched app if its parent dies: `wails dev`
// kills the parent with SIGKILL on every rebuild, which cannot be forwarded.
func watchParent() {
	parent := os.Getppid()
	go func() {
		for range time.Tick(500 * time.Millisecond) {
			if os.Getppid() != parent {
				os.Exit(0)
			}
		}
	}()
}
