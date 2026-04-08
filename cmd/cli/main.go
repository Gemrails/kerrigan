package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

var (
	version = "dev"
)

func main() {
	app := &cli.App{
		Name:    "kerrigan-cli",
		Version: version,
		Usage:   "Kerrigan v2 CLI Client",
		Commands: []*cli.Command{
			{
				Name:  "node",
				Usage: "Node management",
				Subcommands: []*cli.Command{
					{
						Name:   "status",
						Usage:  "Show node status",
						Action: nodeStatus,
					},
					{
						Name:   "info",
						Usage:  "Show node information",
						Action: nodeInfo,
					},
				},
			},
			{
				Name:  "resource",
				Usage: "Resource management",
				Subcommands: []*cli.Command{
					{
						Name:   "list",
						Usage:  "List available resources",
						Action: listResources,
					},
					{
						Name:  "subscribe",
						Usage: "Subscribe to resources",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "type",
								Usage: "Resource type: gpu, storage, bandwidth",
								Value: "gpu",
							},
							&cli.IntFlag{
								Name:  "amount",
								Usage: "Amount",
								Value: 1,
							},
						},
						Action: subscribeResource,
					},
				},
			},
			{
				Name:  "plugin",
				Usage: "Plugin management",
				Subcommands: []*cli.Command{
					{
						Name:   "list",
						Usage:  "List installed plugins",
						Action: listPlugins,
					},
					{
						Name:      "install",
						Usage:     "Install a plugin",
						Args:      true,
						ArgsUsage: "<plugin-id>",
						Action:    installPlugin,
					},
					{
						Name:      "uninstall",
						Usage:     "Uninstall a plugin",
						Args:      true,
						ArgsUsage: "<plugin-id>",
						Action:    uninstallPlugin,
					},
				},
			},
			{
				Name:  "task",
				Usage: "Task management",
				Subcommands: []*cli.Command{
					{
						Name:   "list",
						Usage:  "List tasks",
						Action: listTasks,
					},
					{
						Name:   "submit",
						Usage:  "Submit a new task",
						Action: submitTask,
					},
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Placeholder actions
func nodeStatus(c *cli.Context) error {
	fmt.Println("Node Status: Running")
	return nil
}

func nodeInfo(c *cli.Context) error {
	fmt.Println("Node Info:")
	fmt.Println("  Node ID: ...")
	fmt.Println("  Roles: provider, consumer")
	fmt.Println("  Version: ", version)
	return nil
}

func listResources(c *cli.Context) error {
	fmt.Println("Available Resources:")
	fmt.Println("  GPU:")
	fmt.Println("    - RTX 4090 x 2 (available: 1)")
	fmt.Println("    - A100 40G x 1 (available: 0)")
	fmt.Println("  Storage: 500 GB available")
	fmt.Println("  Bandwidth: 100 Mbps")
	return nil
}

func subscribeResource(c *cli.Context) error {
	resourceType := c.String("type")
	amount := c.Int("amount")
	fmt.Printf("Subscribing to %d %s resources...\n", amount, resourceType)
	return nil
}

func listPlugins(c *cli.Context) error {
	fmt.Println("Installed Plugins:")
	fmt.Println("  - gpu-share v1.0.0")
	fmt.Println("  - storage v1.0.0")
	return nil
}

func installPlugin(c *cli.Context) error {
	pluginID := c.Args().First()
	fmt.Printf("Installing plugin: %s\n", pluginID)
	return nil
}

func uninstallPlugin(c *cli.Context) error {
	pluginID := c.Args().First()
	fmt.Printf("Uninstalling plugin: %s\n", pluginID)
	return nil
}

func listTasks(c *cli.Context) error {
	fmt.Println("Tasks:")
	fmt.Println("  - Task 1: LLM Inference (completed)")
	fmt.Println("  - Task 2: Storage Pinning (running)")
	return nil
}

func submitTask(c *cli.Context) error {
	fmt.Println("Submitting task...")
	return nil
}
