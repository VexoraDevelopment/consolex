package main

import (
	"log/slog"
	"strings"

	"github.com/VexoraDevelopment/consolex"
)

func main() {
	logFile, err := consolex.SetupDefaultSlog(consolex.LoggerConfig{
		LogFilePath: "server.log",
		ArchiveDir:  "logs",
		Level:       slog.LevelDebug,
		Theme:       consolex.NordTheme(),
		FieldProvider: consolex.StaticFieldProvider{
			"addr":     consolex.New().Black().BgCyan().Bold(),
			"proto_id": consolex.New().White().BgBlue().Bold(),
		},
		FieldTransform: consolex.FieldTransformFunc(func(key, value string) (string, bool) {
			if key == "dimension" {
				return "\"" + strings.ToUpper(strings.Trim(value, "\"")) + "\"", true
			}
			return "", false
		}),
	})
	if err != nil {
		panic(err)
	}
	defer logFile.Close()

	slog.Info("Listener running.", "addr", "[::]:19132")
	slog.Debug("Loading dimension...", "dimension", "overworld")
}
