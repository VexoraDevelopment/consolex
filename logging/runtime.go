package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/VexoraDevelopment/consolex/style"
	"github.com/VexoraDevelopment/consolex/term"
)

const LevelTrace slog.Level = slog.LevelDebug - 4

const (
	consoleLevelWidth = 5
	consoleScopeWidth = 9
)

type ConsoleAttrsFunc func(level slog.Level, component string, attrs []slog.Attr) []slog.Attr

type SlogConfig struct {
	Level            slog.Level
	ComponentLevels  map[string]slog.Level
	ComponentKey     string
	DefaultComponent string
	Theme            style.Theme
	Profile          Profile
	Console          io.Writer
	FilePath         string
	FileQueueSize    int
	ConsoleAttrs     ConsoleAttrsFunc
}

type SlogRuntime struct {
	file *asyncLineWriter
}

func (r *SlogRuntime) Close() error {
	if r == nil || r.file == nil {
		return nil
	}
	return r.file.Close()
}

func (r *SlogRuntime) Dropped() uint64 {
	if r == nil || r.file == nil {
		return 0
	}
	return r.file.dropped.Load()
}

func OpenSlog(cfg SlogConfig) (*SlogRuntime, error) {
	term.EnableConsoleANSI()
	if cfg.ComponentKey == "" {
		cfg.ComponentKey = "component"
	}
	if cfg.DefaultComponent == "" {
		cfg.DefaultComponent = "server"
	}
	if cfg.Console == nil {
		cfg.Console = os.Stdout
	}
	if cfg.Theme.TimeKey.Wrap("x") == "x" {
		cfg.Theme = style.DefaultTheme()
	}
	profile := normalizeProfile(cfg.Profile)
	policy := newComponentPolicy(cfg.Level, cfg.ComponentLevels, cfg.ComponentKey, cfg.DefaultComponent)
	console := &minecraftHandler{out: cfg.Console, level: policy.minimum, theme: cfg.Theme, profile: profile, componentKey: cfg.ComponentKey, defaultComponent: cfg.DefaultComponent, attrsFn: cfg.ConsoleAttrs, mu: &sync.Mutex{}}
	var handlers []slog.Handler
	handlers = append(handlers, console)
	runtime := &SlogRuntime{}
	if strings.TrimSpace(cfg.FilePath) != "" {
		if err := os.MkdirAll(filepath.Dir(cfg.FilePath), 0o755); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		queue := cfg.FileQueueSize
		if queue <= 0 {
			queue = 4096
		}
		async := newAsyncLineWriter(file, queue)
		async.recovered = func(dropped uint64) {
			rec := slog.NewRecord(time.Now(), slog.LevelWarn, "File log records dropped", 0)
			rec.AddAttrs(slog.String(cfg.ComponentKey, "logging"), slog.Uint64("dropped", dropped))
			_ = console.Handle(context.Background(), rec)
		}
		runtime.file = async
		handlers = append(handlers, slog.NewJSONHandler(async, &slog.HandlerOptions{Level: policy.minimum}))
	}
	slog.SetDefault(slog.New(&componentPolicyHandler{next: fanoutHandler{handlers: handlers}, policy: policy}))
	return runtime, nil
}

type componentPolicy struct {
	fallback, minimum     slog.Level
	key, defaultComponent string
	levels                map[string]slog.Level
}

func newComponentPolicy(fallback slog.Level, levels map[string]slog.Level, key, defaultComponent string) componentPolicy {
	p := componentPolicy{fallback: fallback, minimum: fallback, key: key, defaultComponent: defaultComponent, levels: make(map[string]slog.Level, len(levels))}
	for component, level := range levels {
		component = strings.ToLower(strings.TrimSpace(component))
		if component == "" {
			continue
		}
		p.levels[component] = level
		if level < p.minimum {
			p.minimum = level
		}
	}
	return p
}

func (p componentPolicy) enabled(component string, level slog.Level) bool {
	if configured, ok := p.levels[strings.ToLower(component)]; ok {
		return level >= configured
	}
	return level >= p.fallback
}

type componentPolicyHandler struct {
	next   slog.Handler
	policy componentPolicy
	attrs  []slog.Attr
}

func (h *componentPolicyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if component := stringAttr(h.attrs, h.policy.key); component != "" {
		return h.policy.enabled(component, level)
	}
	return level >= h.policy.minimum
}

func (h *componentPolicyHandler) Handle(ctx context.Context, rec slog.Record) error {
	component := stringAttr(h.attrs, h.policy.key)
	if component == "" {
		component = recordStringAttr(rec, h.policy.key)
	}
	if component == "" {
		component = h.policy.defaultComponent
		rec.AddAttrs(slog.String(h.policy.key, component))
	}
	if !h.policy.enabled(component, rec.Level) {
		return nil
	}
	return h.next.Handle(ctx, rec)
}

func (h *componentPolicyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	all := append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &componentPolicyHandler{next: h.next.WithAttrs(attrs), policy: h.policy, attrs: all}
}
func (h *componentPolicyHandler) WithGroup(name string) slog.Handler {
	return &componentPolicyHandler{next: h.next.WithGroup(name), policy: h.policy, attrs: h.attrs}
}

type minecraftHandler struct {
	out                            io.Writer
	level                          slog.Leveler
	theme                          style.Theme
	profile                        Profile
	componentKey, defaultComponent string
	attrs                          []slog.Attr
	groups                         []string
	attrsFn                        ConsoleAttrsFunc
	mu                             *sync.Mutex
}

func (h *minecraftHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.level.Level()
}

func (h *minecraftHandler) Handle(_ context.Context, rec slog.Record) error {
	attrs := append([]slog.Attr(nil), h.attrs...)
	rec.Attrs(func(attr slog.Attr) bool { attrs = append(attrs, attr); return true })
	component := stringAttr(attrs, h.componentKey)
	if component == "" {
		component = h.defaultComponent
	}
	if h.attrsFn != nil {
		attrs = h.attrsFn(rec.Level, component, attrs)
	}
	fields := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		appendRenderedAttr(&fields, attr, h.componentKey, h.profile, h.theme)
	}
	timestamp := rec.Time.Format("15:04:05")
	level := levelLabel(rec.Level)
	line := fmt.Sprintf("[%s] %s%s%s%s%s",
		h.theme.TimeValue.Dim().Wrap(timestamp),
		bracketed(levelStyle(h.theme, rec.Level).Wrap(level)), visiblePadding(level, consoleLevelWidth),
		bracketed(h.theme.MsgKey.Wrap(component)), visiblePadding(component, consoleScopeWidth),
		rec.Message,
	)
	if len(fields) != 0 {
		line += " " + strings.Join(fields, " ")
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.out, line+"\n")
	return err
}

func bracketed(value string) string { return "[" + value + "]" }

func visiblePadding(value string, width int) string {
	visible := utf8.RuneCountInString(style.StripANSI(value))
	return strings.Repeat(" ", max(1, width-visible+1))
}

func (h *minecraftHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	all := append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &minecraftHandler{out: h.out, level: h.level, theme: h.theme, profile: h.profile, componentKey: h.componentKey, defaultComponent: h.defaultComponent, attrs: all, groups: h.groups, attrsFn: h.attrsFn, mu: h.mu}
}
func (h *minecraftHandler) WithGroup(name string) slog.Handler {
	groups := append(append([]string(nil), h.groups...), name)
	clone := *h
	clone.groups = groups
	return &clone
}

func appendRenderedAttr(out *[]string, attr slog.Attr, componentKey string, profile Profile, theme style.Theme) {
	attr.Value = attr.Value.Resolve()
	if attr.Key == componentKey || attr.Key == "event" {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		for _, child := range attr.Value.Group() {
			appendRenderedAttr(out, child, componentKey, profile, theme)
		}
		return
	}
	value := fmt.Sprint(attr.Value.Any())
	if attr.Key == "err" || attr.Key == "error" {
		value = theme.ErrKey.Wrap(value)
	}
	if hidden, ok := profile.HideKeys[attr.Key]; ok && hidden {
		*out = append(*out, value)
		return
	}
	*out = append(*out, attr.Key+"="+value)
}

func levelStyle(theme style.Theme, level slog.Level) style.Chalk {
	switch {
	case level <= slog.LevelDebug:
		return theme.Debug
	case level < slog.LevelWarn:
		return theme.Info
	case level < slog.LevelError:
		return theme.Warn
	default:
		return theme.Error
	}
}
func levelLabel(level slog.Level) string {
	switch {
	case level <= LevelTrace:
		return "TRACE"
	case level <= slog.LevelDebug:
		return "DEBUG"
	case level < slog.LevelWarn:
		return "INFO"
	case level < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}
func stringAttr(attrs []slog.Attr, key string) string {
	for _, attr := range attrs {
		if attr.Key == key && attr.Value.Kind() == slog.KindString {
			return attr.Value.String()
		}
	}
	return ""
}
func recordStringAttr(rec slog.Record, key string) string {
	var value string
	rec.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key && attr.Value.Kind() == slog.KindString {
			value = attr.Value.String()
			return false
		}
		return true
	})
	return value
}

type asyncLineWriter struct {
	file      *os.File
	queue     chan []byte
	done      chan struct{}
	dropped   atomic.Uint64
	pending   atomic.Uint64
	recovered func(uint64)
	once      sync.Once
	errMu     sync.Mutex
	err       error
}

func newAsyncLineWriter(file *os.File, size int) *asyncLineWriter {
	w := &asyncLineWriter{file: file, queue: make(chan []byte, size), done: make(chan struct{})}
	go func() {
		defer close(w.done)
		for line := range w.queue {
			if pending := w.pending.Swap(0); pending != 0 && w.recovered != nil {
				w.recovered(pending)
			}
			if _, err := w.file.Write(line); err != nil {
				w.errMu.Lock()
				if w.err == nil {
					w.err = err
				}
				w.errMu.Unlock()
			}
		}
	}()
	return w
}

func (w *asyncLineWriter) Write(data []byte) (int, error) {
	line := append([]byte(nil), data...)
	select {
	case w.queue <- line:
		return len(data), nil
	default:
		w.dropped.Add(1)
		w.pending.Add(1)
		return len(data), nil
	}
}

func (w *asyncLineWriter) Close() error {
	w.once.Do(func() {
		close(w.queue)
		<-w.done
		if err := w.file.Close(); err != nil {
			w.errMu.Lock()
			if w.err == nil {
				w.err = err
			}
			w.errMu.Unlock()
		}
	})
	w.errMu.Lock()
	defer w.errMu.Unlock()
	return w.err
}
