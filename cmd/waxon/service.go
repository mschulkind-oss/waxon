package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// serviceCmd groups install/uninstall/restart/status/logs subcommands for
// running `waxon serve` as a background user service. On Linux this is a
// systemd user unit; on macOS it is a launchd user agent.
func serviceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage waxon as a background user service",
		Long: bold.Sprint("Manage waxon as a background user service.") + `

Installs ` + info.Sprint("waxon serve") + ` as a long-running user service so a deck is always
available at a stable URL. Uses ` + bold.Sprint("systemd --user") + ` on Linux and ` + bold.Sprint("launchd") + ` on macOS.
No root or sudo required — installs into your home directory.

` + dim.Sprint("Subcommands:") + `
  ` + info.Sprint("waxon service install <file.slides>") + `   Install + start the service
  ` + info.Sprint("waxon service uninstall") + `                Stop + remove the service
  ` + info.Sprint("waxon service restart") + `                  Restart the service
  ` + info.Sprint("waxon service status") + `                   Show service status
  ` + info.Sprint("waxon service logs") + `                     Tail service logs

` + dim.Sprint("For agents:") + `
  Run ` + info.Sprint("waxon service install deck.slides") + ` once. After every edit
  to the .slides file, the live-reload server will push the change
  to any open browser tab — no restart needed.`,
	}

	cmd.AddCommand(serviceInstallCmd())
	cmd.AddCommand(serviceUninstallCmd())
	cmd.AddCommand(serviceRestartCmd())
	cmd.AddCommand(serviceStatusCmd())
	cmd.AddCommand(serviceLogsCmd())

	return cmd
}

func serviceInstallCmd() *cobra.Command {
	var (
		port  int
		bind  string
		theme string
	)

	cmd := &cobra.Command{
		Use:   "install <file.slides>",
		Short: "Install waxon serve as a user service",
		Long: bold.Sprint("Install waxon serve as a user service.") + `

Creates a unit file pointing at the given .slides file, then enables
and starts the service so it runs in the background and on login.

` + dim.Sprint("Linux:") + `   ~/.config/systemd/user/waxon.service
` + dim.Sprint("macOS:") + `   ~/Library/LaunchAgents/dev.waxon.waxon.plist

` + dim.Sprint("The waxon binary used is the one currently on your $PATH.") + `
` + dim.Sprint("Run") + ` ` + info.Sprint("just deploy") + ` ` + dim.Sprint("first to install a fresh build into ~/.local/bin."),
		Example: `  waxon service install deck.slides
  waxon service install deck.slides --port 9000 --theme terminal`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			absFile, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			if _, err := os.Stat(absFile); err != nil {
				return fmt.Errorf("slides file: %w", err)
			}

			binPath, err := waxonBinaryPath()
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			switch runtime.GOOS {
			case "linux":
				return installSystemd(out, binPath, absFile, port, bind, theme)
			case "darwin":
				return installLaunchd(out, binPath, absFile, port, bind, theme)
			default:
				return fmt.Errorf("unsupported OS: %s (only linux and darwin)", runtime.GOOS)
			}
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "HTTP server port")
	cmd.Flags().StringVar(&bind, "bind", "127.0.0.1", "Bind address")
	cmd.Flags().StringVar(&theme, "theme", "", "Override the theme from frontmatter")

	return cmd
}

func serviceUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the waxon user service",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			switch runtime.GOOS {
			case "linux":
				return uninstallSystemd(out)
			case "darwin":
				return uninstallLaunchd(out)
			default:
				return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
			}
		},
	}
}

func serviceRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the waxon user service",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch runtime.GOOS {
			case "linux":
				return runStream("systemctl", "--user", "restart", "waxon")
			case "darwin":
				plist, _ := launchdPlistPath()
				if err := runStream("launchctl", "unload", plist); err != nil {
					return err
				}
				return runStream("launchctl", "load", plist)
			default:
				return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
			}
		},
	}
}

func serviceStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show waxon service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch runtime.GOOS {
			case "linux":
				return runStream("systemctl", "--user", "status", "waxon")
			case "darwin":
				return runStream("launchctl", "list", "dev.waxon.waxon")
			default:
				return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
			}
		},
	}
}

func serviceLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail waxon service logs",
		RunE: func(cmd *cobra.Command, args []string) error {
			switch runtime.GOOS {
			case "linux":
				args := []string{"--user", "-u", "waxon"}
				if follow {
					args = append(args, "-f")
				} else {
					args = append(args, "-n", "100")
				}
				return runStream("journalctl", args...)
			case "darwin":
				logFile := filepath.Join(os.Getenv("HOME"), "Library", "Logs", "waxon.log")
				if follow {
					return runStream("tail", "-f", logFile)
				}
				return runStream("tail", "-n", "100", logFile)
			default:
				return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
			}
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", true, "Follow log output")
	return cmd
}

// waxonBinaryPath returns the absolute path of the currently-running waxon
// binary, falling back to PATH lookup. We bake the absolute path into the
// service unit so the service is independent of $PATH at start time.
func waxonBinaryPath() (string, error) {
	if exe, err := os.Executable(); err == nil {
		if abs, err := filepath.Abs(exe); err == nil {
			return abs, nil
		}
	}
	if p, err := exec.LookPath("waxon"); err == nil {
		return filepath.Abs(p)
	}
	return "", fmt.Errorf("could not locate waxon binary")
}

func systemdUnitPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", "waxon.service"), nil
}

func installSystemd(out io.Writer, binPath, slidesFile string, port int, bind, theme string) error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), 0755); err != nil {
		return err
	}

	args := []string{"serve", slidesFile, "--no-open", "--port", strconv.Itoa(port), "--bind", bind}
	if theme != "" {
		args = append(args, "--theme", theme)
	}
	execLine := binPath + " " + strings.Join(args, " ")

	unit := fmt.Sprintf(`[Unit]
Description=waxon slide deck server (%s)
After=network.target

[Service]
Type=simple
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, filepath.Base(slidesFile), execLine)

	if err := os.WriteFile(unitPath, []byte(unit), 0644); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s Wrote %s\n", success.Sprint("✓"), bold.Sprint(unitPath))

	for _, c := range [][]string{
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "waxon"},
		{"systemctl", "--user", "restart", "waxon"},
	} {
		if err := runStream(c[0], c[1:]...); err != nil {
			return err
		}
	}

	fmt.Fprintf(out, "%s waxon service started on %s:%d\n", success.Sprint("✓"), bind, port)
	fmt.Fprintf(out, "\n%s\n", dim.Sprint("Useful commands:"))
	fmt.Fprintf(out, "  %s\n", info.Sprint("waxon service status"))
	fmt.Fprintf(out, "  %s\n", info.Sprint("waxon service logs"))
	fmt.Fprintf(out, "  %s\n", info.Sprint("waxon service uninstall"))
	return nil
}

func uninstallSystemd(out io.Writer) error {
	unitPath, err := systemdUnitPath()
	if err != nil {
		return err
	}
	// Best-effort stop and disable; ignore errors so uninstall is idempotent.
	_ = runStream("systemctl", "--user", "stop", "waxon")
	_ = runStream("systemctl", "--user", "disable", "waxon")

	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	_ = runStream("systemctl", "--user", "daemon-reload")
	fmt.Fprintf(out, "%s Removed waxon service\n", success.Sprint("✓"))
	return nil
}

func launchdPlistPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", "dev.waxon.waxon.plist"), nil
}

func installLaunchd(out io.Writer, binPath, slidesFile string, port int, bind, theme string) error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return err
	}
	logDir := filepath.Join(os.Getenv("HOME"), "Library", "Logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}
	logFile := filepath.Join(logDir, "waxon.log")

	args := []string{"serve", slidesFile, "--no-open", "--port", strconv.Itoa(port), "--bind", bind}
	if theme != "" {
		args = append(args, "--theme", theme)
	}
	progArgs := append([]string{binPath}, args...)

	var argXML strings.Builder
	for _, a := range progArgs {
		fmt.Fprintf(&argXML, "    <string>%s</string>\n", a)
	}

	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>dev.waxon.waxon</string>
  <key>ProgramArguments</key>
  <array>
%s  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>%s</string>
  <key>StandardErrorPath</key>
  <string>%s</string>
</dict>
</plist>
`, argXML.String(), logFile, logFile)

	if err := os.WriteFile(plistPath, []byte(plist), 0644); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s Wrote %s\n", success.Sprint("✓"), bold.Sprint(plistPath))

	// Reload: unload first (ignore errors), then load.
	_ = runStream("launchctl", "unload", plistPath)
	if err := runStream("launchctl", "load", plistPath); err != nil {
		return err
	}

	fmt.Fprintf(out, "%s waxon service started on %s:%d\n", success.Sprint("✓"), bind, port)
	fmt.Fprintf(out, "\n%s\n", dim.Sprint("Useful commands:"))
	fmt.Fprintf(out, "  %s\n", info.Sprint("waxon service status"))
	fmt.Fprintf(out, "  %s\n", info.Sprint("waxon service logs"))
	fmt.Fprintf(out, "  %s\n", info.Sprint("waxon service uninstall"))
	return nil
}

func uninstallLaunchd(out io.Writer) error {
	plistPath, err := launchdPlistPath()
	if err != nil {
		return err
	}
	_ = runStream("launchctl", "unload", plistPath)
	if err := os.Remove(plistPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintf(out, "%s Removed waxon service\n", success.Sprint("✓"))
	return nil
}

// runStream runs a command, streaming stdout/stderr to the parent process.
func runStream(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
