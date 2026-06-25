//go:build windows

package main

import "golang.org/x/sys/windows"

// HideConsoleWindow hides the console window attached to this process, if any.
//
// It is called by `catalogs sync --background` so the hourly Task Scheduler
// heartbeat does not flash a blank console window. Best-effort: a process with
// no console (HWND 0) or any call failure is silently ignored — hiding the
// window must never block or fail the sync.
//
// GetConsoleWindow (kernel32) and ShowWindow (user32) are loaded lazily because
// golang.org/x/sys/windows does not wrap them directly. NewLazySystemDLL
// resolves from the Windows system directory, so this does not introduce a
// DLL-planting surface.
func HideConsoleWindow() {
	getConsoleWindow := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")
	showWindow := windows.NewLazySystemDLL("user32.dll").NewProc("ShowWindow")

	hwnd, _, _ := getConsoleWindow.Call()
	if hwnd == 0 {
		return // no console attached (e.g. conhost --headless) — nothing to hide
	}
	_, _, _ = showWindow.Call(hwnd, uintptr(windows.SW_HIDE))
}
