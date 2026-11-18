package ddcci_test

import (
	"testing"

	"github.com/niluan304/ddcci"
)

func TestParseCapabilitiesTypicalPanel(t *testing.T) {
	// Typical MCCS capabilities string published by MStar-based panels.
	// Exercises: controller marker fused with the following key, cmds opcode
	// list, a value list on a non-continuous code (0x14), input sources (0x60).
	raw := "(prot(monitor)type(lcd)MStarcmds(01 02 03 07 0C E3 F3)" +
		"vcp(02 04 05 08 10 12 14(05 08 0B 0C) 16 18 1A 52 60( 11 12 0F 10) AA(01 02) " +
		"AC AE B2 B6 C6 C8 C9 D6(01 04 05) DC(00 02 03 05 ) DF FD)" +
		"mccs_ver(2.1)mswhql(1))"

	c := ddcci.ParseCapabilities(raw)

	if c.Type != "lcd" {
		t.Errorf("Type = %q, want lcd", c.Type)
	}
	if c.MCCSVer != "2.1" {
		t.Errorf("MCCSVer = %q, want 2.1", c.MCCSVer)
	}

	// Continuous control.
	if !c.Has(0x10) {
		t.Error("expected VCP 0x10 (Brightness) to be supported")
	}
	if v, ok := c.Values(0x10); !ok {
		t.Fatal("Values(0x10) reported unsupported")
	} else if v != nil {
		t.Errorf("VCP 0x10 values = %v, want nil (continuous)", v)
	}

	// Non-continuous with a value list.
	got, ok := c.Values(0x14)
	if !ok {
		t.Fatal("expected VCP 0x14 to be supported")
	}
	want := []int{0x05, 0x08, 0x0B, 0x0C}
	if len(got) != len(want) {
		t.Fatalf("VCP 0x14 values = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VCP 0x14 values = %v, want %v", got, want)
		}
	}

	got, ok = c.Values(0xD6)
	if !ok {
		t.Fatal("expected VCP 0xD6 (PowerMode) to be supported")
	}
	want = []int{0x01, 0x04, 0x05}
	if len(got) != len(want) {
		t.Fatalf("VCP 0xD6 values = %v, want %v", got, want)
	}

	// Input sources should keep their spacing-free value list.
	got, ok = c.Values(0x60)
	if !ok {
		t.Fatal("expected VCP 0x60 to be supported")
	}
	if len(got) != 4 {
		t.Fatalf("VCP 0x60 values = %v, want 4 entries", got)
	}

	// Absent code.
	if c.Has(0xCD) {
		t.Error("VCP 0xCD should not be supported on this monitor")
	}
	if _, ok := c.Values(0xCD); ok {
		t.Error("Values(0xCD) reported ok, want not supported")
	}
}

func TestParseCapabilitiesStructure(t *testing.T) {
	raw := "(prot(monitor)type(tft)model(XG27)cmds(01 02)vcp(10 12(03 04 06) 60(01 03))mccs_ver(2.2))"
	c := ddcci.ParseCapabilities(raw)

	if c.Type != "tft" {
		t.Errorf("Type = %q", c.Type)
	}
	if c.Model != "XG27" {
		t.Errorf("Model = %q, want XG27", c.Model)
	}
	if c.MCCSVer != "2.2" {
		t.Errorf("MCCSVer = %q, want 2.2", c.MCCSVer)
	}

	got, ok := c.Values(0x12)
	if !ok || len(got) != 3 || got[2] != 0x06 {
		t.Errorf("Values(0x12) = %v, %v", got, ok)
	}
	got, ok = c.Values(0x60)
	if !ok || len(got) != 2 || got[1] != 0x03 {
		t.Errorf("Values(0x60) = %v, %v", got, ok)
	}
}

func TestParseCapabilitiesGarbage(t *testing.T) {
	c := ddcci.ParseCapabilities("this is not a capabilities string")
	if c.Raw != "this is not a capabilities string" {
		t.Errorf("Raw = %q", c.Raw)
	}
	if len(c.Features) != 0 {
		t.Errorf("Features = %v, want empty", c.Features)
	}
	if c.Has(0x10) {
		t.Error("Has(0x10) = true on garbage input")
	}

	// Never nil, never panic.
	for _, s := range []string{"", "(", ")", "(((", "vcp(", "vcp() ( (", "type(lcd)model(XGE"} {
		_ = ddcci.ParseCapabilities(s)
	}
}
