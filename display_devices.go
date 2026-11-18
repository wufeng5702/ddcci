//go:build windows

package ddcci

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	eddGetDeviceInterfaceName      = 0x00000001
	displayDeviceAttachedToDesktop = 0x00000001
	displayDeviceMirroringDriver   = 0x00000008
)

// displayDeviceW matches the Windows displayDeviceW struct (cb + 5 fields).
type displayDeviceW struct {
	Cb           uint32
	DeviceName   [32]uint16
	DeviceString [128]uint16
	StateFlags   uint32
	DeviceID     [128]uint16
	DeviceKey    [128]uint16
}

// DisplayDevice is the identifying information for a single attached display device
// (e.g. one element of an adapter's display list).
//
// DeviceID is the full device instance path as reported by EnumDisplayDevicesW,
// for example:
//
//	MONITOR\ABC1234\{4d36e96e-e325-11ce-bfc1-08002be10318}\0000
//
// DeviceKey is DeviceID without the trailing class GUID instance part, which is
// what tooling usually reports as the "device key"; it is stable across reboots.
type DisplayDevice struct {
	AdapterName string // adapter the device is attached to, e.g. \\.\DISPLAY2
	DeviceName  string // e.g. \\.\DISPLAY2\Monitor0
	DeviceID    string
	DeviceKey   string
}

// GetDisplayDevices enumerates all display devices attached to the desktop.
// DeviceID is the device instance path (e.g. MONITOR\ABC1234\...); it is what
// tooling reports as the device "key" and is stable across reboots.
func GetDisplayDevices() ([]DisplayDevice, error) {
	return getDisplayDevices(0)
}

// getDisplayDevices enumerates display devices with the given EnumDisplayDevices
// invariant flags (0 = device instance path, eddGetDeviceInterfaceName = device interface path).
func getDisplayDevices(flags uint32) ([]DisplayDevice, error) {
	var devices []DisplayDevice

	adapterDev := displayDeviceW{}
	adapterDev.Cb = uint32(unsafe.Sizeof(displayDeviceW{}))

	for adapterIndex := 0; ; adapterIndex++ {
		r1, _, _ := procEnumDisplayDevicesW.Call(
			0, // NULL – enumerate adapters
			uintptr(adapterIndex),
			uintptr(unsafe.Pointer(&adapterDev)),
			0,
		)
		if r1 == 0 {
			break
		}

		adapterName := syscall.UTF16ToString(adapterDev.DeviceName[:])

		displayDev := displayDeviceW{}
		displayDev.Cb = uint32(unsafe.Sizeof(displayDeviceW{}))

		for displayIndex := 0; ; displayIndex++ {
			r1, _, _ := procEnumDisplayDevicesW.Call(
				uintptr(unsafe.Pointer(&adapterDev.DeviceName[0])),
				uintptr(displayIndex),
				uintptr(unsafe.Pointer(&displayDev)),
				uintptr(flags),
			)
			if r1 == 0 {
				break
			}

			if displayDev.StateFlags&displayDeviceAttachedToDesktop == 0 {
				continue
			}

			dd := DisplayDevice{
				AdapterName: adapterName,
				DeviceName:  syscall.UTF16ToString(displayDev.DeviceName[:]),
				DeviceID:    syscall.UTF16ToString(displayDev.DeviceID[:]),
			}
			dd.DeviceKey = extractDeviceKey(dd.DeviceID)

			devices = append(devices, dd)
		}
	}

	return devices, nil
}

// extractDeviceKey extracts the persistent device key from a deviceID string.
// The key is the portion before the "#{" class-instance GUID, if present.
func extractDeviceKey(deviceID string) string {
	if idx := strings.Index(deviceID, "#{"); idx >= 0 {
		return deviceID[:idx]
	}
	return deviceID
}

// DisplayDevice returns identifying information for the display device attached
// to the same adapter as this monitor handle (first attached display device).
//
// Multi-monitor KVM/splitter setups where a single HMONITOR hosts several
// physical monitors are matched in the same order as the Windows display
// enumeration order.
func (m *SystemMonitor) DisplayDevice() (*DisplayDevice, error) {
	mi, err := getMonitorInfoW(m.hMonitor)
	if err != nil {
		return nil, err
	}

	devices, err := GetDisplayDevices()
	if err != nil {
		return nil, err
	}

	adapter := strings.TrimRight(syscall.UTF16ToString(mi.SzDevice[:]), "\x00")
	for i := range devices {
		if devices[i].AdapterName == adapter {
			return &devices[i], nil
		}
	}

	return nil, fmt.Errorf("display device for adapter %q not found", adapter)
}

// GetEDID reads, parses and returns the display's EDID.
func (m *SystemMonitor) GetEDID() (*EDIDInfo, error) {
	mi, err := getMonitorInfoW(m.hMonitor)
	if err != nil {
		return nil, err
	}

	adapter := strings.TrimRight(syscall.UTF16ToString(mi.SzDevice[:]), "\x00")
	return readEDIDForAdapter(adapter)
}

// readEDIDForAdapter locates the monitor device under the adapter's registry
// node and returns its parsed EDID.
func readEDIDForAdapter(adapter string) (*EDIDInfo, error) {
	// Match the same display across the two enumeration modes. Both are full,
	// ordered enumerations so indexes correspond.
	devices, err := getDisplayDevices(0)
	if err != nil {
		return nil, err
	}
	iface, err := getDisplayDevices(eddGetDeviceInterfaceName)
	if err != nil {
		return nil, err
	}

	for i := range devices {
		if devices[i].AdapterName != adapter {
			continue
		}

		if i < len(iface) {
			if path := registryPathFromDeviceInterface(iface[i].DeviceID); path != "" {
				return readEDIDFromRegistry(path)
			}
		}
		if i < len(devices) {
			// Fall back to the stable instance ID from the device path.
			if path := registryPathFromInstanceID(devices[i].DeviceID); path != "" {
				return readEDIDFromRegistry(path)
			}
		}
	}

	return nil, fmt.Errorf("display device for adapter %q not found", adapter)
}

// registryPathFromDeviceInterface converts a device interface path such as
//
//	\\?\DISPLAY#ABC1234#1&7689abc&0&UID123#{e6f07b5f-ee97-4a90-b076-33f57bf4eaa7}
//
// into the corresponding registry key path:
//
//	SYSTEM\CurrentControlSet\Enum\DISPLAY\ABC1234\1&7689abc&0&UID123\Device Parameters
func registryPathFromDeviceInterface(iface string) string {
	s := strings.TrimPrefix(iface, `\\?\`)
	parts := strings.Split(s, "#")
	if len(parts) < 3 {
		return ""
	}
	class, hwid, instance := parts[0], parts[1], parts[2]
	if class == "" || hwid == "" || instance == "" {
		return ""
	}
	// Case normalizations: the registry stores the same casing as the interface path.
	return `SYSTEM\CurrentControlSet\Enum\` + class + `\` + hwid + `\` + instance + `\Device Parameters`
}

// registryPathFromInstanceID converts a device instance path such as
//
//	DISPLAY\ABC1234\{4d36e96e-e325-11ce-bfc1-08002be10318}\0000
//
// into the closest registry key path. This only works when the enumeration
// class matches the Enum class used by the driver.
func registryPathFromInstanceID(instance string) string {
	s := strings.TrimPrefix(instance, `\\?\`)
	parts := strings.Split(s, `\`)
	if len(parts) < 3 {
		return ""
	}
	return `SYSTEM\CurrentControlSet\Enum\` + s + `\Device Parameters`
}
