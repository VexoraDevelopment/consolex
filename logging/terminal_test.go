package logging

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStatusLifecycleAndFallback(t *testing.T) {
	var out bytes.Buffer
	r := newTerminalRenderer(&out, false)
	status := r.newStatus("Loading world")
	status.SetProgress(.42)
	status.Success("Loaded world")
	status.Update("must be ignored", .99)
	r.close()
	if got := out.String(); got != "✓ Loaded world\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestTerminalRedrawsStatusesAroundLog(t *testing.T) {
	var out bytes.Buffer
	prompt := new(testPrompt)
	r := &terminalRenderer{out: &out, interactive: true, prompt: prompt}
	r.statuses = []*statusState{{message: "Loading Overworld", progress: .71}, {message: "Preparing spawn", progress: .44}}
	r.drawLocked()
	if err := r.writeLog("[WARN] Plugin Foo took 428ms"); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if strings.Count(got, "[WARN] Plugin Foo took 428ms") != 1 || !strings.HasSuffix(got, "⠙ Loading Overworld 71%\n⠙ Preparing spawn 44%\n") {
		t.Fatalf("redraw order = %q", got)
	}
	if prompt.clean != 1 || prompt.refresh != 1 {
		t.Fatalf("prompt redraw = clean:%d refresh:%d", prompt.clean, prompt.refresh)
	}
}

type testPrompt struct{ clean, refresh int }

func (p *testPrompt) Clean()   { p.clean++ }
func (p *testPrompt) Refresh() { p.refresh++ }

func TestStatusConcurrentUpdatesAndCompletion(t *testing.T) {
	r := newTerminalRenderer(&bytes.Buffer{}, false)
	status := r.newStatus("Preparing spawn")
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(progress float64) {
			defer wg.Done()
			status.SetProgress(progress)
		}(float64(i) / 50)
	}
	wg.Wait()
	status.Close()
	status.SetProgress(1)
	if !status.state.done || len(r.statuses) != 0 {
		t.Fatal("completed status remained active")
	}
	r.close()
}

func TestInteractiveStatusClearsOnClose(t *testing.T) {
	var out bytes.Buffer
	r := newTerminalRenderer(&out, true)
	status := r.newStatus("Loading")
	time.Sleep(statusRefresh + 20*time.Millisecond)
	status.Close()
	time.Sleep(statusRefresh + 20*time.Millisecond)
	r.close()
	if got := out.String(); !strings.Contains(got, "Loading") || !strings.HasSuffix(got, "\r\x1b[2K") {
		t.Fatalf("status was not cleared: %q", got)
	}
}

func TestTerminalTitleIsTTYOnlyAndSanitized(t *testing.T) {
	var interactive bytes.Buffer
	r := newTerminalRenderer(&interactive, true)
	r.setTitle("Pulse\x1b bad\a")
	r.close()
	if got, want := interactive.String(), "\x1b]2;Pulse bad\x07"; got != want {
		t.Fatalf("title = %q, want %q", got, want)
	}
	var redirected bytes.Buffer
	r = newTerminalRenderer(&redirected, false)
	r.setTitle("Pulse")
	r.close()
	if redirected.Len() != 0 {
		t.Fatalf("redirected title = %q", redirected.String())
	}
}

func TestConsoleWriterRendersMinecraftColors(t *testing.T) {
	var interactive bytes.Buffer
	r := newTerminalRenderer(&interactive, true)
	if _, err := (terminalWriter{renderer: r}).Write([]byte("§cDenied§r\n")); err != nil {
		t.Fatal(err)
	}
	r.close()
	if got, want := interactive.String(), "\x1b[91mDenied\x1b[0m\n"; got != want {
		t.Fatalf("colored output = %q, want %q", got, want)
	}
	var redirected bytes.Buffer
	r = newTerminalRenderer(&redirected, false)
	_, _ = (terminalWriter{renderer: r}).Write([]byte("§cDenied§r\n"))
	_, _ = (terminalWriter{renderer: r}).Write([]byte("\x1b[31mStill denied\x1b[0m\n"))
	r.close()
	if got := redirected.String(); got != "Denied\nStill denied\n" {
		t.Fatalf("redirected output = %q", got)
	}
}
