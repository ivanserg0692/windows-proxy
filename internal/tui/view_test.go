package tui

import (
	"bytes"
	"strings"
	"testing"

	"wslproxy/internal/app"
	"wslproxy/internal/tui/screens"
)

func TestViewTemplatePreservesBlockOrderAndSpacing(t *testing.T) {
	for _, tc := range []struct {
		name, busy, problem, notice, want string
	}{
		{name: "no messages", want: "body\n\nfooter\n\nhelp"},
		{name: "busy", busy: "working", want: "body\n\nworking\n\nfooter\n\nhelp"},
		{name: "error", problem: "failure", want: "body\n\nfailure\n\nfooter\n\nhelp"},
		{name: "notice", notice: "saved", want: "body\n\nsaved\n\nfooter\n\nhelp"},
		{name: "all messages", busy: "working", problem: "failure", notice: "saved", want: "body\n\nworking\n\nfailure\n\nsaved\n\nfooter\n\nhelp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := viewData{
				Title: "title", Status: "status", Subtitle: "subtitle",
				Body: "body", Footer: "\n\nfooter", Help: "help",
				Busy: tc.busy, Error: tc.problem, Notice: tc.notice,
			}
			var output bytes.Buffer
			if err := viewTemplate.Execute(&output, data); err != nil {
				t.Fatal(err)
			}
			want := "title                         status\nsubtitle\n\n" + tc.want
			if output.String() != want {
				t.Fatalf("unexpected layout:\ngot  %q\nwant %q", output.String(), want)
			}
		})
	}
}

type snapshotScreen struct {
	recordingScreen
	snapshot screens.Snapshot
	calls    int
}

func (s *snapshotScreen) View(snapshot screens.Snapshot) screens.Content {
	s.snapshot = snapshot
	s.calls++
	return screens.Content{Body: "body {{.Status}}", Footer: "footer", Help: "help"}
}

func TestViewPreparesScreenDataWithoutReadingInactiveServer(t *testing.T) {
	for _, busy := range []string{"", "working"} {
		m := newTestModel(t)
		screen := &snapshotScreen{}
		m.setActive(screen)
		m.running = &app.Running{} // A nil server must not be inspected on other screens.
		m.busy, m.error, m.notice = busy, "bad\x00error", "new\x1bnotice"
		data := m.prepareViewData()
		if screen.calls != 1 || screen.snapshot.Config != m.config || !screen.snapshot.ProxyActive || screen.snapshot.Running != nil || screen.snapshot.RecentEvents != nil {
			t.Fatal("active screen did not receive the expected snapshot exactly once")
		}
		wantHelp := "help"
		if busy != "" {
			wantHelp = "Ctrl+C: cancel and exit"
		}
		if data.Status != "RUNNING" || data.Body != "body {{.Status}}" || data.Footer != "footer" || data.Help != wantHelp || data.Error != "bad error" || data.Notice != "new notice" {
			t.Fatalf("unexpected presentation data: %+v", data)
		}
		view := m.View()
		if !view.AltScreen || view.WindowTitle != "WSL Proxy" || !strings.Contains(view.Content, "body {{.Status}}") {
			t.Fatal("view options or literal screen content changed")
		}
	}
}

func TestViewRetainsSmallTerminalGuard(t *testing.T) {
	for _, size := range [][2]int{{39, 30}, {80, 19}} {
		// No active screen: the guard must return before preparing screen data.
		m := &model{width: size[0], height: size[1]}
		view := m.View()
		if view.Content != "WSL PROXY\nResize terminal to at least 40 x 20.\nCtrl+C to exit." || !view.AltScreen || view.WindowTitle != "" {
			t.Fatalf("small terminal behavior changed: %+v", view)
		}
	}
}

func TestViewShowsRecentEventsOnlyOnTallRunningScreen(t *testing.T) {
	for _, height := range []int{31, 32} {
		m := newTestModel(t)
		m.height = height
		m.logs.lines = []string{"event"}
		m.setActive(m.runningView)
		data := m.prepareViewData()
		if strings.Contains(data.Footer, "event") != (height >= 32) {
			t.Fatalf("unexpected events at height %d: %q", height, data.Footer)
		}
		if data.Status != "STOPPED" {
			t.Fatal("stopped proxy status changed")
		}
		m.setActive(m.menu)
		if strings.Contains(m.prepareViewData().Footer, "event") {
			t.Fatal("running screen events leaked into the menu")
		}
	}
}
