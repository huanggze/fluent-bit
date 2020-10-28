package main

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const (
	binPath  = "/fluent-bit/bin/fluent-bit-core"
	cfgPath  = "/fluent-bit/etc/fluent-bit.conf"
	watchDir = "/fluent-bit/config"
)

var (
	watcher *fsnotify.Watcher
	cmd     *exec.Cmd
)

func main() {
	ctx := context.Background()
	logger := log.NewLogfmtLogger(os.Stdout)

	ready := make(chan struct{})
	var g run.Group
	{
		// Termination handler.
		g.Add(run.SignalHandler(ctx, os.Interrupt, syscall.SIGTERM))
	}
	{
		// Initial loading.
		cancel := make(chan struct{})
		g.Add(
			func() error {
				var err error

				watcher, err = fsnotify.NewWatcher()
				if err != nil {
					level.Error(logger).Log("err", err)
					return err
				}

				// Launch Fluent bit
				cmd = exec.Command(binPath, "-c", cfgPath)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err = cmd.Start()
				if err != nil {
					level.Error(logger).Log("err", err)
					return err
				}

				ready <- struct{}{}

				<-cancel
				return nil
			},
			func(err error) {
				close(cancel)
			},
		)
	}
	{
		// Reload handler.
		cancel := make(chan struct{})
		g.Add(
			func() error {
				// Wait until Fluent Bit starts
				<-ready

				go func() {
					for {
						select {
						case <-cancel:
							return
						case event := <-watcher.Events:
							if !isValidEvent(event) {
								continue
							}

							level.Info(logger).Log("msg", "Config file changed")
							level.Info(logger).Log("msg", "Stop Fluent Bit")

							if err := cmd.Process.Kill(); err != nil {
								level.Error(logger).Log("err", err)
							}
							cmd = exec.Command(binPath, "-c", cfgPath)
							cmd.Stdout = os.Stdout
							cmd.Stderr = os.Stderr
							err := cmd.Start()
							if err != nil {
								level.Error(logger).Log("err", err)
								cancel <- struct{}{}
								return
							}

							level.Info(logger).Log("msg", "Fluent Bit restarts")
						case <-watcher.Errors:
							level.Error(logger).Log("msg", "Watcher stopped")
							cancel <- struct{}{}
							return
						}
					}
				}()

				// Start watcher.
				err := watcher.Add(watchDir)
				if err != nil {
					level.Error(logger).Log("err", err)
					return err
				}

				<-cancel
				return nil
			},
			func(err error) {
				watcher.Close()
				cmd.Process.Kill()
				close(cancel)
			},
		)
	}

	if err := g.Run(); err != nil {
		level.Error(logger).Log("err", err)
		os.Exit(1)
	}
	level.Info(logger).Log("msg", "See you next time!")
}

// Inspired by https://github.com/jimmidyson/configmap-reload
func isValidEvent(event fsnotify.Event) bool {
	if event.Op&fsnotify.Create != fsnotify.Create {
		return false
	}
	if filepath.Base(event.Name) != "..data" {
		return false
	}
	return true
}