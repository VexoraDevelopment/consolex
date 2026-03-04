package main

import (
	"log/slog"

	"github.com/VexoraDevelopment/consolex"
)

func main() {
	profile := consolex.DefaultProfile()
	profile.HideKeys["name"] = false
	profile.HideKeys["dimension"] = false

	logFile, err := consolex.SetupDefaultSlog(consolex.LoggerConfig{
		LogFilePath: "server.log",
		ArchiveDir:  "logs",
		Level:       slog.LevelDebug,
		Theme:       consolex.DefaultTheme(),
		Profile:     profile,
		Dedupe: consolex.DedupeConfig{
			Enabled: true,
			Remap: []consolex.LevelRemapRule{
				{
					From:     "INFO",
					To:       "DEBUG",
					Contains: []string{"conn write packet failed", "context canceled"},
				},
			},
		},
		Processors: []consolex.Processor{
			consolex.ProcessorFunc(func(rec *consolex.LogRecord) {
				// Example: force showing key for world pointer fields in compact layout.
				for i := range rec.Fields {
					if rec.Fields[i].Key == "ptr" {
						rec.Fields[i].ShowKey = true
					}
				}
			}),
		},
		FieldProvider: consolex.FieldStyleFunc(func(key, value string) (consolex.Chalk, bool) {
			switch key {
			case "name":
				return consolex.New().Black().BgYellow(), true
			case "ptr":
				return consolex.New().White().BgMagenta(), true
			default:
				return consolex.Chalk{}, false
			}
		}),
	})
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	slog.Info("world register", "name", "hub_snow", "ptr", "0xc004ba0ea0")
	for i := 0; i < 10; i++ {
		slog.Info("conn write packet failed", "packet", "*packet.UpdateBlock", "err", "context canceled")
	}
}
