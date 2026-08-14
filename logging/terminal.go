package logging

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/VexoraDevelopment/consolex/style"
)

const statusRefresh = 80 * time.Millisecond

var spinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type promptController interface {
	Clean()
	Refresh()
}

type terminalRenderer struct {
	out         io.Writer
	interactive bool
	mu          sync.Mutex
	statuses    []*statusState
	rendered    int
	frame       int
	prompt      promptController
	wake        chan struct{}
	stop        chan struct{}
	done        chan struct{}
	closed      bool
}

type statusState struct {
	renderer *terminalRenderer
	message  string
	progress float64
	done     bool
}

type Status struct{ state *statusState }

func newTerminalRenderer(out io.Writer, interactive bool) *terminalRenderer {
	r := &terminalRenderer{out: out, interactive: interactive, wake: make(chan struct{}, 1), stop: make(chan struct{}), done: make(chan struct{})}
	if interactive {
		go r.run()
	} else {
		close(r.done)
	}
	return r
}

func (r *terminalRenderer) writeLog(line string) error {
	_, err := r.write([]byte(line + "\n"))
	return err
}

func (r *terminalRenderer) write(data []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.prompt != nil {
		r.prompt.Clean()
	}
	if r.interactive {
		r.clearLocked()
	}
	n, err := r.out.Write(data)
	if r.interactive {
		r.drawLocked()
	}
	if r.prompt != nil {
		r.prompt.Refresh()
	}
	return n, err
}

type terminalWriter struct{ renderer *terminalRenderer }

func (w terminalWriter) Write(data []byte) (int, error) {
	rendered := renderMinecraftText(string(data), w.renderer.interactive)
	if !w.renderer.interactive {
		rendered = style.StripANSI(rendered)
	}
	_, err := w.renderer.write([]byte(rendered))
	return len(data), err
}

func renderMinecraftText(text string, ansi bool) string {
	codes := map[rune]string{
		'0': "30", '1': "34", '2': "32", '3': "36", '4': "31", '5': "35", '6': "33", '7': "37",
		'8': "90", '9': "94", 'a': "92", 'b': "96", 'c': "91", 'd': "95", 'e': "93", 'f': "97",
		'l': "1", 'm': "9", 'n': "4", 'o': "3", 'r': "0",
	}
	runes := []rune(text)
	var out strings.Builder
	styled := false
	for i := 0; i < len(runes); i++ {
		if runes[i] != '§' || i+1 >= len(runes) {
			out.WriteRune(runes[i])
			continue
		}
		i++
		key := unicode.ToLower(runes[i])
		code, ok := codes[key]
		if ansi && ok {
			out.WriteString("\x1b[" + code + "m")
			styled = key != 'r'
		}
	}
	if ansi && styled {
		value := out.String()
		if strings.HasSuffix(value, "\n") {
			return strings.TrimSuffix(value, "\n") + "\x1b[0m\n"
		}
		out.WriteString("\x1b[0m")
	}
	return out.String()
}

func (r *terminalRenderer) newStatus(message string) *Status {
	state := &statusState{renderer: r, message: strings.TrimSpace(message), progress: -1}
	r.mu.Lock()
	if r.closed {
		state.done = true
	} else {
		r.statuses = append(r.statuses, state)
	}
	r.mu.Unlock()
	r.notify()
	return &Status{state: state}
}

func (s *Status) Update(message string, progress float64) {
	if s == nil || s.state == nil {
		return
	}
	r := s.state.renderer
	r.mu.Lock()
	if !s.state.done {
		if strings.TrimSpace(message) != "" {
			s.state.message = strings.TrimSpace(message)
		}
		if progress >= 0 {
			s.state.progress = math.Min(1, progress)
		}
	}
	r.mu.Unlock()
	r.notify()
}

func (s *Status) SetProgress(progress float64) { s.Update("", progress) }
func (s *Status) Success(message string)       { s.finish("✓", message) }
func (s *Status) Failed(message string)        { s.finish("✗", message) }
func (s *Status) Cancelled(message string)     { s.finish("-", message) }
func (s *Status) Close()                       { s.finish("", "") }

func (s *Status) finish(marker, message string) {
	if s == nil || s.state == nil {
		return
	}
	r := s.state.renderer
	r.mu.Lock()
	if s.state.done {
		r.mu.Unlock()
		return
	}
	s.state.done = true
	for i, active := range r.statuses {
		if active == s.state {
			r.statuses = append(r.statuses[:i], r.statuses[i+1:]...)
			break
		}
	}
	if strings.TrimSpace(message) != "" {
		if r.prompt != nil {
			r.prompt.Clean()
		}
		if r.interactive {
			r.clearLocked()
		}
		_, _ = fmt.Fprintf(r.out, "%s %s\n", marker, strings.TrimSpace(message))
		if r.interactive {
			r.drawLocked()
		}
		if r.prompt != nil {
			r.prompt.Refresh()
		}
	}
	r.mu.Unlock()
	r.notify()
}

func (r *terminalRenderer) attachPrompt(prompt promptController) {
	r.mu.Lock()
	r.prompt = prompt
	r.mu.Unlock()
}

func (r *terminalRenderer) setTitle(title string) {
	if !r.interactive {
		return
	}
	title = strings.Map(func(char rune) rune {
		if char < 0x20 || char == 0x7f {
			return -1
		}
		return char
	}, title)
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.closed {
		_, _ = fmt.Fprintf(r.out, "\x1b]2;%s\x07", title)
	}
}

func (r *terminalRenderer) notify() {
	if !r.interactive {
		return
	}
	select {
	case r.wake <- struct{}{}:
	default:
	}
}

func (r *terminalRenderer) run() {
	defer close(r.done)
	for {
		select {
		case <-r.stop:
			return
		case <-r.wake:
		}
		timer := time.NewTimer(statusRefresh)
		select {
		case <-r.stop:
			timer.Stop()
			return
		case <-r.wake:
			<-timer.C
		case <-timer.C:
		}
		for {
			r.mu.Lock()
			active := len(r.statuses) != 0
			if active || r.rendered != 0 {
				if r.prompt != nil {
					r.prompt.Clean()
				}
				r.clearLocked()
				if active {
					r.drawLocked()
				}
				if r.prompt != nil {
					r.prompt.Refresh()
				}
			}
			r.mu.Unlock()
			if !active {
				break
			}
			timer := time.NewTimer(statusRefresh)
			select {
			case <-r.stop:
				timer.Stop()
				return
			case <-r.wake:
				<-timer.C
			case <-timer.C:
			}
		}
	}
}

func (r *terminalRenderer) clearLocked() {
	if r.rendered == 0 {
		return
	}
	_, _ = fmt.Fprintf(r.out, "\x1b[%dA", r.rendered)
	for i := 0; i < r.rendered; i++ {
		_, _ = io.WriteString(r.out, "\r\x1b[2K")
		if i+1 < r.rendered {
			_, _ = io.WriteString(r.out, "\x1b[1B")
		}
	}
	if r.rendered > 1 {
		_, _ = fmt.Fprintf(r.out, "\x1b[%dA", r.rendered-1)
	}
	r.rendered = 0
}

func (r *terminalRenderer) drawLocked() {
	for _, status := range r.statuses {
		line := spinnerFrames[r.frame%len(spinnerFrames)] + " " + status.message
		if status.progress >= 0 {
			line += fmt.Sprintf(" %d%%", int(math.Round(status.progress*100)))
		}
		_, _ = io.WriteString(r.out, line+"\n")
		r.rendered++
	}
	r.frame++
}

func (r *terminalRenderer) close() {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return
	}
	r.closed = true
	if r.prompt != nil {
		r.prompt.Clean()
	}
	if r.interactive {
		r.clearLocked()
	}
	r.statuses = nil
	if r.prompt != nil {
		r.prompt.Refresh()
	}
	r.mu.Unlock()
	if r.interactive {
		close(r.stop)
		<-r.done
	}
}
