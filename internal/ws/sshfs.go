package ws

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// SSHFSManager handles mounting/unmounting VM filesystems via SSHFS.
type SSHFSManager struct {
	baseDir string // ~/.agentvm/mounts
	mu      sync.Mutex
	mounts  map[string]string // agentID -> mountPoint
}

// NewSSHFSManager creates a new SSHFS manager.
func NewSSHFSManager(baseDir string) *SSHFSManager {
	mountDir := filepath.Join(baseDir, "mounts")
	os.MkdirAll(mountDir, 0755)
	return &SSHFSManager{
		baseDir: mountDir,
		mounts:  make(map[string]string),
	}
}

// Mount mounts a VM's workspace directory via SSHFS using Lima's SSH config.
func (m *SSHFSManager) Mount(vmName, agentID, project, remotePath string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if mp, ok := m.mounts[agentID]; ok {
		return mp, nil // Already mounted
	}

	if remotePath == "" {
		remotePath = fmt.Sprintf("/home/%s.linux/workspace/%s", getLimaUser(), project)
	}

	mountPoint := filepath.Join(m.baseDir, agentID)
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		return "", fmt.Errorf("creating mount point: %w", err)
	}

	// Use Lima's SSH identity for authentication
	limaDir := filepath.Join(os.Getenv("HOME"), ".lima", vmName)
	sshKey := filepath.Join(limaDir, "ssh", "id_ed25519")
	if _, err := os.Stat(sshKey); os.IsNotExist(err) {
		// Fall back to common key names
		sshKey = filepath.Join(limaDir, "ssh", "id_rsa")
	}

	// Get SSH config from lima
	sshConfigPath := filepath.Join(limaDir, "ssh.config")

	// Build sshfs command
	args := []string{
		"-o", "IdentityFile=" + sshKey,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
	}

	// If lima ssh config exists, use it for port/host
	if _, err := os.Stat(sshConfigPath); err == nil {
		args = append(args, "-F", sshConfigPath)
		args = append(args, fmt.Sprintf("%s:%s", vmName, remotePath))
	} else {
		// Direct connection using lima's default
		args = append(args, fmt.Sprintf("%s@127.0.0.1:%s", getLimaUser(), remotePath))
		args = append(args, "-p", "0") // Lima manages ports
	}

	args = append(args, mountPoint)

	cmd := exec.Command("sshfs", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(mountPoint)
		return "", fmt.Errorf("sshfs mount failed: %w: %s", err, output)
	}

	m.mounts[agentID] = mountPoint
	log.Printf("SSHFS: mounted %s at %s", agentID, mountPoint)
	return mountPoint, nil
}

// Unmount unmounts a previously mounted VM filesystem.
func (m *SSHFSManager) Unmount(agentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mountPoint, ok := m.mounts[agentID]
	if !ok {
		return fmt.Errorf("agent %s is not mounted", agentID)
	}

	cmd := exec.Command("umount", mountPoint)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Try force unmount on macOS
		cmd2 := exec.Command("diskutil", "unmount", "force", mountPoint)
		if output2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return fmt.Errorf("unmount failed: %w: %s / %s", err, output, output2)
		}
	}

	os.Remove(mountPoint)
	delete(m.mounts, agentID)
	log.Printf("SSHFS: unmounted %s", agentID)
	return nil
}

// MountPoint returns the mount point for an agent, or empty string.
func (m *SSHFSManager) MountPoint(agentID string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.mounts[agentID]
}

// IsMounted returns whether an agent's filesystem is currently mounted.
func (m *SSHFSManager) IsMounted(agentID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.mounts[agentID]
	return ok
}

func getLimaUser() string {
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	return "lima"
}
