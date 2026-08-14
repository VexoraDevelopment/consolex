package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/VexoraDevelopment/consolex/style"
)

func TestMinecraftHandler(t *testing.T) {
	var out bytes.Buffer
	h := &minecraftHandler{out: &out, level: slog.LevelDebug, theme: disabledTheme(), profile: DefaultProfile(), componentKey: "component", defaultComponent: "server", mu: new(sync.Mutex)}
	rec := slog.NewRecord(time.Date(2026, 8, 14, 19, 27, 41, 0, time.Local), slog.LevelInfo, "Starting Nucleus dev-82fd391", 0)
	rec.AddAttrs(slog.String("component", "server"), slog.String("event", "server.starting"), slog.String("version", "dev-82fd391"))
	if err := h.Handle(context.Background(), rec); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "[19:27:41] [INFO]  [server]    Starting Nucleus dev-82fd391 version=dev-82fd391\n"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestMinecraftHandlerAlignsMessagesByVisibleWidth(t *testing.T) {
	var out bytes.Buffer
	theme := style.DefaultTheme()
	h := &minecraftHandler{out: &out, level: LevelTrace, theme: theme, profile: DefaultProfile(), componentKey: "component", defaultComponent: "server", mu: new(sync.Mutex)}
	for _, test := range []struct {
		level     slog.Level
		component string
		message   string
	}{
		{slog.LevelInfo, "world", "Loading world Overworld"},
		{slog.LevelWarn, "plugin", "Callback took 482ms"},
		{slog.LevelError, "storage", "Failed to save chunk"},
		{LevelTrace, "scheduler", "Task dispatched"},
	} {
		rec := slog.NewRecord(time.Date(2026, 8, 14, 21, 3, 30, 0, time.Local), test.level, test.message, 0)
		rec.AddAttrs(slog.String("component", test.component))
		if err := h.Handle(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range strings.Split(strings.TrimSpace(style.StripANSI(out.String())), "\n") {
		if got := strings.Index(line, strings.Fields(line)[3]); got != 31 {
			t.Fatalf("message column = %d in %q, want 31", got, line)
		}
	}
}

func TestComponentPolicyWithBoundComponent(t *testing.T) {
	policy := newComponentPolicy(slog.LevelInfo, map[string]slog.Level{"entity": LevelTrace}, "component", "server")
	h := (&componentPolicyHandler{next: slog.NewTextHandler(&bytes.Buffer{}, nil), policy: policy}).WithAttrs([]slog.Attr{slog.String("component", "entity")})
	if !h.Enabled(context.Background(), LevelTrace) {
		t.Fatal("entity trace disabled")
	}
	server := (&componentPolicyHandler{next: slog.NewTextHandler(&bytes.Buffer{}, nil), policy: policy}).WithAttrs([]slog.Attr{slog.String("component", "server")})
	if server.Enabled(context.Background(), slog.LevelDebug) {
		t.Fatal("default debug enabled")
	}
}

func TestOpenSlogWritesJSONL(t *testing.T) {
	old := slog.Default()
	t.Cleanup(func() { slog.SetDefault(old) })
	var console bytes.Buffer
	path := filepath.Join(t.TempDir(), "server.jsonl")
	runtime, err := OpenSlog(SlogConfig{Level: slog.LevelInfo, Theme: disabledTheme(), Console: &console, FilePath: path, FileQueueSize: 8})
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("Ready", "component", "server", "event", "server.ready", "runtime_id", uint64(42))
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(data), &record); err != nil {
		t.Fatal(err)
	}
	for key := range map[string]struct{}{"component": {}, "event": {}, "runtime_id": {}} {
		if _, ok := record[key]; !ok {
			t.Fatalf("missing %s in %s", key, data)
		}
	}
	if strings.Contains(console.String(), "event=") {
		t.Fatalf("event leaked: %q", console.String())
	}
}

func disabledTheme() style.Theme {
	theme := style.DefaultTheme()
	disabled := style.Disabled()
	theme.TimeKey, theme.TimeValue, theme.MsgKey = disabled, disabled, disabled
	theme.Debug, theme.Info, theme.Warn, theme.Error, theme.ErrKey = disabled, disabled, disabled, disabled, disabled
	return theme
}
