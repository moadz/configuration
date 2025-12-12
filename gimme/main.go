package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Config represents the gimme.yaml configuration file
type Config struct {
	Browser  string               `yaml:"browser"`
	VPN      string               `yaml:"vpn"`
	Clusters map[string][]Cluster `yaml:"clusters"`
}

// Cluster represents a single cluster configuration
type Cluster struct {
	Name         string `yaml:"name"`
	Region       string `yaml:"region"`
	Console      string `yaml:"console"`
	Prometheus   string `yaml:"prometheus"`
	Alertmanager string `yaml:"alertmanager"`
	CIDR         string `yaml:"cidr"`
	Bastion      string `yaml:"bastion"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

func defaultConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".config/gimme/gimme.yaml"
	}
	return filepath.Join(homeDir, ".config", "gimme", "gimme.yaml")
}

func findCluster(cfg *Config, env, region string) (*Cluster, error) {
	clusters, ok := cfg.Clusters[env]
	if !ok {
		validEnvs := make([]string, 0, len(cfg.Clusters))
		for k := range cfg.Clusters {
			validEnvs = append(validEnvs, k)
		}
		return nil, fmt.Errorf("unknown environment %q, valid environments: %v", env, validEnvs)
	}

	for i := range clusters {
		if clusters[i].Region == region {
			return &clusters[i], nil
		}
	}

	validRegions := make([]string, 0, len(clusters))
	for _, c := range clusters {
		validRegions = append(validRegions, c.Region)
	}
	return nil, fmt.Errorf("unknown region %q for environment %q, valid regions: %v", region, env, validRegions)
}

func runOSAScript(ctx context.Context, script string) (string, error) {
	slog.Debug("executing osascript", "script", script)
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	s := strings.TrimSpace(out.String())
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		slog.Error("osascript execution failed", "error", msg, "script", script)
		return s, fmt.Errorf("osascript failed: %s (script=%q)", msg, script)
	}
	slog.Debug("osascript executed successfully", "output", s)
	return s, nil
}

func viscosityState(ctx context.Context, vpnName string) (string, error) {
	slog.Debug("checking VPN state", "vpn", vpnName)
	// Important: select the connection object first, then ask for its state.
	script := fmt.Sprintf(
		`tell application "Viscosity" to get state of first connection whose name is %q`,
		vpnName,
	)
	state, err := runOSAScript(ctx, script)
	if err != nil {
		slog.Error("failed to get VPN state", "vpn", vpnName, "error", err)
		return state, err
	}
	slog.Debug("got VPN state", "vpn", vpnName, "state", state)
	return state, nil
}

func viscosityConnect(ctx context.Context, vpnName string) error {
	slog.Info("initiating VPN connection", "vpn", vpnName)
	script := fmt.Sprintf(`tell application "Viscosity" to connect %q`, vpnName)
	_, err := runOSAScript(ctx, script)
	if err != nil {
		slog.Error("failed to initiate VPN connection", "vpn", vpnName, "error", err)
		return err
	}
	slog.Debug("VPN connection initiated", "vpn", vpnName)
	return nil
}

// ensureViscosityConnected checks state; if disconnected it connects and verifies Connected.
// waitTimeout is the max time to wait for Connected after initiating connect.
func ensureViscosityConnected(ctx context.Context, vpnName string, waitTimeout time.Duration) error {
	slog.Info("ensuring VPN is connected", "vpn", vpnName, "timeout", waitTimeout)
	state, err := viscosityState(ctx, vpnName)
	if err != nil {
		return err
	}

	// Normalize common outputs just in case.
	state = strings.TrimSpace(state)

	if strings.EqualFold(state, "Connected") {
		slog.Info("VPN already connected", "vpn", vpnName)
		return nil
	}

	if strings.EqualFold(state, "Disconnected") {
		slog.Info("VPN is disconnected, attempting to connect", "vpn", vpnName)
		if err := viscosityConnect(ctx, vpnName); err != nil {
			return err
		}
	} else {
		// e.g. "Connecting", "Disconnecting" — we can just wait it out.
		slog.Info("VPN in transitional state, waiting", "vpn", vpnName, "state", state)
	}

	slog.Debug("starting connection polling", "vpn", vpnName, "timeout", waitTimeout)
	deadline := time.Now().Add(waitTimeout)
	for {
		if time.Now().After(deadline) {
			last, _ := viscosityState(ctx, vpnName)
			slog.Error("timed out waiting for VPN connection", "vpn", vpnName, "lastState", strings.TrimSpace(last))
			return fmt.Errorf("timed out waiting for VPN %q to connect; last state=%q", vpnName, strings.TrimSpace(last))
		}

		time.Sleep(750 * time.Millisecond)

		state, err := viscosityState(ctx, vpnName)
		if err != nil {
			// transient AppleScript failures happen; keep retrying until timeout
			slog.Warn("transient error checking VPN state, retrying", "vpn", vpnName, "error", err)
			continue
		}
		state = strings.TrimSpace(state)

		if strings.EqualFold(state, "Connected") {
			slog.Info("VPN connection established", "vpn", vpnName)
			return nil
		}
		if strings.EqualFold(state, "Disconnected") {
			// If it fell back to disconnected, something went wrong.
			// Maybe user cancelled the connection.
			slog.Error("VPN returned to disconnected after connect attempt", "vpn", vpnName)
			return errors.New("VPN returned to Disconnected after connect attempt")
		}
		// otherwise keep polling (Connecting, etc.)
		slog.Debug("VPN still connecting, polling", "vpn", vpnName, "state", state)
	}
}

func runCommandLogged(ctx context.Context, name string, args ...string) error {
	slog.Info("executing command", "cmd", name, "args", args)
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func addRouteBestEffort(ctx context.Context, cidr string) {
	// sudo route add -net <CIDR> -interface en0
	slog.Info("adding route", "cidr", cidr, "interface", "en0")
	err := runCommandLogged(ctx, "sudo", "route", "add", "-net", cidr, "-interface", "en0")
	if err != nil {
		// This can fail if the route already exists; that's fine per your requirement.
		slog.Warn("route add failed (continuing)", "cidr", cidr, "error", err)
	} else {
		slog.Info("route add succeeded", "cidr", cidr)
	}
}

// startSSHuttle starts sshuttle and returns the command and a channel that closes when connected.
// The ready channel will be closed when sshuttle outputs "Connected" or the process exits.
func startSSHuttle(ctx context.Context, bastion, cidr string) (*exec.Cmd, <-chan struct{}, error) {
	slog.Info("starting sshuttle", "bastion", bastion, "cidr", cidr)

	slog.Info("executing command", "cmd", "sshuttle", "args", []string{"-r", bastion, cidr})
	cmd := exec.CommandContext(ctx, "sshuttle", "-r", bastion, cidr)

	// Connect stdin so the user can enter passwords (sudo and/or SSH)
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		slog.Error("failed to create stdout pipe", "error", err)
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		slog.Error("failed to create stderr pipe", "error", err)
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		slog.Error("failed to start sshuttle", "error", err)
		return nil, nil, err
	}

	slog.Debug("sshuttle process started", "pid", cmd.Process.Pid)

	// Channel that signals when sshuttle is connected
	ready := make(chan struct{})
	var readyOnce sync.Once

	// Stream stdout in background
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			line := sc.Text()
			slog.Info("sshuttle output", "message", line)
		}
	}()

	// Stream stderr and watch for "Connected" message
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			line := sc.Text()
			// Print to stderr so user sees password prompts and status
			fmt.Fprintln(os.Stderr, line)
			slog.Debug("sshuttle stderr", "message", line)

			// sshuttle prints "Connected to server." when the tunnel is up
			if strings.Contains(line, "Connected") {
				readyOnce.Do(func() {
					slog.Info("sshuttle tunnel established")
					close(ready)
				})
			}
		}
		// If stderr closes without "Connected", close ready anyway to unblock
		readyOnce.Do(func() { close(ready) })
	}()

	return cmd, ready, nil
}

func openURLInBrowser(ctx context.Context, browserApp, url string) error {
	// macOS: open -a "Google Chrome" "https://..."
	slog.Info("opening URL in browser", "browser", browserApp, "url", url)
	return runCommandLogged(ctx, "open", "-a", browserApp, url)
}

var (
	configFile   string
	sshuttleWait time.Duration
)

var rootCmd = &cobra.Command{
	Use:   "gimme <env> <region>",
	Short: "Give me a private AppSRE cluster",
	Long: `gimme connects to a private AppSRE cluster by:
1. Ensuring VPN is connected via Viscosity
2. Adding network route for the cluster CIDR
3. Starting sshuttle tunnel through bastion
4. Opening console, prometheus, and alertmanager in browser

Ensure you are on MacOS, and have Viscosity installed.
Also ensure you have a valid gimme.yaml config file.

Arguments:
  env     Environment: stage, int, or prod
  region  Region code (e.g., ue1, uw2)

Examples:
  gimme stage ue1    # Connect to staging cluster in us-east-1
  gimme int uw2      # Connect to integration cluster in us-west-2
`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		env := args[0]
		region := args[1]

		slog.Info("loading config", "path", configFile)
		cfg, err := loadConfig(configFile)
		if err != nil {
			return err
		}

		// Find the cluster
		cluster, err := findCluster(cfg, env, region)
		if err != nil {
			return err
		}

		slog.Info("selected cluster",
			"name", cluster.Name,
			"env", env,
			"region", region,
			"cidr", cluster.CIDR,
			"bastion", cluster.Bastion,
		)

		ctx := context.Background()

		// Ensure VPN is connected
		if err := ensureViscosityConnected(ctx, cfg.VPN, 30*time.Second); err != nil {
			slog.Error("failed to ensure VPN connection", "error", err)
			return err
		}
		slog.Info("VPN is connected")

		// Add route for cluster CIDR
		addRouteBestEffort(ctx, cluster.CIDR)

		// Start sshuttle
		sshuttleCmd, ready, err := startSSHuttle(ctx, cluster.Bastion, cluster.CIDR)
		if err != nil {
			slog.Error("sshuttle failed to start", "error", err)
			return err
		}

		// Wait for sshuttle to connect (or timeout)
		slog.Info("waiting for sshuttle to establish tunnel (enter password if prompted)", "timeout", sshuttleWait)
		select {
		case <-ready:
			slog.Info("sshuttle signaled ready")
		case <-time.After(sshuttleWait):
			slog.Warn("sshuttle wait timeout reached, proceeding anyway")
		}

		// Open all cluster URLs in browser
		urls := []struct {
			name string
			url  string
		}{
			{"console", cluster.Console},
			{"prometheus", cluster.Prometheus},
			{"alertmanager", cluster.Alertmanager},
		}

		for _, u := range urls {
			if u.url != "" {
				if err := openURLInBrowser(ctx, cfg.Browser, u.url); err != nil {
					slog.Warn("failed to open URL", "name", u.name, "url", u.url, "error", err)
					// keep sshuttle running; opening the browser is not critical
				}
			}
		}

		// Keep the program alive as long as sshuttle runs.
		if err := sshuttleCmd.Wait(); err != nil {
			slog.Error("sshuttle exited with error", "error", err)
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", defaultConfigPath(), "Path to gimme.yaml config file")
	rootCmd.Flags().DurationVarP(&sshuttleWait, "sshuttle-wait", "w", 60*time.Second, "Max time to wait for sshuttle to connect")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
