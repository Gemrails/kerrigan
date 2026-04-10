package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kerrigan/kerrigan/internal/core"
	"github.com/kerrigan/kerrigan/pkg/log"
	"github.com/kerrigan/kerrigan/pkg/utils"
	"github.com/urfave/cli/v2"
)

var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	app := &cli.App{
		Name:    "kerrigan-node",
		Version: version,
		Usage:   "Kerrigan v2 - Distributed Resource Trading Network",
		Before: func(c *cli.Context) error {
			// Initialize logger
			logLevel := c.String("log-level")
			logFormat := c.String("log-format")
			log.Init(logLevel, logFormat)
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to configuration file",
				Value:   "configs/node.yaml",
			},
			&cli.StringFlag{
				Name:    "log-level",
				Aliases: []string{"l"},
				Usage:   "Log level: debug, info, warn, error",
				Value:   "info",
			},
			&cli.StringFlag{
				Name:    "log-format",
				Aliases: []string{"f"},
				Usage:   "Log format: text, json",
				Value:   "text",
			},
			&cli.StringFlag{
				Name:  "data-dir",
				Usage: "Data directory",
				Value: "",
			},
			&cli.StringFlag{
				Name:    "roles",
				Aliases: []string{"r"},
				Usage:   "Node roles: provider, consumer, relay, validator (comma-separated)",
				Value:   "provider",
			},
			&cli.StringFlag{
				Name:  "plugins",
				Usage: "Enabled plugins (comma-separated)",
				Value: "",
			},
			&cli.BoolFlag{
				Name:  "genesis",
				Usage: "Start as genesis node",
			},
			&cli.StringFlag{
				Name:  "seed-nodes",
				Usage: "Seed nodes for bootstrap (comma-separated)",
				Value: "",
			},
			&cli.IntFlag{
				Name:  "control-port",
				Usage: "Control plane port",
				Value: 38888,
			},
			&cli.IntFlag{
				Name:  "data-port",
				Usage: "Data plane port",
				Value: 38889,
			},
		},
		Action: runNode,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runNode(c *cli.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Load configuration
	_ = c.String("config") // config path for future use
	dataDir := c.String("data-dir")
	if dataDir == "" {
		dataDir = utils.GetDataDir("")
	}

	log.Info("===========================================")
	log.Info("  Kerrigan v2 - Distributed Resource Network")
	log.Infof("  Version: %s", version)
	log.Infof("  Build Time: %s", buildTime)
	log.Info("===========================================")
	log.Infof("Data directory: %s", dataDir)

	// Ensure data directory exists
	if err := utils.EnsureDir(dataDir); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}

	// Parse roles
	roles := parseRoles(c.String("roles"))

	// Create node configuration
	nodeConfig := &core.NodeConfig{
		NodeID:         utils.GetNodeID(),
		Roles:          roles,
		Genesis:        c.Bool("genesis"),
		DataDir:        dataDir,
		ControlPort:    c.Int("control-port"),
		DataPort:       c.Int("data-port"),
		SeedNodes:      parseSeedNodes(c.String("seed-nodes")),
		EnabledPlugins: parsePlugins(c.String("plugins")),
	}

	log.Infof("Node ID: %s", nodeConfig.NodeID)
	log.Infof("Roles: %v", roles)

	// Create and start node
	node, err := core.NewNode(ctx, nodeConfig)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}

	// Start node
	if err := node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}

	log.Info("Node started successfully")

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		log.Infof("Received signal: %v", sig)
	case <-ctx.Done():
		log.Info("Context cancelled")
	}

	// Graceful shutdown
	log.Info("Shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := node.Stop(shutdownCtx); err != nil {
		log.Errorf("Error during shutdown: %v", err)
	}

	log.Info("Node stopped")
	return nil
}

func parseRoles(rolesStr string) []core.NodeRole {
	var roles []core.NodeRole
	switch rolesStr {
	case "provider":
		roles = append(roles, core.RoleProvider)
	case "consumer":
		roles = append(roles, core.RoleConsumer)
	case "relay":
		roles = append(roles, core.RoleRelay)
	case "validator":
		roles = append(roles, core.RoleValidator)
	default:
		roles = append(roles, core.RoleProvider)
	}
	return roles
}

func parseSeedNodes(seedStr string) []string {
	if seedStr == "" {
		return nil
	}
	var nodes []string
	// Simple comma split - in production use more robust parsing
	for _, n := range seedStr {
		if string(n) == "," {
			continue
		}
	}
	return nodes
}

func parsePlugins(pluginsStr string) []string {
	if pluginsStr == "" {
		return []string{"gpu-share", "storage"}
	}
	// Simple comma split
	var plugins []string
	start := 0
	for i, c := range pluginsStr {
		if c == ',' {
			plugins = append(plugins, pluginsStr[start:i])
			start = i + 1
		}
	}
	if start < len(pluginsStr) {
		plugins = append(plugins, pluginsStr[start:])
	}
	return plugins
}
