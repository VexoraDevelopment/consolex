package style

type Theme struct {
	TimeKey   Chalk
	TimeValue Chalk
	MsgKey    Chalk
	Debug     Chalk
	Info      Chalk
	Warn      Chalk
	Error     Chalk
	ErrKey    Chalk
	PlayerKey Chalk
	WorldKey  Chalk
}

func DefaultTheme() Theme {
	return Theme{
		TimeKey:   New().Gray(),
		TimeValue: New().BrightBlue(),
		MsgKey:    New().BrightWhite(),
		Debug:     New().Black().BgCyan().Bold(),
		Info:      New().Black().BgHex("#40E0D0").Bold(),
		Warn:      New().Black().BgYellow().Bold(),
		Error:     New().White().BgRed().Bold(),
		ErrKey:    New().White().BgRed().Bold(),
		PlayerKey: New().BrightCyan(),
		WorldKey:  New().Magenta(),
	}
}

func NordTheme() Theme {
	return Theme{
		TimeKey:   New().Hex("#81A1C1"),
		TimeValue: New().Hex("#88C0D0"),
		MsgKey:    New().Hex("#ECEFF4").Bold(),
		Debug:     New().Black().BgHex("#88C0D0").Bold(),
		Info:      New().Black().BgHex("#8FBCBB").Bold(),
		Warn:      New().Black().BgHex("#EBCB8B").Bold(),
		Error:     New().Hex("#ECEFF4").BgHex("#BF616A").Bold(),
		ErrKey:    New().Hex("#ECEFF4").BgHex("#D08770").Bold(),
		PlayerKey: New().Hex("#B48EAD"),
		WorldKey:  New().Hex("#5E81AC"),
	}
}

func SunsetTheme() Theme {
	return Theme{
		TimeKey:   New().Hex("#F8C8DC"),
		TimeValue: New().Hex("#F4A261"),
		MsgKey:    New().Hex("#FFF1E6"),
		Debug:     New().Black().BgHex("#9BF6FF").Bold(),
		Info:      New().Black().BgHex("#70E0C0").Bold(),
		Warn:      New().Black().BgHex("#FFD166").Bold(),
		Error:     New().Hex("#FFF1E6").BgHex("#FF6B6B").Bold(),
		ErrKey:    New().Hex("#FFF1E6").BgHex("#FF8FA3").Bold(),
		PlayerKey: New().Hex("#CDB4DB"),
		WorldKey:  New().Hex("#A0C4FF"),
	}
}
