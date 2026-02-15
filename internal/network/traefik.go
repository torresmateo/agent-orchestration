package network

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mateo/agentvm/internal/registry"
)

type TraefikWriter struct {
	dynamicDir string
	domain     string
}

func NewTraefikWriter(baseDir, domain string) *TraefikWriter {
	return &TraefikWriter{
		dynamicDir: filepath.Join(baseDir, "traefik", "dynamic"),
		domain:     domain,
	}
}

func (tw *TraefikWriter) WriteRoute(reg *registry.AgentRegistration) error {
	if err := os.MkdirAll(tw.dynamicDir, 0755); err != nil {
		return err
	}

	routerName := sanitize(reg.AgentID)
	serviceName := sanitize(reg.AgentID) + "-svc"
	host := fmt.Sprintf("%s.%s.%s", reg.AgentID, reg.Project, tw.domain)

	port := 8080
	if len(reg.Ports) > 0 {
		port = reg.Ports[0]
	}

	config := fmt.Sprintf(`# Auto-generated route for agent %s
http:
  routers:
    %s:
      rule: "Host(\x60%s\x60)"
      service: %s
      entryPoints:
        - websecure
      tls: {}

  services:
    %s:
      loadBalancer:
        servers:
          - url: "http://%s:%d"
`, reg.AgentID, routerName, host, serviceName, serviceName, reg.VMIP, port)

	filename := filepath.Join(tw.dynamicDir, fmt.Sprintf("%s.yaml", routerName))
	return os.WriteFile(filename, []byte(config), 0644)
}

func (tw *TraefikWriter) RemoveRoute(agentID string) error {
	filename := filepath.Join(tw.dynamicDir, fmt.Sprintf("%s.yaml", sanitize(agentID)))
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (tw *TraefikWriter) SubdomainFor(agentID, project string) string {
	return fmt.Sprintf("%s.%s.%s", agentID, project, tw.domain)
}

func sanitize(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, ".", "-"), "/", "-")
}

func WriteStaticConfig(baseDir string, httpPort, httpsPort int) error {
	dynamicDir := filepath.Join(baseDir, "traefik", "dynamic")
	certsDir := filepath.Join(baseDir, "certs")

	config := fmt.Sprintf(`# Traefik static configuration for agentvm
entryPoints:
  web:
    address: ":%d"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":%d"

providers:
  file:
    directory: "%s"
    watch: true

tls:
  stores:
    default:
      defaultCertificate:
        certFile: "%s/agents.test.pem"
        keyFile: "%s/agents.test-key.pem"

api:
  dashboard: true
  insecure: true

log:
  level: INFO
`, httpPort, httpsPort, dynamicDir, certsDir, certsDir)

	traefikDir := filepath.Join(baseDir, "traefik")
	if err := os.MkdirAll(traefikDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(traefikDir, "traefik.yaml"), []byte(config), 0644)
}
