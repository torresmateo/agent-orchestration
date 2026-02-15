package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type Reporter struct {
	baseURL string
	client  *http.Client
}

func NewReporter(baseURL string) *Reporter {
	return &Reporter{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (r *Reporter) Report(agentID, state, message string) {
	payload := map[string]string{
		"agentID": agentID,
		"state":   state,
		"message": message,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Failed to marshal status report: %v", err)
		return
	}

	url := fmt.Sprintf("%s/status", r.baseURL)
	resp, err := r.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		log.Printf("Failed to report status to host: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Status report returned HTTP %d", resp.StatusCode)
	}
}

func (r *Reporter) Register(agentID, vmName, vmIP, project, tool string, ports []int) error {
	payload := map[string]interface{}{
		"agentID": agentID,
		"vmName":  vmName,
		"vmIP":    vmIP,
		"project": project,
		"tool":    tool,
		"ports":   ports,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshaling register request: %w", err)
	}

	url := fmt.Sprintf("%s/register", r.baseURL)
	resp, err := r.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("register request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("register returned HTTP %d", resp.StatusCode)
	}
	return nil
}
