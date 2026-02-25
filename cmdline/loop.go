package cmdline

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/chzyer/readline"
)

type Command struct {
	Name        string
	Aliases     []string
	Description string
	Execute     func(args string)
	Complete    func(argPos int, prefix string) []string
}

type Options struct {
	Prompt          string
	HistoryLimit    int
	InterruptPrompt string
	EOFPrompt       string
	OnInterrupt     func()
	OnUnknown       func(name string)
	Resolve         func(name, args string) bool
	CommandNames    func(prefix string) []string
	ArgSuggestions  func(name string, argPos int, prefix string) []string
	Out             *os.File
	Log             *slog.Logger
}

type Loop struct {
	opts     Options
	done     chan struct{}
	commands map[string]*Command
	started  time.Time
}

func NewLoop(opts Options) *Loop {
	if strings.TrimSpace(opts.Prompt) == "" {
		opts.Prompt = "> "
	}
	if opts.HistoryLimit <= 0 {
		opts.HistoryLimit = 200
	}
	if strings.TrimSpace(opts.InterruptPrompt) == "" {
		opts.InterruptPrompt = "^C"
	}
	if strings.TrimSpace(opts.EOFPrompt) == "" {
		opts.EOFPrompt = "exit"
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.Log == nil {
		opts.Log = slog.Default()
	}
	l := &Loop{
		opts:     opts,
		done:     make(chan struct{}),
		commands: map[string]*Command{},
		started:  time.Now(),
	}
	l.Register(Command{
		Name:        "help",
		Description: "Show available console commands",
		Execute: func(string) {
			names := l.availableNames("")
			_, _ = fmt.Fprintln(opts.Out, "Available commands:")
			for _, n := range names {
				_, _ = fmt.Fprintln(opts.Out, " - "+n)
			}
		},
	})
	l.Register(Command{
		Name:        "clear",
		Aliases:     []string{"cls"},
		Description: "Clear terminal screen",
		Execute: func(string) {
			_, _ = fmt.Fprint(opts.Out, "\x1b[H\x1b[2J")
		},
	})
	l.Register(Command{
		Name:        "uptime",
		Description: "Show console loop uptime",
		Execute: func(string) {
			_, _ = fmt.Fprintf(opts.Out, "uptime: %s\n", time.Since(l.started).Truncate(time.Second))
		},
	})
	l.Register(Command{
		Name:        "pid",
		Description: "Show current process id",
		Execute: func(string) {
			_, _ = fmt.Fprintf(opts.Out, "pid: %d\n", os.Getpid())
		},
	})
	l.Register(Command{
		Name:        "echo",
		Description: "Print text back to console",
		Execute: func(args string) {
			_, _ = fmt.Fprintln(opts.Out, args)
		},
	})
	return l
}

func (l *Loop) Register(c Command) {
	name := strings.ToLower(strings.TrimSpace(c.Name))
	if name == "" {
		return
	}
	if len(c.Aliases) == 0 {
		c.Aliases = []string{name}
	} else {
		seen := map[string]struct{}{}
		out := make([]string, 0, len(c.Aliases)+1)
		out = append(out, name)
		seen[name] = struct{}{}
		for _, a := range c.Aliases {
			s := strings.ToLower(strings.TrimSpace(a))
			if s == "" {
				continue
			}
			if _, ok := seen[s]; ok {
				continue
			}
			seen[s] = struct{}{}
			out = append(out, s)
		}
		c.Aliases = out
	}
	cmdCopy := c
	for _, a := range c.Aliases {
		l.commands[a] = &cmdCopy
	}
}

func (l *Loop) Start() <-chan struct{} {
	go func() {
		defer close(l.done)
		rl, err := readline.NewEx(&readline.Config{
			Prompt:          l.opts.Prompt,
			HistoryLimit:    l.opts.HistoryLimit,
			InterruptPrompt: l.opts.InterruptPrompt,
			EOFPrompt:       l.opts.EOFPrompt,
			AutoComplete:    &completer{loop: l},
		})
		if err != nil {
			l.opts.Log.Warn("console readline init failed", "err", err)
			return
		}
		defer func(rl *readline.Instance) {
			_ = rl.Close()
		}(rl)
		l.opts.Log.Info("console command input enabled")
		for {
			line, err := rl.Readline()
			if errors.Is(err, readline.ErrInterrupt) {
				if l.opts.OnInterrupt != nil {
					l.opts.OnInterrupt()
				}
				return
			}
			if err != nil {
				break
			}
			name, args, ok := splitCommand(line)
			if !ok {
				continue
			}
			if cmd, found := l.commands[name]; found {
				if cmd.Execute != nil {
					cmd.Execute(args)
				}
				continue
			}
			if l.opts.Resolve != nil && l.opts.Resolve(name, args) {
				continue
			}
			if l.opts.OnUnknown != nil {
				l.opts.OnUnknown(name)
			}
		}
		l.opts.Log.Info("console input stopped")
	}()
	return l.done
}

type completer struct {
	loop *Loop
}

func (c *completer) Do(line []rune, pos int) ([][]rune, int) {
	input := string(line[:pos])
	trimmed := strings.TrimLeftFunc(input, unicode.IsSpace)
	if strings.HasPrefix(trimmed, "/") {
		trimmed = strings.TrimPrefix(trimmed, "/")
	}
	if trimmed == "" {
		return toRunes(c.loop.availableNames("")), 0
	}
	hasTrailingSpace := len(trimmed) > 0 && unicode.IsSpace(rune(trimmed[len(trimmed)-1]))
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return toRunes(completionTail(c.loop.availableNames(""), "")), 0
	}
	if len(parts) == 1 && !hasTrailingSpace {
		prefix := strings.ToLower(parts[0])
		return toRunes(completionTail(c.loop.availableNames(prefix), prefix)), len([]rune(prefix))
	}
	name := strings.ToLower(parts[0])
	argPos := 0
	prefix := ""
	if hasTrailingSpace {
		argPos = len(parts) - 1
	} else {
		argPos = len(parts) - 2
		prefix = parts[len(parts)-1]
	}
	if argPos < 0 {
		return nil, 0
	}
	suggestions := map[string]struct{}{}
	if cmd, ok := c.loop.commands[name]; ok && cmd.Complete != nil {
		for _, s := range cmd.Complete(argPos, prefix) {
			if matchPrefix(s, prefix) {
				suggestions[s] = struct{}{}
			}
		}
	} else if c.loop.opts.ArgSuggestions != nil {
		for _, s := range c.loop.opts.ArgSuggestions(name, argPos, prefix) {
			if matchPrefix(s, prefix) {
				suggestions[s] = struct{}{}
			}
		}
	}
	if len(suggestions) == 0 {
		return nil, 0
	}
	out := make([]string, 0, len(suggestions))
	for s := range suggestions {
		out = append(out, s)
	}
	sort.Strings(out)
	return toRunes(completionTail(out, prefix)), len([]rune(prefix))
}

func (l *Loop) availableNames(prefix string) []string {
	names := map[string]struct{}{}
	for alias := range l.commands {
		if matchPrefix(alias, prefix) {
			names[alias] = struct{}{}
		}
	}
	if l.opts.CommandNames != nil {
		for _, n := range l.opts.CommandNames(prefix) {
			s := strings.ToLower(strings.TrimSpace(n))
			if s == "" {
				continue
			}
			if matchPrefix(s, prefix) {
				names[s] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(names))
	for n := range names {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func splitCommand(line string) (name, args string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return "", "", false
	}
	if strings.HasPrefix(line, "/") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "/"))
	}
	parts := strings.SplitN(line, " ", 2)
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		return "", "", false
	}
	name = strings.ToLower(strings.TrimSpace(parts[0]))
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return name, args, true
}

func matchPrefix(value, prefix string) bool {
	if prefix == "" {
		return true
	}
	return strings.HasPrefix(strings.ToLower(value), strings.ToLower(prefix))
}

func completionTail(options []string, prefix string) []string {
	out := make([]string, 0, len(options))
	prefixRunes := []rune(prefix)
	prefixLen := len(prefixRunes)
	for _, option := range options {
		optRunes := []rune(option)
		if prefixLen > 0 && len(optRunes) >= prefixLen &&
			strings.EqualFold(string(optRunes[:prefixLen]), prefix) {
			out = append(out, string(optRunes[prefixLen:]))
			continue
		}
		out = append(out, option)
	}
	return out
}

func toRunes(items []string) [][]rune {
	out := make([][]rune, 0, len(items))
	for _, item := range items {
		out = append(out, []rune(item))
	}
	return out
}
