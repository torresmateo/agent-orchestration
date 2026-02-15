package lima

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Client interface {
	Create(ctx context.Context, opts CreateOptions) error
	Clone(ctx context.Context, opts CloneOptions) error
	Start(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	Delete(ctx context.Context, name string, force bool) error
	List(ctx context.Context) ([]Instance, error)
	Get(ctx context.Context, name string) (*Instance, error)
	Shell(ctx context.Context, opts ShellOptions) (string, error)
	Copy(ctx context.Context, opts CopyOptions) error
}

type client struct {
	limactlPath string
}

func NewClient() (Client, error) {
	path, err := exec.LookPath("limactl")
	if err != nil {
		return nil, fmt.Errorf("limactl not found in PATH: %w", err)
	}
	return &client{limactlPath: path}, nil
}

func (c *client) run(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.limactlPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("limactl %s failed: %w\nstderr: %s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.String(), nil
}

func (c *client) Create(ctx context.Context, opts CreateOptions) error {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	args := []string{"create"}
	if opts.Template != "" {
		args = append(args, opts.Template)
	}
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	if opts.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", opts.CPUs))
	}
	if opts.Memory != "" {
		args = append(args, "--memory", opts.Memory)
	}
	if opts.Disk != "" {
		args = append(args, "--disk", opts.Disk)
	}
	args = append(args, "--tty=false")

	_, err := c.run(ctx, args...)
	if err != nil {
		return err
	}

	if opts.Start {
		return c.Start(ctx, opts.Name)
	}
	return nil
}

func (c *client) Clone(ctx context.Context, opts CloneOptions) error {
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	_, err := c.run(ctx, "clone", opts.Source, opts.Target)
	if err != nil {
		return err
	}

	if opts.Start {
		return c.Start(ctx, opts.Target)
	}
	return nil
}

func (c *client) Start(ctx context.Context, name string) error {
	_, err := c.run(ctx, "start", name)
	return err
}

func (c *client) Stop(ctx context.Context, name string) error {
	_, err := c.run(ctx, "stop", name)
	return err
}

func (c *client) Delete(ctx context.Context, name string, force bool) error {
	args := []string{"delete", name}
	if force {
		args = append(args, "--force")
	}
	_, err := c.run(ctx, args...)
	return err
}

type limaListEntry struct {
	Name      string `json:"name"`
	Status    string `json:"status"`
	Dir       string `json:"dir"`
	Arch      string `json:"arch"`
	CPUs      int    `json:"cpus"`
	Memory    int64  `json:"memory"`
	Disk      int64  `json:"disk"`
	SSHAddr   string `json:"sshAddress"`
	Networks  []struct {
		VNL       string `json:"vnl"`
		Interface string `json:"interface"`
		MACAddr   string `json:"macAddress"`
	} `json:"network"`
}

func (c *client) List(ctx context.Context) ([]Instance, error) {
	output, err := c.run(ctx, "list", "--json")
	if err != nil {
		return nil, err
	}

	var entries []limaListEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var entry limaListEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		entries = append(entries, entry)
	}

	instances := make([]Instance, 0, len(entries))
	for _, e := range entries {
		inst := Instance{
			Name:    e.Name,
			Status:  InstanceStatus(e.Status),
			Dir:     e.Dir,
			Arch:    e.Arch,
			CPUs:    e.CPUs,
			Memory:  e.Memory,
			Disk:    e.Disk,
			SSHAddr: e.SSHAddr,
		}
		for _, n := range e.Networks {
			inst.Network = append(inst.Network, NetworkInfo{
				VNL:       n.VNL,
				Interface: n.Interface,
				MACAddr:   n.MACAddr,
			})
		}
		instances = append(instances, inst)
	}
	return instances, nil
}

func (c *client) Get(ctx context.Context, name string) (*Instance, error) {
	instances, err := c.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, inst := range instances {
		if inst.Name == name {
			return &inst, nil
		}
	}
	return nil, fmt.Errorf("instance %q not found", name)
}

func (c *client) Shell(ctx context.Context, opts ShellOptions) (string, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	args := []string{"shell", opts.Instance}
	if opts.Command != "" {
		args = append(args, "--", opts.Command)
		args = append(args, opts.Args...)
	}
	return c.run(ctx, args...)
}

func (c *client) Copy(ctx context.Context, opts CopyOptions) error {
	var src, dst string
	switch opts.Direction {
	case CopyToVM:
		src = opts.LocalPath
		dst = fmt.Sprintf("%s:%s", opts.Instance, opts.VMPath)
	case CopyFromVM:
		src = fmt.Sprintf("%s:%s", opts.Instance, opts.VMPath)
		dst = opts.LocalPath
	default:
		return fmt.Errorf("invalid copy direction: %d", opts.Direction)
	}
	_, err := c.run(ctx, "copy", src, dst)
	return err
}

func FindLimactl() (string, error) {
	return exec.LookPath("limactl")
}

func FindBinary(name string) (string, error) {
	return exec.LookPath(name)
}

func GetVMIP(ctx context.Context, c Client, name string) (string, error) {
	output, err := c.Shell(ctx, ShellOptions{
		Instance: name,
		Command:  "hostname",
		Args:     []string{"-I"},
		Timeout:  10 * time.Second,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get IP for %s: %w", name, err)
	}
	parts := strings.Fields(strings.TrimSpace(output))
	if len(parts) == 0 {
		return "", fmt.Errorf("no IP address found for %s", name)
	}
	return parts[0], nil
}
