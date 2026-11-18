//go:build windows

package ddcci_test

import (
	"testing"

	"github.com/niluan304/ddcci"
)

func TestDisplayDevices(t *testing.T) {
	devices, err := ddcci.GetDisplayDevices()
	if err != nil {
		t.Fatalf("GetDisplayDevices: %v", err)
	}
	for _, d := range devices {
		t.Logf("Adapter=%q DeviceName=%q DeviceID=%q DeviceKey=%q", d.AdapterName, d.DeviceName, d.DeviceID, d.DeviceKey)
	}
}

func TestGetEDID(t *testing.T) {
	monitors, err := ddcci.NewSystemMonitors()
	if err != nil {
		t.Fatalf("NewSystemMonitors: %v", err)
	}
	for i := range monitors {
		ed, err := monitors[i].GetEDID()
		if err != nil {
			t.Logf("monitor %d: %v", i, err)
			continue
		}
		t.Logf("monitor %d: model=%q serial=%d edidLen=%d week=%d year=%d", i, ed.ModelName, ed.Serial, len(ed.Raw), ed.Week, ed.Year)
	}
}
