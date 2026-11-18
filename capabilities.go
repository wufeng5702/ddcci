package ddcci

import "strings"

// Capabilities is a parsed MCCS capabilities string, e.g.
//
//	(prot(monitor)type(lcd)cmds(01 02)vcp(10(05 08) D6(01 04 05))mccs_ver(2.1))
//
// Only what the library needs is kept: panel type, model, MCCS version and the
// set of supported VCP codes with their allowed values. For continuous controls
// the value slice is nil.
type Capabilities struct {
	Raw     string `json:"raw"`
	Type    string `json:"type,omitempty"`     // e.g. "lcd"
	Model   string `json:"model,omitempty"`    // may be empty
	MCCSVer string `json:"mccs_ver,omitempty"` // e.g. "2.1"

	// Features maps each supported VCP code to its list of allowed values,
	// or nil for continuous controls.
	Features map[uint8][]int `json:"features"`
}

// Has reports whether the monitor supports the given VCP code.
func (c *Capabilities) Has(code int) bool {
	if code < 0 || code > 255 {
		return false
	}
	_, ok := c.Features[uint8(code)]
	return ok
}

// Values returns the allowed values reported for the given VCP code.
// The slice is nil for continuous controls; the second return value
// reports whether the code is supported at all.
func (c *Capabilities) Values(code int) ([]int, bool) {
	if code < 0 || code > 255 {
		return nil, false
	}
	v, ok := c.Features[uint8(code)]
	return v, ok
}

// ParseCapabilities parses an MCCS capabilities string.
//
// The parser is forgiving: it never returns nil and skips tokens it does not
// know about (unknown groups and stray bytes are skipped), so a firmware that
// publishes a marker fused with another key (e.g. `cmds(...)`) does not lose
// the remaining tokens.
func ParseCapabilities(raw string) *Capabilities {
	c := &Capabilities{Raw: strings.TrimRight(raw, "\x00"), Features: map[uint8][]int{}}
	s := strings.TrimSpace(c.Raw)
	s = strings.TrimSuffix(strings.TrimPrefix(s, "("), ")")

	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			return c
		}

		key, rest := splitToken(s)
		if key == "" {
			s = s[1:] // skip stray byte; s is non-empty here
			continue
		}

		if !hasGroup(rest) {
			s = rest
			continue
		}
		group, after := takeGroup(rest)

		switch key {
		case "type":
			c.Type = group
		case "model":
			c.Model = group
		case "mccs_ver":
			c.MCCSVer = group
		case "vcp":
			parseFeatures(group, c.Features)
		}
		s = after
	}
}

func splitToken(s string) (token, rest string) {
	i := 0
	for i < len(s) && isTokenByte(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

func isTokenByte(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

func hasGroup(s string) bool {
	s = strings.TrimLeft(s, " \t\r\n")
	return strings.HasPrefix(s, "(")
}

// takeGroup reads the parenthesized group at the start of s and returns
// its inner text plus the remainder after the matching ')'.
// s must start with '(' (ignoring leading whitespace).
func takeGroup(s string) (string, string) {
	i := strings.Index(s, "(")
	depth := 0
	for j := i; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return s[i+1 : j], s[j+1:]
			}
		}
	}
	return s[i+1:], ""
}

func parseFeatures(s string, out map[uint8][]int) {
	for {
		s = strings.TrimLeft(s, " \t\r\n")
		if s == "" {
			return
		}
		if !isHex(s[0]) {
			s = s[1:] // skip unexpected byte
			continue
		}

		alpha, rest := spanHex(s)
		code := parseHexUint8(alpha)
		s = strings.TrimLeft(rest, " \t\r\n")

		var values []int
		if strings.HasPrefix(s, "(") {
			var inner string
			inner, s = takeGroup(s)
			values = parseHexList(inner)
		}
		out[code] = values
	}
}

// spanHex consumes the leading hex-digit run of s.
func spanHex(s string) (run, rest string) {
	i := 0
	for i < len(s) && isHex(s[i]) {
		i++
	}
	return s[:i], s[i:]
}

func isHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func parseHexUint8(s string) uint8 {
	v := 0
	for i := range len(s) {
		var digit int
		switch {
		case s[i] >= '0' && s[i] <= '9':
			digit = int(s[i] - '0')
		case s[i] >= 'a' && s[i] <= 'f':
			digit = int(s[i]-'a') + 10
		default:
			digit = int(s[i]-'A') + 10
		}
		v = v*16 + digit
	}
	return uint8(v)
}

func parseHexList(s string) []int {
	fields := strings.Fields(s)
	out := make([]int, 0, len(fields))
	for _, field := range fields {
		v := 0
		for i := range len(field) {
			var digit int
			switch {
			case field[i] >= '0' && field[i] <= '9':
				digit = int(field[i] - '0')
			case field[i] >= 'a' && field[i] <= 'f':
				digit = int(field[i]-'a') + 10
			default:
				digit = int(field[i]-'A') + 10
			}
			v = v*16 + digit
		}
		out = append(out, v)
	}
	return out
}
