//go:build windows

package ddcci

import "golang.org/x/sys/windows"

// Lazy-loaded system DLLs and their procedures.
// Procedures are resolved on first call and cached by the runtime.
var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dxva2  = windows.NewLazySystemDLL("dxva2.dll")

	procEnumDisplayMonitors = user32.NewProc(EnumDisplayMonitors)
	procGetMonitorInfoW     = user32.NewProc(GetMonitorInfoW)
	procEnumDisplayDevicesW = user32.NewProc(EnumDisplayDevicesW)

	procGetNumberOfPhysicalMonitorsFromHMONITOR = dxva2.NewProc(GetNumberOfPhysicalMonitorsFromHMONITOR)
	procGetPhysicalMonitorsFromHMONITOR         = dxva2.NewProc(GetPhysicalMonitorsFromHMONITOR)
	procDestroyPhysicalMonitor                  = dxva2.NewProc(DestroyPhysicalMonitor)

	procGetVCPFeatureAndVCPFeatureReply         = dxva2.NewProc(GetVCPFeatureAndVCPFeatureReply)
	procSetVCPFeature                           = dxva2.NewProc(SetVCPFeature)
	procGetMonitorBrightness                    = dxva2.NewProc(GetMonitorBrightness)
	procGetMonitorCapabilities                  = dxva2.NewProc(GetMonitorCapabilities)
	procGetMonitorColorTemperature              = dxva2.NewProc(GetMonitorColorTemperature)
	procGetMonitorContrast                      = dxva2.NewProc(GetMonitorContrast)
	procGetMonitorDisplayAreaPosition           = dxva2.NewProc(GetMonitorDisplayAreaPosition)
	procGetMonitorDisplayAreaSize               = dxva2.NewProc(GetMonitorDisplayAreaSize)
	procGetMonitorRedGreenOrBlueDrive           = dxva2.NewProc(GetMonitorRedGreenOrBlueDrive)
	procGetMonitorRedGreenOrBlueGain            = dxva2.NewProc(GetMonitorRedGreenOrBlueGain)
	procGetMonitorTechnologyType                = dxva2.NewProc(GetMonitorTechnologyType)
	procDegaussMonitor                          = dxva2.NewProc(DegaussMonitor)
	procRestoreMonitorFactoryColorDefaults      = dxva2.NewProc(RestoreMonitorFactoryColorDefaults)
	procRestoreMonitorFactoryDefaults           = dxva2.NewProc(RestoreMonitorFactoryDefaults)
	procSaveCurrentMonitorSettings              = dxva2.NewProc(SaveCurrentMonitorSettings)
	procSetMonitorBrightness                    = dxva2.NewProc(SetMonitorBrightness)
	procSetMonitorContrast                      = dxva2.NewProc(SetMonitorContrast)
	procGetCapabilitiesStringLength             = dxva2.NewProc(GetCapabilitiesStringLength)
	procCapabilitiesRequestAndCapabilitiesReply = dxva2.NewProc(CapabilitiesRequestAndCapabilitiesReply)
	procGetTimingReport                         = dxva2.NewProc(GetTimingReport)
	procSaveCurrentSettings                     = dxva2.NewProc(SaveCurrentSettings)
)
