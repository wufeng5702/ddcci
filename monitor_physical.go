//go:build windows

package ddcci

import (
	"errors"
	"fmt"
	"slices"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/niluan304/ddcci/vcp"
)

// PhysicalMonitor
// Contains a handle and text description corresponding to a physical monitor.
//
// See [microsoft-windows doc]
//
// [microsoft-windows doc]: https://learn.microsoft.com/en-us/windows/win32/api/physicalmonitorenumerationapi/ns-physicalmonitorenumerationapi-physical_monitor
type PhysicalMonitor struct {
	// Handle to the physical monitor.
	handle windows.Handle

	// Text description of the physical monitor.
	// A physical monitor description is always an array of 128 characters.
	description string
}

// physicalMonitorRaw matches the Windows PHYSICAL_MONITOR struct.
type physicalMonitorRaw struct {
	// Handle to the physical monitor.
	hPhysicalMonitor windows.Handle

	// Text description of the physical monitor.
	// A physical monitor description is always an array of 128 characters.
	szPhysicalMonitorDescription [128]uint16
}

// Description Text description of the physical monitor.
// A physical monitor description is always an array of 128 characters.
func (m *PhysicalMonitor) Description() string {
	return m.description
}

// Close destroys the underlying physical monitor handle.
// It is safe to call Close multiple times; later calls are no-ops.
// The monitor no longer accepts commands after Close returns.
func (m *PhysicalMonitor) Close() error {
	if m == nil || m.handle == 0 {
		return nil
	}

	proc := procDestroyPhysicalMonitor
	r1, _, errno := proc.Call(uintptr(m.handle))
	if r1 == 0 {
		return errors.Join(errno, errors.New("DestroyPhysicalMonitor fail to call"))
	}

	m.handle = 0
	return nil
}

// NewPhysicalMonitors
// Retrieves all physical monitors associated with an HMONITOR monitor handle.
// The caller is responsible for closing the returned monitors with Close.
//
// [in] hMonitor
// A monitor handle. PhysicalMonitor handles are returned by
// several Multiple Display PhysicalMonitor functions,
// including EnumDisplayMonitors and MonitorFromWindow,
// which are part of the graphics device interface (GDI).
//
// [out] pPhysicalMonitorArray
// Pointer to an array of PHYSICAL_MONITOR structures. The caller must allocate the array.
func NewPhysicalMonitors(m *SystemMonitor) ([]*PhysicalMonitor, error) {
	num, err := m.GetNumberOfPhysicalMonitorsFromHMONITOR()
	if err != nil {
		return nil, err
	}
	if num <= 0 {
		num = 1
	}

	// PHYSICAL_MONITOR is `HANDLE + WCHAR[128]` (264 bytes on 64-bit).
	// Passing a buffer smaller than that lets the callee write out of bounds.
	data := make([]physicalMonitorRaw, num)
	err = m.dxva2Call(procGetPhysicalMonitorsFromHMONITOR,
		uintptr(num),
		uintptr(unsafe.Pointer(&data[0])),
	)
	if err != nil {
		return nil, err
	}

	monitors := make([]*PhysicalMonitor, 0, num)
	for i := range data {
		monitors = append(monitors, &PhysicalMonitor{
			handle:      data[i].hPhysicalMonitor,
			description: windows.UTF16ToString(data[i].szPhysicalMonitorDescription[:]),
		})
	}
	return monitors, nil
}

// NewPhysicalMonitor
// Retrieves the first physical monitor associated with an HMONITOR monitor handle.
// Additional physical monitors (rare, e.g. some KVM or splitter setups) are
// released automatically, so the caller must Close the returned monitor.
func NewPhysicalMonitor(m *SystemMonitor) (*PhysicalMonitor, error) {
	monitors, err := NewPhysicalMonitors(m)
	if err != nil {
		return nil, err
	}

	first := monitors[0]
	for _, extra := range monitors[1:] {
		_ = extra.Close()
	}
	return first, nil
}

// PhysicalMonitor0
// return the first PhysicalMonitor in your Windows computer, so your computer must be connected to a monitor.
//
// it is short name: NewPhysicalMonitor(NewSystemMonitors()[0]),
// if err != nil during in the function, the result will be nil
func PhysicalMonitor0() *PhysicalMonitor {
	monitors, err := NewSystemMonitors()
	if err != nil {
		return nil
	}

	m, err := NewPhysicalMonitor(&monitors[0])
	if err != nil {
		return nil
	}
	return m
}

// call dxva2 function, where the first argument is the physical monitor handle.
// The DDC/CI bus is unreliable; a single failed call is retried once after 100ms.
func (m *PhysicalMonitor) call(proc *windows.LazyProc, args ...uintptr) error {
	args = slices.Insert(args, 0, uintptr(m.handle))

	var err error
	for range 2 {
		r1, r2, errno := proc.Call(args...)
		if errors.Is(errno, errno0) {
			_ = r1
			_ = r2

			return nil
		}
		err = errors.Join(errno, fmt.Errorf("proc(%s) fail to call(%v)", proc.Name, args))
		time.Sleep(100 * time.Millisecond)
	}

	return err
}

// GetVCPFeatureAndVCPFeatureReply
// Retrieves the current value, maximum value,
// and code type of Virtual Control Panel (VCP) code for a monitor.
//
// [in] vcpCode
// VCP code to query.
// The VCP codes are Include the VESA PhysicalMonitor Control Command Set (MCCS) standard,
// versions 1.0 and 2.0. This parameter must specify a continuous or non-continuous VCP,
// or a vendor-specific code. It should not be a table control code.
//
// [out] pdwCurrentValue
// Receives the current value of the VCP code. This parameter can be NULL.
//
// [out] pdwMaximumValue
// If vcpCode specifies a continuous VCP code, this parameter receives the maximum value of the VCP code.
// If vcpCode specifies a non-continuous VCP code, the value received in this parameter is undefined.
// This parameter can be NULL.
func (m *PhysicalMonitor) GetVCPFeatureAndVCPFeatureReply(coder vcp.Coder) (current, maxValue int, err error) {
	// [out] pvct
	//
	// Receives the VCP code type, as a member of the MC_VCP_CODE_TYPE enumeration.
	// This parameter can be NULL.
	codeType := 0

	err = m.call(procGetVCPFeatureAndVCPFeatureReply,
		uintptr(coder.Code()),
		uintptr(unsafe.Pointer(&codeType)),
		uintptr(unsafe.Pointer(&current)),
		uintptr(unsafe.Pointer(&maxValue)),
	)
	if err != nil {
		return 0, 0, err
	}

	return current, maxValue, nil
}

// SetVCPFeature
// Handle to a physical monitor. To get the monitor handle,
// call GetPhysicalMonitorsFromHMONITOR or GetPhysicalMonitorsFromIDirect3DDevice9.
//
// [in] vcpCode
// VCP code to set.
// The VCP codes are defined in the VESA PhysicalMonitor Control Command Set (MCCS) standard, version 1.0 and 2.0.
// This parameter must specify a continuous or non-continuous VCP,
// or a vendor-specific code. It should not be a table control code.
//
// [in] value
// Value of the VCP code.
//
// [out]
// If the function succeeds, the return value is TRUE.
// If the function fails, the return value is FALSE.
// To get extended error information, call GetLastError.
func (m *PhysicalMonitor) SetVCPFeature(code vcp.VCP) error {
	err := m.call(procSetVCPFeature,
		uintptr(code.Code()),
		uintptr(code.Value()),
	)
	if err != nil {
		return err
	}

	return nil
}

// GetBrightness
// Retrieves a monitor's minimum, maximum, and current brightness settings.
//
// [out] pdwMinimumBrightness
// Receives the monitor's minimum brightness.
//
// [out] pdwCurrentBrightness
// Receives the monitor's current brightness.
//
// [out] pdwMaximumBrightness
// Receives the monitor's maximum brightness.
func (m *PhysicalMonitor) GetBrightness() (minValue, current, maxValue int, err error) {
	err = m.call(procGetMonitorBrightness,
		uintptr(unsafe.Pointer(&minValue)),
		uintptr(unsafe.Pointer(&current)),
		uintptr(unsafe.Pointer(&maxValue)),
	)

	return minValue, current, maxValue, err
}

// GetCapabilities
// Retrieves the configuration capabilities of a monitor.
// Call this function to find out which high-level monitor configuration functions
// are supported by the monitor.
//
// [out] pdwMonitorCapabilities
// Receives a bitwise OR of capabilities flags. See Remarks.
//
// [out] pdwSupportedColorTemperatures
// Receives a bitwise OR of color temperature flags. See Remarks.
func (m *PhysicalMonitor) GetCapabilities() (capabilities, supportedColorTemperatures int, err error) {
	err = m.call(procGetMonitorCapabilities,
		uintptr(unsafe.Pointer(&capabilities)),
		uintptr(unsafe.Pointer(&supportedColorTemperatures)),
	)

	return capabilities, supportedColorTemperatures, err
}

// GetColorTemperature
// Retrieves a monitor's current color temperature.
//
// [out] pctCurrentColorTemperature
// Receives the monitor's current color temperature,
// specified as a member of the MC_COLOR_TEMPERATURE enumeration.
func (m *PhysicalMonitor) GetColorTemperature() (current int, err error) {
	err = m.call(procGetMonitorColorTemperature,
		uintptr(unsafe.Pointer(&current)),
	)

	return current, err
}

// GetContrast
// Retrieves a monitor's minimum, maximum, and current contrast settings.
//
// [out] pdwMinimumContrast
// Receives the monitor's minimum contrast.
//
// [out] pdwCurrentContrast
// Receives the monitor's current contrast.
//
// [out] pdwMaximumContrast
// Receives the monitor's maximum contrast.
func (m *PhysicalMonitor) GetContrast() (minValue, current, maxValue int, err error) {
	err = m.call(procGetMonitorContrast,
		uintptr(unsafe.Pointer(&minValue)),
		uintptr(unsafe.Pointer(&current)),
		uintptr(unsafe.Pointer(&maxValue)),
	)

	return minValue, current, maxValue, err
}

type PositionType int

const (
	HorizontalPosition PositionType = iota
	VerticalPosition
)

// GetDisplayAreaPosition
// Retrieves a monitor's minimum, maximum, and current horizontal or vertical position.
//
// [in] ptPositionType
//
// A member of the MC_POSITION_TYPE enumeration,
// specifying whether to retrieve the horizontal position or the vertical position.
//
// [out] pdwMinimumPosition
// Receives the minimum horizontal or vertical position.
//
// [out] pdwCurrentPosition
// Receives the current horizontal or vertical position.
//
// [out] pdwMaximumPosition
// Receives the maximum horizontal or vertical position.
func (m *PhysicalMonitor) GetDisplayAreaPosition(positionType PositionType) (minValue, current, maxValue int, err error) {
	err = m.call(procGetMonitorDisplayAreaPosition,
		uintptr(positionType),
		uintptr(unsafe.Pointer(&minValue)),
		uintptr(unsafe.Pointer(&current)),
		uintptr(unsafe.Pointer(&maxValue)),
	)

	return minValue, current, maxValue, err
}

type SizeType int

const (
	Width SizeType = iota
	Height
)

// GetDisplayAreaSize
//
// [out] pdwMinimumWidthOrHeight
// Receives the minimum width or height.
//
// [out] pdwCurrentWidthOrHeight
// Receives the current width or height.
//
// [out] pdwMaximumWidthOrHeight
// Receives the maximum width or height.
func (m *PhysicalMonitor) GetDisplayAreaSize(sizeType SizeType) (minValue, current, maxValue int, err error) {
	err = m.call(procGetMonitorDisplayAreaSize,
		uintptr(sizeType),
		uintptr(unsafe.Pointer(&minValue)),
		uintptr(unsafe.Pointer(&current)),
		uintptr(unsafe.Pointer(&maxValue)),
	)

	return minValue, current, maxValue, err
}

type DriveType int

const (
	RedDrive DriveType = iota
	GreenDrive
	BlueDrive
)

// GetRedGreenOrBlueDrive
// Retrieves a monitor's red, green, or blue drive value.
//
// [in] dtDriveType
// A member of the McDriveType enumeration, specifying whether to retrieve the red, green, or blue drive value.
//
// [out] pdwMinimumDrive
// Receives the monitor's minimum red, green, or blue drive value.
//
// [out] pdwCurrentDrive
// Receives the monitor's current red, green, or blue drive value.
//
// [out] pdwMaximumDrive
// Receives the monitor's maximum red, green, or blue drive value.
func (m *PhysicalMonitor) GetRedGreenOrBlueDrive(driveType DriveType) (minValue, current, maxValue int, err error) {
	err = m.call(procGetMonitorRedGreenOrBlueDrive,
		uintptr(driveType),
		uintptr(unsafe.Pointer(&minValue)),
		uintptr(unsafe.Pointer(&current)),
		uintptr(unsafe.Pointer(&maxValue)),
	)

	return minValue, current, maxValue, err
}

// GetRedGreenOrBlueGain
// Retrieves a monitor's red, green, or blue gain value.
//
// [in] dtDriveType
// A member of the McDriveType enumeration, specifying whether to retrieve the red, green, or blue gain value.
//
// [out] pdwMinimumGain
// Receives the monitor's minimum red, green, or blue gain value.
//
// [out] pdwCurrentGain
// Receives the monitor's current red, green, or blue gain value.
//
// [out] pdwMaximumGain
// Receives the monitor's maximum red, green, or blue gain value.
func (m *PhysicalMonitor) GetRedGreenOrBlueGain(driveType DriveType) (minValue, current, maxValue int, err error) {
	err = m.call(procGetMonitorRedGreenOrBlueGain,
		uintptr(driveType),
		uintptr(unsafe.Pointer(&minValue)),
		uintptr(unsafe.Pointer(&current)),
		uintptr(unsafe.Pointer(&maxValue)),
	)

	return minValue, current, maxValue, err
}

// GetTechnologyType
//
// [out] pdtyDisplayTechnologyType
// Receives the technology type, specified as a member of the MC_DISPLAY_TECHNOLOGY_TYPE enumeration.
func (m *PhysicalMonitor) GetTechnologyType() (value int, err error) {
	err = m.call(procGetMonitorTechnologyType,
		uintptr(unsafe.Pointer(&value)),
	)

	return value, err
}

// Degauss
// Degausses a monitor.
// Degaussing improves a monitor's image quality and color fidelity by demagnetizing the monitor.
func (m *PhysicalMonitor) Degauss() error {
	err := m.call(procDegaussMonitor)
	if err != nil {
		return err
	}

	return nil
}

// RestoreFactoryColorDefaults
// Restores a monitor's color settings to their factory defaults.
func (m *PhysicalMonitor) RestoreFactoryColorDefaults() error {
	err := m.call(procRestoreMonitorFactoryColorDefaults)
	if err != nil {
		return err
	}

	return nil
}

// RestoreFactoryDefaults
// Restores a monitor's settings to their factory defaults.
func (m *PhysicalMonitor) RestoreFactoryDefaults() error {
	err := m.call(procRestoreMonitorFactoryDefaults)
	if err != nil {
		return err
	}

	return nil
}

// SaveCurrentMonitorSettings
// Saves the current monitor settings to the display's nonvolatile storage.
func (m *PhysicalMonitor) SaveCurrentMonitorSettings() error {
	err := m.call(procSaveCurrentMonitorSettings)
	if err != nil {
		return err
	}

	return nil
}

// SetBrightness
// Sets a monitor's brightness value.
// Increasing the brightness value makes the display on the monitor brighter,
// and decreasing it makes the display dimmer.
//
// [in] dwNewBrightness
// Brightness value. To get the monitor's minimum and maximum brightness values, call GetMonitorBrightness.
//
// It is same as PhysicalMonitor.SetVCPFeature(vcp.Brightness. Value(value))
func (m *PhysicalMonitor) SetBrightness(value int) error {
	err := m.call(procSetMonitorBrightness,
		uintptr(value),
	)
	if err != nil {
		return err
	}

	return nil
}

// SetContrast
// Sets a monitor's contrast value.
//
// [in] value
// Contrast value. To get the monitor's minimum and maximum contrast values, call GetMonitorContrast.
//
// It is same as PhysicalMonitor.SetVCPFeature(vcp.Contrast. Value(value))
func (m *PhysicalMonitor) SetContrast(value int) error {
	err := m.call(procSetMonitorContrast,
		uintptr(value),
	)
	if err != nil {
		return err
	}

	return nil
}

// GetCapabilitiesStringLength
//
// Retrieves the length of a monitor's capabilities string.
//
// [out] pdwCapabilitiesStringLengthInCharacters
// Receives the length of the capabilities string, in characters, including the terminating null character.
//
// The DDC/CI bus is unreliable right after the computer wakes from sleep; the
// call is retried (up to 3 times, 100ms apart) before giving up.
func (m *PhysicalMonitor) GetCapabilitiesStringLength() (size int, err error) {
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		err = m.call(procGetCapabilitiesStringLength,
			uintptr(unsafe.Pointer(&size)),
		)
		if err == nil {
			return size, nil
		}
	}

	return 0, err
}

// CapabilitiesRequestAndCapabilitiesReply
// Retrieves a string describing a monitor's capabilities.
//
// [out] pszASCIICapabilitiesString
// Pointer to a buffer that receives the monitor's capabilities string.
// The caller must allocate this buffer.
//
// The DDC/CI bus is unreliable right after the computer wakes from sleep; the
// call is retried (up to 5 times, 100ms apart) before giving up.
func (m *PhysicalMonitor) CapabilitiesRequestAndCapabilitiesReply() (info string, err error) {
	size, err := m.GetCapabilitiesStringLength()
	if err != nil {
		return "", err
	}

	data := make([]byte, size)
	for attempt := range 5 {
		if attempt > 0 {
			time.Sleep(100 * time.Millisecond)
		}

		err = m.call(procCapabilitiesRequestAndCapabilitiesReply,
			uintptr(unsafe.Pointer(&data[0])),
			uintptr(size),
		)
		if err == nil {
			if i := slices.Index(data, 0); i >= 0 {
				data = data[:i]
			}
			return string(data), nil
		}
	}

	return "", err
}

// TimingReport
// Contains information from a monitor's timing report.
type TimingReport struct {
	// The monitor's horizontal synchronization frequency in Hz.
	HorizontalFrequency uint32

	// The monitor's vertical synchronization frequency in Hz.
	VerticalFrequency uint32

	// Timing status byte.
	// For more information about this value,
	// see the Display Data Channel Command Interface (DDC/CI) standard.
	//
	// 4.7 Get Timing Report & Timing Message
	//  Bit 7 = 1 Sync.Freq. out of range
	//  Bit 6 = 1 Unstable count
	//  Bit 5-2 Reserved, shall be set to 0
	//  Bit 1 = 1 Positive Horizontal sync polarity
	//  Bit 1 = 0 Negative Horizontal sync polarity
	//  Bit 0 = 1 Positive Vertical sync polarity
	//  Bit 0 = 0 Negative Vertical sync polarity
	TimingStatus byte
}

// GetTimingReport
// Retrieves a monitor's horizontal and vertical synchronization frequencies.
//
// [out] pmtrMonitorTimingReport
// Pointer to an TimingReport structure that receives the timing information.
func (m *PhysicalMonitor) GetTimingReport() (*TimingReport, error) {
	report := new(TimingReport)

	err := m.call(procGetTimingReport,
		uintptr(unsafe.Pointer(report)),
	)
	if err != nil {
		return nil, err
	}

	return report, nil
}

// SaveCurrentSettings
// Saves the current monitor settings to the display's nonvolatile storage.
//
// note:
// This low-level function is identical to the high-level function SaveCurrentMonitorSettings.
func (m *PhysicalMonitor) SaveCurrentSettings() error {
	err := m.call(procSaveCurrentSettings)
	if err != nil {
		return err
	}

	return nil
}
