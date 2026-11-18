//go:build windows

// ddcci command line tool.
//
// Exit codes: 0 on success, 1 on error, 2 on usage error.
//
//	ddcci list                 List connected monitors (description, EDID model, device key)
//	ddcci caps                 Dump the raw MCCS capabilities string, parsed when available
//	ddcci get <code>           Read a VCP code (hex, e.g. 0x10, or decimal)
//	ddcci set <code> <value>   Write a VCP code
//	ddcci scan [start [end]]   Read a range of VCP codes, default 0xE0-0xFF (read-only)
//	ddcci power <mode>         Write Power Mode (0xD6): on|standby|suspend|off|hardoff
//
// Global flags for every command:
//
//	-i N        select monitor by index from `ddcci list` (default 0)
//	-format F   output format: text|json (default "text")
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/niluan304/ddcci"
	"github.com/niluan304/ddcci/vcp"
)

const (
	formatText = "text"
	formatJSON = "json"
)

var errUsage = errors.New("usage error")

// JSON output keys shared between commands.
const (
	jsonKeyCode    = "code"
	jsonKeyCurrent = "current"
	jsonKeyMax     = "max"
	jsonKeyWritten = "written"
	jsonKeyReread  = "reread"
	jsonKeyValue   = "value"
)

type commonFlags struct {
	index  int
	format string
}

func (c *commonFlags) register(fs *flag.FlagSet) {
	fs.IntVar(&c.index, "i", 0, "monitor index from `ddcci list`")
	fs.StringVar(&c.format, "format", "text", "output format: text|json")
}

// parse parses args with fs and validates the output format.
func (c *commonFlags) parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	switch c.format {
	case formatText, formatJSON:
		return nil
	default:
		fmt.Fprintf(os.Stderr, "invalid -format %q (expected text|json)\n", c.format)
		return errUsage
	}
}

func (c *commonFlags) isJSON() bool { return c.format == formatJSON }

type monitorEntry struct {
	system   ddcci.SystemMonitor
	physical *ddcci.PhysicalMonitor
}

func collectMonitors() ([]monitorEntry, error) {
	systems, err := ddcci.NewSystemMonitors()
	if err != nil {
		return nil, fmt.Errorf("enumerate system monitors: %w", err)
	}

	var out []monitorEntry
	for i := range systems {
		m := systems[i]
		phys, err := ddcci.NewPhysicalMonitors(&m)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip monitor %d: %v\n", i, err)
			continue
		}
		for _, p := range phys {
			out = append(out, monitorEntry{system: m, physical: p})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no physical monitors found")
	}
	return out, nil
}

func closeAll(entries []monitorEntry) {
	for i := range entries {
		_ = entries[i].physical.Close()
	}
}

func pickMonitor(entries []monitorEntry, index int) (monitorEntry, error) {
	if index < 0 || index >= len(entries) {
		return monitorEntry{}, fmt.Errorf("monitor index %d out of range [0, %d)", index, len(entries))
	}
	return entries[index], nil
}

// withMonitor collects monitors, selects the one at index, and runs fn.
// All collected monitor handles are closed when fn returns.
func withMonitor(index int, fn func(monitorEntry) error) error {
	monitors, err := collectMonitors()
	if err != nil {
		return err
	}
	defer closeAll(monitors)

	m, err := pickMonitor(monitors, index)
	if err != nil {
		return err
	}
	return fn(m)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func parseInt(s string) (int, error) {
	if v, err := strconv.ParseInt(s, 0, 64); err == nil {
		return int(v), nil
	}
	v, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q (use decimal, 0x-prefixed hex, or bare hex)", s)
	}
	return int(v), nil
}

func monitorName(e monitorEntry) string {
	name := e.physical.Description()
	if ed, err := e.system.GetEDID(); err == nil && ed.ModelName != "" {
		if name == "Generic PnP Monitor" {
			return ed.ModelName
		}
		return name + " (" + ed.ModelName + ")"
	}
	return name
}

type monitorJSON struct {
	Index       int    `json:"index"`
	Description string `json:"description"`
	Model       string `json:"model,omitempty"`
	Adapter     string `json:"adapter,omitempty"`
	DeviceKey   string `json:"deviceKey,omitempty"`
	Ddcci       string `json:"ddcci"`
}

func fillMonitorJSON(m monitorEntry, index int) monitorJSON {
	j := monitorJSON{Index: index, Description: m.physical.Description()}

	if d, err := m.system.DisplayDevice(); err == nil {
		j.Adapter = d.AdapterName
		j.DeviceKey = d.DeviceKey
	}
	if ed, err := m.system.GetEDID(); err == nil {
		j.Model = ed.ModelName
	}
	if _, _, err := m.physical.GetVCPFeatureAndVCPFeatureReply(vcp.Brightness); err != nil {
		j.Ddcci = "invalid"
	} else {
		j.Ddcci = "ok"
	}
	return j
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := cf.parse(fs, args); err != nil {
		return err
	}

	monitors, err := collectMonitors()
	if err != nil {
		return err
	}
	defer closeAll(monitors)

	mj := make([]monitorJSON, 0, len(monitors))
	for i := range monitors {
		mj = append(mj, fillMonitorJSON(monitors[i], i))
	}

	if cf.isJSON() {
		return printJSON(mj)
	}
	for i := range mj {
		m := &mj[i]
		fmt.Printf("[%d] %s\n", m.Index, m.Description)
		if m.Model != "" {
			fmt.Printf("    model:     %s\n", m.Model)
		}
		if m.Adapter != "" {
			fmt.Printf("    adapter:   %s\n", m.Adapter)
		}
		if m.DeviceKey != "" {
			fmt.Printf("    deviceKey: %s\n", m.DeviceKey)
		}
		fmt.Printf("    ddcci:     %s\n", m.Ddcci)
	}
	return nil
}

func printCapsHuman(m monitorEntry, caps *ddcci.Capabilities, raw string) {
	fmt.Printf("monitor:  %s\n", monitorName(m))
	fmt.Printf("raw:      %s\n\n", strings.TrimRight(raw, "\x00"))
	fmt.Printf("type:     %s\n", caps.Type)
	if caps.Model != "" {
		fmt.Printf("model:    %s\n", caps.Model)
	}
	fmt.Printf("mccs:     %s\n", caps.MCCSVer)

	codes := make([]uint8, 0, len(caps.Features))
	for code := range caps.Features {
		codes = append(codes, code)
	}
	slices.Sort(codes)

	fmt.Println("vcp:")
	for _, code := range codes {
		features := caps.Features[code]
		if features == nil {
			fmt.Printf("  0x%02X\n", code)
			continue
		}

		vals := make([]string, 0, len(features))
		for _, x := range features {
			vals = append(vals, fmt.Sprintf("0x%02X", x))
		}
		fmt.Printf("  0x%02X: %s\n", code, strings.Join(vals, " "))
	}
}

func cmdCaps(args []string) error {
	fs := flag.NewFlagSet("caps", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := cf.parse(fs, args); err != nil {
		return err
	}

	return withMonitor(cf.index, func(m monitorEntry) error {
		raw, err := m.physical.CapabilitiesRequestAndCapabilitiesReply()
		if err != nil {
			return fmt.Errorf("read capabilities: %w", err)
		}
		caps := ddcci.ParseCapabilities(raw)

		if cf.isJSON() {
			return printJSON(caps)
		}
		printCapsHuman(m, caps, raw)
		return nil
	})
}

func cmdGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := cf.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: ddcci get <code>")
		return errUsage
	}

	code, err := parseInt(fs.Arg(0))
	if err != nil {
		return err
	}

	return withMonitor(cf.index, func(m monitorEntry) error {
		current, maxValue, err := m.physical.GetVCPFeatureAndVCPFeatureReply(rawCode(code))
		if err != nil {
			return fmt.Errorf("read vcp 0x%02X: %w", code, err)
		}
		if cf.isJSON() {
			return printJSON(map[string]any{jsonKeyCode: code, jsonKeyCurrent: current, jsonKeyMax: maxValue})
		}
		fmt.Printf("0x%02X: current=0x%04X (%d) max=0x%04X (%d)\n", code, current, current, maxValue, maxValue)
		return nil
	})
}

func cmdSet(args []string) error {
	fs := flag.NewFlagSet("set", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := cf.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(os.Stderr, "usage: ddcci set <code> <value>")
		return errUsage
	}

	code, err := parseInt(fs.Arg(0))
	if err != nil {
		return err
	}
	value, err := parseInt(fs.Arg(1))
	if err != nil {
		return err
	}

	return withMonitor(cf.index, func(m monitorEntry) error {
		if err := m.physical.SetVCPFeature(vcp.NewVCP(code, value)); err != nil {
			return fmt.Errorf("write vcp 0x%02X = %d: %w", code, value, err)
		}

		current, _, verr := m.physical.GetVCPFeatureAndVCPFeatureReply(rawCode(code))
		if cf.isJSON() {
			v := map[string]any{jsonKeyCode: code, jsonKeyWritten: value}
			if verr == nil {
				v[jsonKeyReread] = current
			}
			return printJSON(v)
		}
		fmt.Printf("0x%02X: written %d\n", code, value)
		if verr == nil {
			fmt.Printf("0x%02X: re-read current=0x%04X (%d)\n", code, current, current)
		} else {
			fmt.Printf("0x%02X: re-read failed: %v\n", code, verr)
		}
		return nil
	})
}

func parseScanRange(args []string) (int, int, error) {
	switch len(args) {
	case 0:
		return 0xE0, 0xFF, nil
	case 1:
		v, err := parseInt(args[0])
		if err != nil {
			return 0, 0, err
		}
		return v, 0xFF, nil
	case 2:
		start, err := parseInt(args[0])
		if err != nil {
			return 0, 0, err
		}
		end, err := parseInt(args[1])
		if err != nil {
			return 0, 0, err
		}
		return start, end, nil
	default:
		fmt.Fprintln(os.Stderr, "usage: ddcci scan [start [end]]")
		return 0, 0, errUsage
	}
}

func cmdScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := cf.parse(fs, args); err != nil {
		return err
	}

	start, end, err := parseScanRange(fs.Args())
	if err != nil {
		return err
	}

	type scanRow struct {
		Code    int    `json:"code"`
		Current int    `json:"current"`
		Max     int    `json:"max"`
		Err     string `json:"error,omitempty"`
	}

	return withMonitor(cf.index, func(m monitorEntry) error {
		var rows []scanRow
		for code := start; code <= end; code++ {
			current, maxValue, err := m.physical.GetVCPFeatureAndVCPFeatureReply(rawCode(code))
			if err != nil {
				rows = append(rows, scanRow{Code: code, Err: err.Error()})
				continue
			}
			rows = append(rows, scanRow{Code: code, Current: current, Max: maxValue})
		}

		if cf.isJSON() {
			return printJSON(rows)
		}
		for _, r := range rows {
			if r.Err != "" {
				fmt.Printf("0x%02X: -\n", r.Code)
				continue
			}
			fmt.Printf("0x%02X: current=0x%04X (%d) max=0x%04X (%d)\n",
				r.Code, r.Current, r.Current, r.Max, r.Max)
		}
		return nil
	})
}

func parsePowerMode(arg string) (int, string, error) {
	modes := map[string]int{
		"on":       0x01,
		"standby":  0x02,
		"suspend":  0x03,
		"off":      0x04,
		"hardoff":  0x05,
		"poweroff": 0x05,
		"hard-off": 0x05,
	}
	key := strings.ToLower(arg)
	if value, ok := modes[key]; ok {
		return value, key, nil
	}
	v, err := parseInt(arg)
	if err != nil {
		return 0, "", fmt.Errorf("unknown mode %q (use on|standby|suspend|off|hardoff or a number 1-5)", arg)
	}
	return v, arg, nil
}

func cmdPower(args []string) error {
	fs := flag.NewFlagSet("power", flag.ContinueOnError)
	var cf commonFlags
	cf.register(fs)
	if err := cf.parse(fs, args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: ddcci power on|standby|suspend|off|hardoff")
		return errUsage
	}

	value, name, err := parsePowerMode(fs.Arg(0))
	if err != nil {
		return err
	}

	return withMonitor(cf.index, func(m monitorEntry) error {
		if err := m.physical.SetVCPFeature(vcp.NewVCP(0xD6, value)); err != nil {
			return fmt.Errorf("write power mode 0x%02X: %w", value, err)
		}
		if cf.isJSON() {
			return printJSON(map[string]any{"mode": name, jsonKeyValue: value, jsonKeyWritten: true})
		}
		fmt.Printf("power mode written: 0x%02X (%s)\n", value, name)
		return nil
	})
}

const usageText = `monitor diagnostic / control CLI (Windows, DDC/CI)

Usage: ddcci <command> [options]

Commands:
  list                 List connected monitors (description, EDID model, device key)
  caps                 Dump the raw MCCS capabilities string, parsed when available
  get <code>           Read a VCP code (0x10 or 16)
  set <code> <value>   Write a VCP code
  scan [start [end]]   Read a range of VCP codes, default 0xE0-0xFF (read-only)
  power <mode>         Write Power Mode (0xD6): on|standby|suspend|off|hardoff

Global flags (all commands):
  -i N        select monitor by index from "ddcci list" (default 0)
  -format F   output format: text|json (default "text")

Exit codes: 0 on success, 1 on error, 2 on usage error.
`

type rawCode int

func (c rawCode) Code() int { return int(c) }

func run(cmd string, args []string) error {
	switch cmd {
	case "list":
		return cmdList(args)
	case "caps":
		return cmdCaps(args)
	case "get":
		return cmdGet(args)
	case "set":
		return cmdSet(args)
	case "scan":
		return cmdScan(args)
	case "power":
		return cmdPower(args)
	case "probe": // deprecated alias of caps
		return cmdCaps(args)
	case "-h", "--help", "help":
		fmt.Print(usageText)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n%s", cmd, usageText)
		return errUsage
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		os.Exit(2)
	}

	err := run(os.Args[1], os.Args[2:])
	if errors.Is(err, errUsage) {
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
