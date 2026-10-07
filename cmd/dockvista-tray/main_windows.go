//go:build windows

// Command dockvista-tray is a Windows notification-area helper: start/stop
// the sibling dockvista.exe, open the browser, no extra desktop window.
package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/getlantern/systray"
)

const listenAddr = "127.0.0.1:8080"

type controller struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	owned   bool
	dataDir string
	server  string
	logFile *os.File
}

func main() {
	systray.Run(onReady, func() {})
}

func onReady() {
	systray.SetIcon(trayIcon())
	systray.SetTitle("DockVista")
	systray.SetTooltip("DockVista")

	mOpen := systray.AddMenuItem("Open DockVista", "Open http://"+listenAddr)
	mStart := systray.AddMenuItem("Start", "Start the DockVista server")
	mStop := systray.AddMenuItem("Stop", "Stop the server this tray started")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Stop (if we started it) and exit")

	c, err := newController()
	if err != nil {
		systray.SetTooltip("DockVista: " + err.Error())
		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
				case <-mStart.ClickedCh:
				case <-mStop.ClickedCh:
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
		return
	}

	_ = c.start()
	c.syncMenu(mStart, mStop)

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowser("http://" + listenAddr)
			case <-mStart.ClickedCh:
				if err := c.start(); err != nil {
					systray.SetTooltip("DockVista: " + err.Error())
				}
				c.syncMenu(mStart, mStop)
			case <-mStop.ClickedCh:
				c.stop()
				c.syncMenu(mStart, mStop)
			case <-mQuit.ClickedCh:
				c.stop()
				_ = c.logFile.Close()
				systray.Quit()
				return
			}
		}
	}()
}

func newController() (*controller, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	server := filepath.Join(filepath.Dir(self), "dockvista.exe")
	if _, err := os.Stat(server); err != nil {
		return nil, fmt.Errorf("put dockvista.exe next to dockvista-tray.exe")
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, err = os.UserConfigDir()
		if err != nil {
			return nil, err
		}
	}
	dataDir := filepath.Join(base, "DockVista")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	logFile, err := os.OpenFile(filepath.Join(dataDir, "server.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	return &controller{dataDir: dataDir, server: server, logFile: logFile}, nil
}

func (c *controller) start() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if probe("/healthz") {
		c.owned = c.cmd != nil && c.cmd.Process != nil
		systray.SetTooltip("DockVista — running")
		return nil
	}
	cmd := exec.Command(c.server)
	cmd.Env = append(os.Environ(),
		"DOCKVISTA_ADDR="+listenAddr,
		"DOCKVISTA_DATA_DIR="+c.dataDir,
	)
	cmd.Stdout = c.logFile
	cmd.Stderr = c.logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	c.cmd = cmd
	c.owned = true
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if probe("/healthz") {
			systray.SetTooltip("DockVista — running")
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()
	c.cmd = nil
	c.owned = false
	return fmt.Errorf("server did not become ready; see %%LOCALAPPDATA%%\\DockVista\\server.log")
}

func (c *controller) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.owned || c.cmd == nil || c.cmd.Process == nil {
		if !probe("/healthz") {
			systray.SetTooltip("DockVista — stopped")
		}
		return
	}
	_ = c.cmd.Process.Kill()
	_, _ = c.cmd.Process.Wait()
	c.cmd = nil
	c.owned = false
	systray.SetTooltip("DockVista — stopped")
}

func (c *controller) syncMenu(start, stop *systray.MenuItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	up := probe("/healthz")
	if up {
		start.Disable()
	} else {
		start.Enable()
	}
	if c.owned {
		stop.Enable()
	} else {
		stop.Disable()
	}
}

func probe(path string) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get("http://" + listenAddr + path)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode == http.StatusOK
}

func openBrowser(rawURL string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL).Start()
}
