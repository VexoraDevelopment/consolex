# consolex

Small standalone console toolkit:
- colored `slog` to terminal + file
- log rotation/compression
- interactive readline loop with autocomplete
- built-in commands: `help`, `clear/cls`, `uptime`, `pid`, `echo`
- fluent chalk-like styling API
- prebuilt themes and palette helpers
- pipeline logging architecture (`parse -> processors -> render`)

## Minimal usage

```go
package main

import (
	"log/slog"

	"github.com/VexoraDevelopment/consolex"
)

func main() {
logFile, err := consolex.SetupDefaultSlog(consolex.LoggerConfig{
	LogFilePath: "server.log",
	ArchiveDir:  "logs",
	Level:       slog.LevelDebug,
	Theme:       consolex.NordTheme(),
	FieldProvider: consolex.StaticFieldProvider{
		"proto_id": consolex.New().White().BgBlue().Bold(),
		"raddr":    consolex.New().Black().BgYellow(),
	},
	FieldTransform: consolex.FieldTransformFunc(func(key, value string) (string, bool) {
		if key == "raddr" {
			return "\"***hidden***\"", true
		}
		return "", false
	}),
})
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	loop := consolex.NewLoop(consolex.Options{
		Resolve: func(name, args string) bool {
			// handle your custom commands here
			return false
		},
	})
	<-loop.Start()
}
```

## Chalk-like style API

```go
ch := consolex.New()
println(ch.Bold().BrightGreen().Sprint("server online"))
println(ch.Hex("#66ccff").Underline().Sprint("hello"))
println(ch.BgHex("#202020").White().Sprint("with background"))
```

## Preset palettes

```go
p := consolex.DefaultPalette()
println(p.Success("ok"))
println(p.Warn("careful"))
println(p.Error("boom"))
println(p.KV("proto", 924))
```

## Structure

- `consolex/style`: chalk API + themes
- `consolex/logging`: colored slog + rotation + pipeline
- `consolex/cmdline`: interactive command loop/autocomplete
- `consolex/term`: terminal ANSI helpers
- `consolex` root: facade API for easy import
- `consolex/examples/basic`: minimal integration example
- `consolex/examples/pipeline`: pipeline/profile/processors example

## Logging Pipeline

`consolex/logging` now uses `LogRecord` pipeline:

1. Parse raw slog text line to structured `LogRecord`.
2. Run processors (`FieldTransform`, `FieldProvider`, extra custom processors).
3. Render record with renderer (`defaultRenderer` by default).

You can inject custom processors and renderer via `LoggerConfig`:

```go
cfg := consolex.LoggerConfig{
	Theme:      consolex.NordTheme(),
	Profile:    consolex.DefaultProfile(),
	Processors: []consolex.Processor{
		consolex.ProcessorFunc(func(rec *consolex.LogRecord) {
			// custom mutation logic
		}),
	},
	Renderer: consolex.RendererFunc(func(rec *consolex.LogRecord) string {
		// custom final formatting
		return rec.Raw
	}),
}
```
