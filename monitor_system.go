//go:build windows

package ddcci

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const errno0 = windows.Errno(0)

// The RECT structure defines a rectangle by the coordinates of its upper-left and lower-right corners.
//
// See [microsoft-windows doc]
//
// [microsoft-windows doc]: https://learn.microsoft.com/en-us/windows/win32/api/windef/ns-windef-rect
type RECT struct {
	Left   int32
	Top    int32
	Right  int32
	Bottom int32
}

// MONITORINFOEXW matches the Windows MONITORINFOEXW struct.
// It carries the adapter device name (e.g. `\\.\DISPLAY2`) for a monitor.
type MONITORINFOEXW struct {
	cbSize    uint32
	RcMonitor RECT
	RcWork    RECT
	DwFlags   uint32
	SzDevice  [32]uint16
}

// SystemMonitor
//
// See [microsoft-windows doc]
//
// [microsoft-windows doc]: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nc-winuser-monitorenumproc
type SystemMonitor struct {
	// A handle to the display monitor. This value will always be non-NULL.
	hMonitor windows.Handle

	// A handle to a device context.
	// This value is NULL if the hdc parameter of EnumDisplayMonitors was NULL.
	hdc windows.Handle

	// A pointer to a RECT structure.
	rect *RECT

	// Application-defined data that EnumDisplayMonitors passes directly to the enumeration function.
	data uintptr
}

// HMonitor returns the raw HMONITOR handle of the monitor.
func (m *SystemMonitor) HMonitor() windows.Handle { return m.hMonitor }

// Rect returns the monitor rectangle.
func (m *SystemMonitor) Rect() RECT { return *m.rect }

// NewSystemMonitors
// The EnumDisplayMonitors function enumerates display monitors (including
// invisible pseudo-monitors associated with the mirroring drivers)
// that intersect a region formed by the intersection of a specified clipping rectangle
// and the visible region of a device context.
//
// [microsoft-windows doc]: https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-enumdisplaymonitors
func NewSystemMonitors() ([]SystemMonitor, error) {
	var monitors []SystemMonitor
	callback := func(hMonitor, hdc windows.Handle, rect *RECT, data uintptr) uintptr {
		monitors = append(monitors, SystemMonitor{
			hMonitor: hMonitor,
			hdc:      hdc,
			rect:     rect,
			data:     data,
		})

		// enumerate the next monitor
		return 1
	}

	const handle0 = uintptr(windows.Handle(0))
	_, _, errno := procEnumDisplayMonitors.Call(
		handle0,
		handle0,
		windows.NewCallback(callback),
		handle0,
	)
	if !errors.Is(errno, errno0) {
		return nil, errors.Join(errno, errors.New("get system monitors"))
	}
	return monitors, nil
}

// getMonitorInfoW returns MONITORINFOEXW for an HMONITOR.
func getMonitorInfoW(hMonitor windows.Handle) (*MONITORINFOEXW, error) {
	mi := &MONITORINFOEXW{}
	mi.cbSize = uint32(unsafe.Sizeof(*mi))

	r1, _, errno := procGetMonitorInfoW.Call(
		uintptr(hMonitor),
		uintptr(unsafe.Pointer(mi)),
	)
	if r1 == 0 {
		return nil, errors.Join(errno, errors.New("GetMonitorInfoW"))
	}
	return mi, nil
}

// dxva2Call is a short name: m.dxva2.Call(proc(name), m.hMonitor, args...)
// for functions taking the HMONITOR as first argument.
func (m *SystemMonitor) dxva2Call(proc *windows.LazyProc, args ...uintptr) error {
	args = append([]uintptr{uintptr(m.hMonitor)}, args...)
	r1, r2, errno := proc.Call(args...)
	if !errors.Is(errno, errno0) {
		return errors.Join(errno, fmt.Errorf("dxva2 fail to call proc(%s)", proc.Name))
	}

	_ = r1
	_ = r2

	return nil
}

// GetNumberOfPhysicalMonitorsFromHMONITOR
// Retrieves the number of physical monitors associated with an HMONITOR monitor handle.
// Call this function before calling GetPhysicalMonitorsFromHMONITOR.
//
// [out] num
// Receives the number of physical monitors associated with the monitor handle.
func (m *SystemMonitor) GetNumberOfPhysicalMonitorsFromHMONITOR() (num int, err error) {
	err = m.dxva2Call(procGetNumberOfPhysicalMonitorsFromHMONITOR,
		uintptr(unsafe.Pointer(&num)),
	)
	if err != nil {
		return 0, err
	}

	return num, nil
}
