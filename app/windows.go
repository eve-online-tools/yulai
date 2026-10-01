package app

import (
	"net/url"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

const addWindowName = "add-character"

// Picker is narrow and tall; the SSO page wants room.
var (
	pickerSize = [2]int{440, 640}
	ssoSize    = [2]int{1000, 800}
)

// loginWindow implements character.LoginUI with a second Wails window.
type loginWindow struct {
	mu       sync.Mutex
	win      *application.WebviewWindow
	onClosed func()
}

func (l *loginWindow) OpenPicker() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.win != nil {
		l.win.Focus()
		return
	}
	w := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             addWindowName,
		Title:            "Add character",
		Width:            pickerSize[0],
		Height:           pickerSize[1],
		MinWidth:         pickerSize[0],
		MinHeight:        480,
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/#/add",
	})
	w.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		l.mu.Lock()
		cb := l.onClosed
		l.win, l.onClosed = nil, nil
		l.mu.Unlock()
		if cb != nil {
			cb()
		}
	})
	l.win = w
}

func (l *loginWindow) ShowSSO(ssoURL string, onClosed func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.win == nil {
		return
	}
	l.onClosed = onClosed
	l.win.SetSize(ssoSize[0], ssoSize[1])
	l.win.Center()
	l.win.SetURL(ssoURL)
}

func (l *loginWindow) Fail(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.win == nil {
		return
	}
	l.onClosed = nil
	l.win.SetSize(pickerSize[0], pickerSize[1])
	l.win.Center()
	l.win.SetURL("/#/add?" + url.Values{"error": {msg}}.Encode())
}

func (l *loginWindow) Close() {
	l.mu.Lock()
	w := l.win
	l.win, l.onClosed = nil, nil
	l.mu.Unlock()
	if w != nil {
		w.Close()
	}
}
