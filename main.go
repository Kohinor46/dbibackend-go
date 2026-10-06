// dbibackend: PC-side server for installing titles onto a Nintendo Switch running DBI over USB.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/Kohinor46/dbibackend-go/dbi"
	"github.com/Kohinor46/dbibackend-go/gui"
	"github.com/Kohinor46/dbibackend-go/usbconn"
	"github.com/Kohinor46/dbibackend-go/winusb"
)

// Internal: the elevated half of the Windows driver installer (see package winusb).
const installWinUSBFlag = "install-winusb"

func main() {
	cli := flag.Bool("cli", false, "run without GUI (requires titles dir)")
	debug := flag.Bool("debug", false, "enable debug output")
	installDriver := flag.Bool("install-driver", false, "Windows: install the WinUSB driver for the connected Switch and exit")
	installWinUSB := flag.String(installWinUSBFlag, "", "")
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, "Install local titles into Nintendo Switch via USB (DBI).\n\nUsage: %s [-cli] [-debug] [titles_dir]\n       %s -install-driver\n\n", os.Args[0], os.Args[0])
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name != installWinUSBFlag {
				fmt.Fprintf(out, "  -%s\n    \t%s\n", f.Name, f.Usage)
			}
		})
	}
	flag.Parse()
	dir := flag.Arg(0)

	switch {
	case *installWinUSB != "":
		os.Exit(winusb.RunElevated(*installWinUSB))
	case *installDriver:
		attachConsole()
		os.Exit(runInstallDriver())
	case !*cli:
		gui.Run(dir, *debug)
		return
	}

	attachConsole()
	if dir == "" {
		flag.Usage()
		os.Exit(2)
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		fmt.Fprintln(os.Stderr, "Specified path must be a directory")
		os.Exit(2)
	}
	if err := runCLI(dir, *debug); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// runCLI mirrors the original Python backend: wait for the Switch, serve one session, exit.
func runCLI(dir string, debug bool) error {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	hinted := false
	dev, err := usbconn.Wait(ctx, dbi.SwitchVID, dbi.SwitchPID, func(err error) {
		switch {
		case errors.Is(err, usbconn.ErrNotFound):
			log.Info("Waiting for switch")
		case errors.Is(err, usbconn.ErrNoDriver):
			if !hinted { // the same error repeats every second
				hinted = true
				log.Warn("Switch found, but it has no WinUSB driver. Run once: " + os.Args[0] + " -install-driver")
			}
		default:
			log.Warn("Cannot open Switch", "err", err)
		}
	})
	if err != nil {
		return err
	}
	defer dev.Close()

	var last string
	srv := &dbi.Server{Dir: dir, Log: log, OnEvent: func(e dbi.Event) {
		if p, ok := e.(dbi.ProgressEvent); ok && p.Title.Name != last {
			last = p.Title.Name
			log.Info("Sending", "title", p.Title.Name)
		}
	}}
	return srv.Serve(ctx, dev)
}

func runInstallDriver() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := winusb.Install(ctx)
	if res.Log != "" && !errors.Is(err, winusb.ErrNotConnected) { // that log line repeats the error
		fmt.Println(res.Log)
	}
	switch {
	case err != nil:
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	case res.Reboot:
		fmt.Println("Restart Windows to finish the installation.")
	default:
		fmt.Println("Done.")
	}
	return 0
}
