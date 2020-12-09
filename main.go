package main

import (
	"context"
	"github.com/fsnotify/fsnotify"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/oklog/run"
	"os"
	"os/exec"
	"syscall"
)

const (
	binPath  = "/fluent-bit/bin/fluent-bit"
	cfgPath  = "/fluent-bit/etc/fluent-bit.conf"
	watchDir = "/fluent-bit/config"
)

var (
	cmd *exec.Cmd
)

func main() {
	logger := log.NewLogfmtLogger(os.Stdout)

	var g run.Group
	{
		// Termination handler.
		g.Add(run.SignalHandler(context.Background(), os.Interrupt, syscall.SIGTERM))
	}
	{
		// Watch the Fluent bit, if the Fluent bit not exists or stopped, restart it.
		cancel := make(chan struct{})
		g.Add(
			func() error {

				for {
					select {
					case <-cancel:
						return nil
					default:
					}

					if cmd == nil {
						cmd = exec.Command(binPath, "-c", cfgPath)
						cmd.Stdout = os.Stdout
						cmd.Stderr = os.Stderr
						if err := cmd.Start(); err != nil {
							_ = level.Error(logger).Log("msg", "start Fluent bit error", "error", err)
						}

						_ = level.Info(logger).Log("msg", "Fluent bit started")
					}

					if cmd != nil {
						_ = level.Error(logger).Log("msg", "Fluent bit exited", "error", cmd.Wait())
						cmd = nil
					}
				}
			},
			func(err error) {
				close(cancel)
				if cmd != nil {
					_ = cmd.Process.Kill()
				}
			},
		)
	}
	{
		// Watch the config file, if the config file changed, stop Fluent bit.
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			_ = level.Error(logger).Log("err", err)
			return
		}

		// Start watcher.
		err = watcher.Add(watchDir)
		if err != nil {
			_ = level.Error(logger).Log("err", err)
			return
		}

		cancel := make(chan struct{})
		g.Add(
			func() error {

				for {
					select {
					case <-cancel:
						return nil
					case event := <-watcher.Events:
						if !isValidEvent(event) {
							continue
						}

						_ = level.Info(logger).Log("msg", "Config file changed")
						_ = level.Info(logger).Log("msg", "Stop Fluent Bit")

						if err := cmd.Process.Kill(); err != nil {
							_ = level.Error(logger).Log("msg", "Stop Fluent Bit error", "error", err)
						}
					case <-watcher.Errors:
						_ = level.Error(logger).Log("msg", "Watcher stopped")
						return nil
					}
				}
			},
			func(err error) {
				_ = watcher.Close()
				if cmd != nil {
					_ = cmd.Process.Kill()
				}
				close(cancel)
			},
		)
	}

	if err := g.Run(); err != nil {
		_ = level.Error(logger).Log("err", err)
		os.Exit(1)
	}
	_ = level.Info(logger).Log("msg", "See you next time!")
}

// Inspired by https://github.com/jimmidyson/configmap-reload
func isValidEvent(event fsnotify.Event) bool {
	if event.Op&fsnotify.Create != fsnotify.Create {
		return false
	}
	//if filepath.Base(event.Name) != "..data" {
	//	return false
	//}
	return true
}
