## DDCCI

`ddcci` is a go library that calls the interface provided by the operating system and interacts with the display through the `DDC/CI` display communication standard protocol.
- Get the description, driver information, and current location of the monitor
- Get/Set the brightness, contrast, color temperature, or other adjustable parameters of the monitor
- Modify the power mode of the monitor (requires support from the monitor itself)

## Install
```shell
go get github.com/niluan304/ddcci
```

## Example
```go
package main

import (
	"fmt"

	"github.com/niluan304/ddcci"
)

func main() {
	monitors, err := ddcci.NewSystemMonitors()
	if err != nil {
		panic(err)
	}

	for _, monitor := range monitors {
		m, err := ddcci.NewPhysicalMonitor(&monitor)
		if err != nil {
			panic(err)
		}

		minValue, current, maxValue, err := m.GetBrightness()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Brightness: minValue %d, current: %d, maxValue %d\n", minValue, current, maxValue)
	}
}

```

## CLI
This repository includes an ad-hoc command line tool (see `cmd/ddcci`) built on the library:

```shell
go run ./cmd/ddcci list            # list monitors (description, EDID model, adapter, device key, DDC/CI status)
go run ./cmd/ddcci caps            # print raw MCCS capabilities string, plus parsed output
go run ./cmd/ddcci get 0x10        # read VCP 0x10 (brightness)
go run ./cmd/ddcci set 0x10 75     # write VCP 0x10
go run ./cmd/ddcci scan            # dump vendor VCP range 0xE0-0xFF (read-only)
go run ./cmd/ddcci power hardoff   # write Power Mode (0xD6)
```

Every command accepts `-i N` to select a monitor by index (default 0) and `-format=json` (or `--format=json`) for machine-readable output, e.g. `go run ./cmd/ddcci get -format=json 0x10`. Flags must precede positional arguments (Go's standard `flag` package semantics).

## FAQ
### What operating systems does DDCCI support?
- Now only Windows. Welcome PR to support other OS.

## Development
- `go test ./...` runs unit tests only.
- `go test ./...` runs everything, including tests that talk to the attached monitor
  (brightness/contrast sweeps, screen off/on; unsupported VCP codes are skipped).
- The factory-reset test is gated behind the `destructive` build tag:
  `go test -tags destructive ./...` (this restores the monitor to factory defaults!)
- Monitor handles must be closed with `(*PhysicalMonitor).Close()` after use;
  `NewPhysicalMonitors` returns every physical monitor behind an HMONITOR.
- Capabilities strings can be parsed with `ParseCapabilities`.
- After the computer wakes from sleep, monitor handles may become stale; re-create them.

## Reference
- [ddcutil Documentation](https://www.ddcutil.com/vcpinfo_output/)
- [Monitor Configuration | Microsoft Learn](https://learn.microsoft.com/en-us/windows/win32/api/_monitor/)

## Credits
- [goland](https://www.jetbrains.com/go/)