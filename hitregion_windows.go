//go:build windows

package main

import "golang.org/x/sys/windows"

var (
	gdi32              = windows.NewLazySystemDLL("gdi32.dll")
	procCreateRectRgn  = gdi32.NewProc("CreateRectRgn")
	procCombineRgn     = gdi32.NewProc("CombineRgn")
	procDeleteObject   = gdi32.NewProc("DeleteObject")
	procSetWindowRgn   = user32.NewProc("SetWindowRgn")
	procReleaseCapture = user32.NewProc("ReleaseCapture")
	procPostMessageW   = user32.NewProc("PostMessageW")
)

const (
	rgnOr           = 2
	wmNCLButtonDown = 0x00A1
	htCaption       = 2
)

// startNativeDrag hands the drag to Windows' own move loop, as if the title bar was grabbed.
// It must run on the UI thread, which owns the mouse capture.
func startNativeDrag(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	procReleaseCapture.Call()
	procPostMessageW.Call(hwnd, wmNCLButtonDown, htCaption, 0)
}

// setHitRegion limits where the window receives mouse input; clicks elsewhere fall
// through to whatever is underneath. An empty list restores the whole window.
func setHitRegion(hwnd uintptr, rects []HitRect) {
	if hwnd == 0 {
		return
	}
	if len(rects) == 0 {
		procSetWindowRgn.Call(hwnd, 0, 1)
		return
	}
	region, _, _ := procCreateRectRgn.Call(0, 0, 0, 0)
	for _, r := range rects {
		part, _, _ := procCreateRectRgn.Call(
			uintptr(int32(r.X)), uintptr(int32(r.Y)),
			uintptr(int32(r.X+r.W)), uintptr(int32(r.Y+r.H)),
		)
		procCombineRgn.Call(region, region, part, rgnOr)
		procDeleteObject.Call(part)
	}
	// On success the system owns the region, so it must not be deleted here.
	if ok, _, _ := procSetWindowRgn.Call(hwnd, region, 1); ok == 0 {
		procDeleteObject.Call(region)
	}
}
