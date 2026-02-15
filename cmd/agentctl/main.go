package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/mateo/agentvm/internal/api"
	"github.com/mateo/agentvm/internal/config"
	"github.com/mateo/agentvm/internal/lima"
	"github.com/spf13/cobra"
)

var cfg config.Config

func main() {
	cfg, _ = config.Load()

	root := &cobra.Command{
		Use:   "agentctl",
		Short: "Control the agent VM orchestration system",
	}

	root.AddCommand(
		masterCmd(),
		dispatchCmd(),
		statusCmd(),
		poolCmd(),
		logsCmd(),
		shellCmd(),
		killCmd(),
		setupCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// --- master ---

func masterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "master",
		Short: "Manage the golden master VM",
	}
	cmd.AddCommand(masterCreateCmd(), masterStatusCmd(), masterUpdateCmd(), masterSyncCmd())
	return cmd
}

func masterCreateCmd() *cobra.Command {
	var cpus, memGiB, diskGiB int
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create the golden master VM from template",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := lima.NewClient()
			if err != nil {
				return err
			}

			if err := config.EnsureDirs(); err != nil {
				return err
			}

			// Render the template
			tmplCfg := lima.DefaultTemplateConfig()
			if cpus > 0 {
				tmplCfg.CPUs = cpus
			}
			if memGiB > 0 {
				tmplCfg.MemoryGiB = memGiB
			}
			if diskGiB > 0 {
				tmplCfg.DiskGiB = diskGiB
			}

			templatePath := filepath.Join(config.BaseDir(), "agent-master.yaml")
			fmt.Println("Rendering Lima template...")
			if err := lima.RenderTemplate(tmplCfg, templatePath); err != nil {
				return fmt.Errorf("rendering template: %w", err)
			}

			// Check if master already exists
			ctx := context.Background()
			if inst, err := client.Get(ctx, cfg.VM.Master); err == nil {
				return fmt.Errorf("master VM %q already exists (status: %s). Delete it first or use 'master update'", inst.Name, inst.Status)
			}

			fmt.Printf("Creating golden master VM %q (this may take several minutes)...\n", cfg.VM.Master)
			err = client.Create(ctx, lima.CreateOptions{
				Name:     cfg.VM.Master,
				Template: templatePath,
				Start:    false,
				Timeout:  15 * time.Minute,
			})
			if err != nil {
				return fmt.Errorf("creating master: %w", err)
			}

			fmt.Printf("Starting master VM %q...\n", cfg.VM.Master)
			if err := client.Start(ctx, cfg.VM.Master); err != nil {
				return fmt.Errorf("starting master: %w", err)
			}

			// Verify Docker works
			fmt.Println("Verifying Docker inside VM...")
			output, err := client.Shell(ctx, lima.ShellOptions{
				Instance: cfg.VM.Master,
				Command:  "docker",
				Args:     []string{"run", "--rm", "hello-world"},
				Timeout:  2 * time.Minute,
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Docker verification failed: %v\n", err)
			} else {
				fmt.Println("Docker verified successfully")
				_ = output
			}

			// Stop the master (it's a template, not for running)
			fmt.Println("Stopping master VM (used as clone source)...")
			if err := client.Stop(ctx, cfg.VM.Master); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to stop master: %v\n", err)
			}

			fmt.Printf("\nGolden master %q created and ready for cloning.\n", cfg.VM.Master)
			return nil
		},
	}
	cmd.Flags().IntVar(&cpus, "cpus", 0, "Override CPU count (default from config)")
	cmd.Flags().IntVar(&memGiB, "memory", 0, "Override memory in GiB (default from config)")
	cmd.Flags().IntVar(&diskGiB, "disk", 0, "Override disk in GiB (default from config)")
	return cmd
}

func masterStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show golden master VM status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := lima.NewClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			inst, err := client.Get(ctx, cfg.VM.Master)
			if err != nil {
				return fmt.Errorf("master VM %q not found. Run 'agentctl master create' first", cfg.VM.Master)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintf(w, "Name:\t%s\n", inst.Name)
			fmt.Fprintf(w, "Status:\t%s\n", inst.Status)
			fmt.Fprintf(w, "Arch:\t%s\n", inst.Arch)
			fmt.Fprintf(w, "CPUs:\t%d\n", inst.CPUs)
			fmt.Fprintf(w, "Memory:\t%d bytes\n", inst.Memory)
			fmt.Fprintf(w, "Disk:\t%d bytes\n", inst.Disk)
			fmt.Fprintf(w, "Dir:\t%s\n", inst.Dir)
			if inst.SSHAddr != "" {
				fmt.Fprintf(w, "SSH:\t%s\n", inst.SSHAddr)
			}
			w.Flush()
			return nil
		},
	}
}

func masterUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Update the golden master VM (re-run provisioning)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := lima.NewClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			inst, err := client.Get(ctx, cfg.VM.Master)
			if err != nil {
				return fmt.Errorf("master VM not found: %w", err)
			}

			if inst.Status != lima.StatusRunning {
				fmt.Println("Starting master VM...")
				if err := client.Start(ctx, cfg.VM.Master); err != nil {
					return err
				}
			}

			fmt.Println("Re-syncing harness binary...")
			_, err = client.Shell(ctx, lima.ShellOptions{
				Instance: cfg.VM.Master,
				Command:  "sudo",
				Args:     []string{"cp", "/mnt/host-shared/bin/agent-harness", "/usr/local/bin/agent-harness"},
				Timeout:  30 * time.Second,
			})
			if err != nil {
				return fmt.Errorf("syncing harness: %w", err)
			}

			fmt.Println("Stopping master VM...")
			return client.Stop(ctx, cfg.VM.Master)
		},
	}
}

func masterSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync agent-harness binary to master VM shared mount",
		RunE: func(cmd *cobra.Command, args []string) error {
			src := "bin/agent-harness"
			dst := filepath.Join(config.BaseDir(), "shared", "bin", "agent-harness")

			if _, err := os.Stat(src); err != nil {
				return fmt.Errorf("harness binary not found at %s. Run 'make build-harness' first", src)
			}

			data, err := os.ReadFile(src)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
				return err
			}

			if err := os.WriteFile(dst, data, 0755); err != nil {
				return err
			}

			fmt.Printf("Synced harness binary to %s\n", dst)
			return nil
		},
	}
}

// --- dispatch ---

func dispatchCmd() *cobra.Command {
	var req api.DispatchRequest
	cmd := &cobra.Command{
		Use:   "dispatch",
		Short: "Dispatch a task to a new agent",
		RunE: func(cmd *cobra.Command, args []string) error {
			if req.Project == "" || req.RepoURL == "" || req.Prompt == "" {
				return fmt.Errorf("--project, --repo, and --prompt are required")
			}
			if req.Tool == "" {
				req.Tool = "claude-code"
			}

			client := api.NewClient(cfg.API.Port)
			resp, err := client.Dispatch(req)
			if err != nil {
				return fmt.Errorf("dispatch failed: %w", err)
			}

			fmt.Printf("Agent dispatched:\n")
			fmt.Printf("  Agent ID:  %s\n", resp.AgentID)
			fmt.Printf("  VM:        %s\n", resp.VMName)
			fmt.Printf("  IP:        %s\n", resp.VMIP)
			fmt.Printf("  URL:       https://%s\n", resp.Subdomain)
			return nil
		},
	}
	cmd.Flags().StringVar(&req.Project, "project", "", "Project name")
	cmd.Flags().StringVar(&req.RepoURL, "repo", "", "Git repository URL")
	cmd.Flags().StringVar(&req.Issue, "issue", "", "Issue identifier (e.g. PROJ-123)")
	cmd.Flags().StringVar(&req.Tool, "tool", "claude-code", "Coding tool: claude-code, opencode, amp, cline")
	cmd.Flags().StringVar(&req.Prompt, "prompt", "", "Task prompt")
	cmd.Flags().StringVar(&req.Branch, "branch", "", "Branch name (auto-generated if empty)")
	cmd.Flags().IntVar(&req.MaxTime, "max-time", 30, "Max execution time in minutes")
	return cmd
}

// --- status ---

func statusCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show pool and agent status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient(cfg.API.Port)
			status, err := client.Status()
			if err != nil {
				return fmt.Errorf("failed to get status: %w", err)
			}

			if jsonOutput {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(status)
			}

			fmt.Printf("Pool: %d warm | %d active | %d cold\n", status.Warm, status.Active, status.Cold)
			if len(status.Agents) > 0 {
				fmt.Println("\nActive Agents:")
				w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
				fmt.Fprintf(w, "ID\tVM\tPROJECT\tTOOL\tSTATE\tELAPSED\n")
				for _, a := range status.Agents {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
						a.AgentID, a.VMName, a.Project, a.Tool, a.State,
						a.Elapsed.Round(time.Second))
				}
				w.Flush()
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output as JSON")
	return cmd
}

// --- pool ---

func poolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Pool management commands",
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show pool status",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient(cfg.API.Port)
			status, err := client.Status()
			if err != nil {
				return err
			}
			fmt.Printf("Warm: %d | Active: %d | Cold: %d\n", status.Warm, status.Active, status.Cold)
			return nil
		},
	}

	replenishCmd := &cobra.Command{
		Use:   "replenish",
		Short: "Trigger pool replenishment",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient(cfg.API.Port)
			return client.PoolReplenish()
		},
	}

	drainCmd := &cobra.Command{
		Use:   "drain",
		Short: "Drain all warm VMs",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient(cfg.API.Port)
			return client.PoolDrain()
		},
	}

	var warmSize int
	resizeCmd := &cobra.Command{
		Use:   "resize",
		Short: "Resize warm pool",
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient(cfg.API.Port)
			return client.PoolResize(warmSize)
		},
	}
	resizeCmd.Flags().IntVar(&warmSize, "warm", 3, "New warm pool size")

	cmd.AddCommand(statusCmd, replenishCmd, drainCmd, resizeCmd)
	return cmd
}

// --- logs ---

func logsCmd() *cobra.Command {
	var follow, execution bool
	cmd := &cobra.Command{
		Use:   "logs <agent-id>",
		Short: "Fetch agent logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient(cfg.API.Port)
			reader, err := client.Logs(args[0], follow, execution)
			if err != nil {
				return err
			}
			defer reader.Close()
			_, err = io.Copy(os.Stdout, reader)
			return err
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().BoolVar(&execution, "execution", false, "Show execution output only")
	return cmd
}

// --- shell ---

func shellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <agent-id-or-vm>",
		Short: "Open interactive shell into agent VM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Execute limactl shell interactively
			return execLimaShell(args[0])
		},
	}
}

func execLimaShell(name string) error {
	binary, err := lima.FindLimactl()
	if err != nil {
		return err
	}
	return runInteractive(binary, "shell", name)
}

// --- kill ---

func killCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "kill <agent-id>",
		Short: "Kill an active agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client := api.NewClient(cfg.API.Port)
			if err := client.Kill(args[0]); err != nil {
				return err
			}
			fmt.Printf("Agent %s killed\n", args[0])
			return nil
		},
	}
}

// --- setup ---

func setupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "First-time setup: create directories, generate config, install dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Setting up agentvm...")

			if err := config.EnsureDirs(); err != nil {
				return fmt.Errorf("creating directories: %w", err)
			}
			fmt.Println("  Created ~/.agentvm directory structure")

			if _, err := os.Stat(config.ConfigPath()); os.IsNotExist(err) {
				if err := config.Save(config.Default()); err != nil {
					return fmt.Errorf("saving default config: %w", err)
				}
				fmt.Println("  Created default config.yaml")
			} else {
				fmt.Println("  Config already exists, skipping")
			}

			// Check dependencies
			deps := []string{"limactl", "docker", "traefik", "mkcert"}
			for _, d := range deps {
				if _, err := findBinary(d); err != nil {
					fmt.Printf("  Warning: %s not found in PATH\n", d)
				} else {
					fmt.Printf("  Found %s\n", d)
				}
			}

			fmt.Println("\nSetup complete. Next steps:")
			fmt.Println("  1. make build-harness && make install-harness")
			fmt.Println("  2. agentctl master create")
			fmt.Println("  3. Start agentd daemon")
			return nil
		},
	}
}

func findBinary(name string) (string, error) {
	return lima.FindBinary(name)
}
