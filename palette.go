package consolex

import "fmt"

type Palette struct {
	Theme Theme
}

func NewPalette(theme Theme) Palette {
	return Palette{Theme: theme}
}

func DefaultPalette() Palette { return NewPalette(DefaultTheme()) }
func NordPalette() Palette    { return NewPalette(NordTheme()) }
func SunsetPalette() Palette  { return NewPalette(SunsetTheme()) }

func (p Palette) Success(v ...any) string { return p.Theme.Info.Bold().Sprint(v...) }
func (p Palette) Info(v ...any) string    { return p.Theme.Info.Sprint(v...) }
func (p Palette) Warn(v ...any) string    { return p.Theme.Warn.Bold().Sprint(v...) }
func (p Palette) Error(v ...any) string   { return p.Theme.Error.Bold().Sprint(v...) }
func (p Palette) Debug(v ...any) string   { return p.Theme.Debug.Sprint(v...) }
func (p Palette) Muted(v ...any) string   { return p.Theme.TimeKey.Sprint(v...) }

func (p Palette) KV(key string, value any) string {
	return p.Theme.MsgKey.Wrap(key) + "=" + p.Theme.TimeValue.Wrap(fmt.Sprint(value))
}
