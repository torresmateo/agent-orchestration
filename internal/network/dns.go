package network

import (
	"fmt"
	"os"
	"path/filepath"
)

func WriteDnsmasqConfig(domain string) error {
	configDir := "/opt/homebrew/etc/dnsmasq.d"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating dnsmasq config dir: %w", err)
	}

	config := fmt.Sprintf("address=/%s/127.0.0.1\n", domain)
	configPath := filepath.Join(configDir, "agentvm.conf")
	return os.WriteFile(configPath, []byte(config), 0644)
}

func WriteResolverConfig(domain string) error {
	resolverDir := "/etc/resolver"
	config := fmt.Sprintf("nameserver 127.0.0.1\nport 53\n")
	configPath := filepath.Join(resolverDir, domain)

	// This requires sudo, so we just generate the content
	// The setup script handles the actual file creation
	fmt.Printf("To enable DNS resolution, run:\n")
	fmt.Printf("  sudo mkdir -p %s\n", resolverDir)
	fmt.Printf("  echo '%s' | sudo tee %s\n", config, configPath)

	return nil
}
