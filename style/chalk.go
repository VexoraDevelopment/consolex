package style

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Chalk struct {
	enabled bool
	codes   []string
}

func New() Chalk {
	return Chalk{enabled: true}
}

func Disabled() Chalk {
	return Chalk{enabled: false}
}

func (c Chalk) WithEnabled(enabled bool) Chalk {
	c.enabled = enabled
	return c
}

func (c Chalk) cloneWith(code string) Chalk {
	out := Chalk{enabled: c.enabled, codes: make([]string, 0, len(c.codes)+1)}
	out.codes = append(out.codes, c.codes...)
	out.codes = append(out.codes, code)
	return out
}

func (c Chalk) code(v int) Chalk { return c.cloneWith(strconv.Itoa(v)) }

func (c Chalk) Bold() Chalk          { return c.code(1) }
func (c Chalk) Dim() Chalk           { return c.code(2) }
func (c Chalk) Italic() Chalk        { return c.code(3) }
func (c Chalk) Underline() Chalk     { return c.code(4) }
func (c Chalk) Inverse() Chalk       { return c.code(7) }
func (c Chalk) Strikethrough() Chalk { return c.code(9) }

func (c Chalk) Black() Chalk   { return c.code(30) }
func (c Chalk) Red() Chalk     { return c.code(31) }
func (c Chalk) Green() Chalk   { return c.code(32) }
func (c Chalk) Yellow() Chalk  { return c.code(33) }
func (c Chalk) Blue() Chalk    { return c.code(34) }
func (c Chalk) Magenta() Chalk { return c.code(35) }
func (c Chalk) Cyan() Chalk    { return c.code(36) }
func (c Chalk) White() Chalk   { return c.code(37) }
func (c Chalk) Gray() Chalk    { return c.code(90) }

func (c Chalk) BrightBlack() Chalk   { return c.code(90) }
func (c Chalk) BrightRed() Chalk     { return c.code(91) }
func (c Chalk) BrightGreen() Chalk   { return c.code(92) }
func (c Chalk) BrightYellow() Chalk  { return c.code(93) }
func (c Chalk) BrightBlue() Chalk    { return c.code(94) }
func (c Chalk) BrightMagenta() Chalk { return c.code(95) }
func (c Chalk) BrightCyan() Chalk    { return c.code(96) }
func (c Chalk) BrightWhite() Chalk   { return c.code(97) }

func (c Chalk) BgBlack() Chalk   { return c.code(40) }
func (c Chalk) BgRed() Chalk     { return c.code(41) }
func (c Chalk) BgGreen() Chalk   { return c.code(42) }
func (c Chalk) BgYellow() Chalk  { return c.code(43) }
func (c Chalk) BgBlue() Chalk    { return c.code(44) }
func (c Chalk) BgMagenta() Chalk { return c.code(45) }
func (c Chalk) BgCyan() Chalk    { return c.code(46) }
func (c Chalk) BgWhite() Chalk   { return c.code(47) }

func (c Chalk) RGB(r, g, b uint8) Chalk {
	return c.cloneWith(fmt.Sprintf("38;2;%d;%d;%d", r, g, b))
}

func (c Chalk) BgRGB(r, g, b uint8) Chalk {
	return c.cloneWith(fmt.Sprintf("48;2;%d;%d;%d", r, g, b))
}

func (c Chalk) Hex(hex string) Chalk {
	if r, g, b, ok := parseHexColor(hex); ok {
		return c.RGB(r, g, b)
	}
	return c
}

func (c Chalk) BgHex(hex string) Chalk {
	if r, g, b, ok := parseHexColor(hex); ok {
		return c.BgRGB(r, g, b)
	}
	return c
}

func (c Chalk) Wrap(text string) string {
	if !c.enabled || len(c.codes) == 0 || text == "" {
		return text
	}
	return "\x1b[" + strings.Join(c.codes, ";") + "m" + text + "\x1b[0m"
}

func (c Chalk) Sprint(v ...any) string {
	return c.Wrap(fmt.Sprint(v...))
}

func (c Chalk) Sprintf(format string, v ...any) string {
	return c.Wrap(fmt.Sprintf(format, v...))
}

var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func StripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func parseHexColor(hex string) (uint8, uint8, uint8, bool) {
	h := strings.TrimSpace(strings.TrimPrefix(hex, "#"))
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return 0, 0, 0, false
	}
	return uint8(v >> 16), uint8((v >> 8) & 0xff), uint8(v & 0xff), true
}
