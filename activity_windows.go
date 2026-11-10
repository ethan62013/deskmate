//go:build windows

package main

import (
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32                  = windows.NewLazySystemDLL("user32.dll")
	kernel32                = windows.NewLazySystemDLL("kernel32.dll")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadPID  = user32.NewProc("GetWindowThreadProcessId")
	procGetWindowRect       = user32.NewProc("GetWindowRect")
	procMonitorFromWindow   = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW     = user32.NewProc("GetMonitorInfoW")
	procGetLastInputInfo    = user32.NewProc("GetLastInputInfo")
	procGetShellWindow      = user32.NewProc("GetShellWindow")
	procGetTickCount64      = kernel32.NewProc("GetTickCount64")
)

type rect struct{ Left, Top, Right, Bottom int32 }

type monitorInfo struct {
	CbSize    uint32
	RcMonitor rect
	RcWork    rect
	DwFlags   uint32
}

type lastInputInfo struct {
	CbSize uint32
	DwTime uint32
}

const monitorDefaultToNearest = 2

func foregroundApp() (exe string, fullscreen bool) {
	hwnd, _, _ := procGetForegroundWindow.Call()
	if hwnd == 0 {
		return "", false
	}
	if shell, _, _ := procGetShellWindow.Call(); shell == hwnd {
		return "explorer.exe", false
	}
	var pid uint32
	procGetWindowThreadPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	if pid != 0 {
		if h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid); err == nil {
			buf := make([]uint16, windows.MAX_PATH)
			size := uint32(len(buf))
			if windows.QueryFullProcessImageName(h, 0, &buf[0], &size) == nil {
				exe = strings.ToLower(filepath.Base(windows.UTF16ToString(buf[:size])))
			}
			windows.CloseHandle(h)
		}
	}

	var wr rect
	if ok, _, _ := procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr))); ok != 0 {
		mon, _, _ := procMonitorFromWindow.Call(hwnd, monitorDefaultToNearest)
		mi := monitorInfo{CbSize: uint32(unsafe.Sizeof(monitorInfo{}))}
		if mon != 0 {
			if ok, _, _ := procGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); ok != 0 {
				m := mi.RcMonitor
				fullscreen = wr.Left <= m.Left && wr.Top <= m.Top && wr.Right >= m.Right && wr.Bottom >= m.Bottom
			}
		}
	}
	return exe, fullscreen
}

func idleDuration() time.Duration {
	info := lastInputInfo{CbSize: uint32(unsafe.Sizeof(lastInputInfo{}))}
	if ok, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 0
	}
	now, _, _ := procGetTickCount64.Call()
	// DwTime is a 32-bit tick count; compare in 32-bit space to survive wraparound.
	elapsed := uint32(now) - info.DwTime
	return time.Duration(elapsed) * time.Millisecond
}
