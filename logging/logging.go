package logging

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/VexoraDevelopment/consolex/style"
	"github.com/VexoraDevelopment/consolex/term"
)

type RecordField struct {
	Key      string
	Value    string
	ValueOut string
	ShowKey  bool
	Styled   bool
	Style    style.Chalk
}

type LogRecord struct {
	Raw     string
	Time    string
	Level   string
	Message string
	Fields  []RecordField
}

type Profile struct {
	HideKeys    map[string]bool
	LevelLabels map[string]string
	CompactMode bool
}

func DefaultProfile() Profile {
	return Profile{
		HideKeys: map[string]bool{
			"time":   true,
			"level":  true,
			"msg":    true,
			"err":    true,
			"error":  true,
			"player": true,
			"world":  true,
		},
		LevelLabels: map[string]string{
			"DEBUG": "DBG",
			"INFO":  "INF",
			"WARN":  "WRN",
			"ERROR": "ERR",
		},
		CompactMode: true,
	}
}

func normalizeProfile(p Profile) Profile {
	d := DefaultProfile()
	if p.HideKeys == nil {
		p.HideKeys = d.HideKeys
	}
	if p.LevelLabels == nil {
		p.LevelLabels = d.LevelLabels
	}
	if !p.CompactMode {
		p.CompactMode = d.CompactMode
	}
	return p
}

type Processor interface {
	Process(rec *LogRecord)
}

type ProcessorFunc func(rec *LogRecord)

func (f ProcessorFunc) Process(rec *LogRecord) {
	f(rec)
}

type Renderer interface {
	Render(rec *LogRecord) string
}

type RendererFunc func(rec *LogRecord) string

func (f RendererFunc) Render(rec *LogRecord) string {
	return f(rec)
}

type FieldStyleProvider interface {
	StyleField(key, value string) (style.Chalk, bool)
}

type FieldTransformer interface {
	TransformField(key, value string) (string, bool)
}

type FieldStyleFunc func(key, value string) (style.Chalk, bool)

func (f FieldStyleFunc) StyleField(key, value string) (style.Chalk, bool) {
	return f(key, value)
}

type FieldTransformFunc func(key, value string) (string, bool)

func (f FieldTransformFunc) TransformField(key, value string) (string, bool) {
	return f(key, value)
}

type StaticFieldProvider map[string]style.Chalk

func (p StaticFieldProvider) StyleField(key, _ string) (style.Chalk, bool) {
	st, ok := p[key]
	return st, ok
}

type fieldTransformProcessor struct {
	transformer FieldTransformer
}

func (p fieldTransformProcessor) Process(rec *LogRecord) {
	if p.transformer == nil {
		return
	}
	for i := range rec.Fields {
		if out, ok := p.transformer.TransformField(rec.Fields[i].Key, rec.Fields[i].Value); ok {
			rec.Fields[i].ValueOut = out
		}
	}
}

type fieldStyleProcessor struct {
	provider FieldStyleProvider
}

func (p fieldStyleProcessor) Process(rec *LogRecord) {
	if p.provider == nil {
		return
	}
	for i := range rec.Fields {
		value := rec.Fields[i].Value
		if rec.Fields[i].ValueOut != "" {
			value = rec.Fields[i].ValueOut
		}
		if st, ok := p.provider.StyleField(rec.Fields[i].Key, value); ok {
			rec.Fields[i].Style = st
			rec.Fields[i].Styled = true
		}
	}
}

type errorFieldProcessor struct {
	errStyle style.Chalk
}

func (p errorFieldProcessor) Process(rec *LogRecord) {
	for i := range rec.Fields {
		if rec.Fields[i].Styled {
			continue
		}
		if rec.Fields[i].Key == "err" || rec.Fields[i].Key == "error" {
			rec.Fields[i].Style = p.errStyle
			rec.Fields[i].Styled = true
		}
	}
}

type Pipeline struct {
	processors []Processor
	renderer   Renderer
}

func NewPipeline(theme style.Theme, profile Profile, provider FieldStyleProvider, transformer FieldTransformer, extras []Processor, renderer Renderer) *Pipeline {
	processors := []Processor{
		fieldTransformProcessor{transformer: transformer},
		fieldStyleProcessor{provider: provider},
		errorFieldProcessor{errStyle: theme.ErrKey},
	}
	processors = append(processors, extras...)
	if renderer == nil {
		renderer = newDefaultRenderer(theme, profile)
	}
	return &Pipeline{processors: processors, renderer: renderer}
}

func (p *Pipeline) Colorize(line string) string {
	rec := ParseTextLogLine(line)
	for _, proc := range p.processors {
		proc.Process(rec)
	}
	return p.renderer.Render(rec)
}

type defaultRenderer struct {
	theme   style.Theme
	profile Profile
}

func newDefaultRenderer(theme style.Theme, profile Profile) Renderer {
	return defaultRenderer{theme: theme, profile: normalizeProfile(profile)}
}

func (r defaultRenderer) Render(rec *LogRecord) string {
	parts := make([]string, 0, 3+len(rec.Fields))
	if rec.Time != "" {
		parts = append(parts, r.theme.TimeValue.Dim().Wrap(rec.Time))
	}
	if rec.Level != "" {
		parts = append(parts, r.levelBadge(rec.Level))
	}
	if rec.Message != "" {
		parts = append(parts, rec.Message)
	}
	for _, f := range rec.Fields {
		value := f.Value
		if f.ValueOut != "" {
			value = f.ValueOut
		}
		if f.Styled {
			value = f.Style.Wrap(value)
		}
		showKey := f.ShowKey
		if r.profile.CompactMode {
			if hidden, ok := r.profile.HideKeys[f.Key]; ok && hidden {
				showKey = false
			}
		}
		if !showKey {
			parts = append(parts, value)
			continue
		}
		parts = append(parts, f.Key+"="+value)
	}
	return strings.Join(parts, " ")
}

func (r defaultRenderer) levelBadge(level string) string {
	lvl := strings.ToUpper(level)
	label := lvl
	if l, ok := r.profile.LevelLabels[lvl]; ok {
		label = l
	}
	switch lvl {
	case "DEBUG":
		return r.theme.Debug.Wrap(label)
	case "WARN":
		return r.theme.Warn.Wrap(label)
	case "ERROR":
		return r.theme.Error.Wrap(label)
	default:
		return r.theme.Info.Wrap(label)
	}
}

func ParseTextLogLine(line string) *LogRecord {
	rec := &LogRecord{Raw: line, Fields: make([]RecordField, 0, 16)}
	tokens := splitQuotedTokens(line)
	for _, tok := range tokens {
		i := strings.IndexByte(tok, '=')
		if i <= 0 {
			continue
		}
		key := tok[:i]
		value := tok[i+1:]
		switch key {
		case "time":
			rec.Time = value
		case "level":
			rec.Level = strings.Trim(value, "\"")
		case "msg":
			rec.Message = value
		default:
			rec.Fields = append(rec.Fields, RecordField{
				Key:     key,
				Value:   value,
				ShowKey: true,
			})
		}
	}
	return rec
}

func splitQuotedTokens(s string) []string {
	tokens := make([]string, 0, 16)
	start := -1
	inQuotes := false
	escaped := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if start == -1 && c != ' ' {
			start = i
		}
		if start == -1 {
			continue
		}
		if inQuotes {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				inQuotes = false
			}
			continue
		}
		if c == '"' {
			inQuotes = true
			continue
		}
		if c == ' ' {
			tokens = append(tokens, s[start:i])
			start = -1
		}
	}
	if start != -1 {
		tokens = append(tokens, s[start:])
	}
	return tokens
}

var (
	stateMu      sync.RWMutex
	currentTheme = style.DefaultTheme()
	currentProf  = DefaultProfile()
	pipeline     = NewPipeline(currentTheme, currentProf, nil, nil, nil, nil)
)

type LoggerConfig struct {
	LogFilePath    string
	ArchiveDir     string
	Level          slog.Level
	Theme          style.Theme
	Profile        Profile
	FieldProvider  FieldStyleProvider
	FieldTransform FieldTransformer
	Processors     []Processor
	Renderer       Renderer
}

func SetupDefaultSlog(cfg LoggerConfig) (*os.File, error) {
	term.EnableConsoleANSI()
	theme := cfg.Theme
	if theme.TimeKey.Wrap("x") == "x" {
		theme = style.DefaultTheme()
	}
	prof := normalizeProfile(cfg.Profile)
	pl := NewPipeline(theme, prof, cfg.FieldProvider, cfg.FieldTransform, cfg.Processors, cfg.Renderer)

	stateMu.Lock()
	currentTheme = theme
	currentProf = prof
	pipeline = pl
	stateMu.Unlock()

	logPath := strings.TrimSpace(cfg.LogFilePath)
	if logPath == "" {
		logPath = "server.log"
	}
	archiveDir := strings.TrimSpace(cfg.ArchiveDir)
	if archiveDir == "" {
		archiveDir = "logs"
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return nil, fmt.Errorf("create logs dir: %w", err)
	}
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", logPath, err)
	}

	consoleOut := NewColorizingWriter(os.Stdout)
	consoleHandler := slog.NewTextHandler(consoleOut, &slog.HandlerOptions{Level: cfg.Level})
	fileHandler := slog.NewTextHandler(file, &slog.HandlerOptions{Level: cfg.Level})
	slog.SetDefault(slog.New(fanoutHandler{handlers: []slog.Handler{consoleHandler, fileHandler}}))
	return file, nil
}

func RotateAndCompressLog(srcPath, archiveDir string) error {
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		srcPath = "server.log"
	}
	archiveDir = strings.TrimSpace(archiveDir)
	if archiveDir == "" {
		archiveDir = "logs"
	}
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return err
	}
	info, err := os.Stat(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() == 0 {
		return nil
	}
	ts := time.Now().Format("2006-01-02_15-04-05")
	dst := filepath.Join(archiveDir, fmt.Sprintf("server_%s.log.gz", ts))

	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func(in *os.File) {
		_ = in.Close()
	}(in)

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(out)

	_, copyErr := io.Copy(gz, in)
	closeErr := gz.Close()
	outCloseErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if outCloseErr != nil {
		return outCloseErr
	}
	return os.Truncate(srcPath, 0)
}

type fanoutHandler struct {
	handlers []slog.Handler
}

func (h fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, next := range h.handlers {
		if next.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (h fanoutHandler) Handle(ctx context.Context, rec slog.Record) error {
	var firstErr error
	for _, next := range h.handlers {
		if err := next.Handle(ctx, rec); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (h fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, 0, len(h.handlers))
	for _, next := range h.handlers {
		out = append(out, next.WithAttrs(attrs))
	}
	return fanoutHandler{handlers: out}
}

func (h fanoutHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, 0, len(h.handlers))
	for _, next := range h.handlers {
		out = append(out, next.WithGroup(name))
	}
	return fanoutHandler{handlers: out}
}

type ColorizingWriter struct {
	dst io.Writer
	buf []byte
}

func NewColorizingWriter(dst io.Writer) *ColorizingWriter {
	return &ColorizingWriter{dst: dst}
}

func (w *ColorizingWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		if _, err := io.WriteString(w.dst, ColorizeLogLine(line)+"\n"); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

func ColorizeLogLine(line string) string {
	stateMu.RLock()
	pl := pipeline
	stateMu.RUnlock()
	if pl == nil {
		return line
	}
	return pl.Colorize(line)
}
