package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

func NewClient(port int) *Client {
	return &Client{
		BaseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Dispatch(req DispatchRequest) (*DispatchResponse, error) {
	var resp DispatchResponse
	if err := c.post("/dispatch", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Status() (*PoolStatus, error) {
	var resp PoolStatus
	if err := c.get("/status", &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Kill(agentID string) error {
	return c.post(fmt.Sprintf("/agents/%s/kill", agentID), nil, nil)
}

func (c *Client) Logs(agentID string, follow bool, execution bool) (io.ReadCloser, error) {
	url := fmt.Sprintf("%s/agents/%s/logs?follow=%v&execution=%v", c.BaseURL, agentID, follow, execution)
	resp, err := c.HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	return resp.Body, nil
}

func (c *Client) PoolReplenish() error {
	return c.post("/pool/replenish", nil, nil)
}

func (c *Client) PoolDrain() error {
	return c.post("/pool/drain", nil, nil)
}

func (c *Client) PoolResize(warmSize int) error {
	return c.post("/pool/resize", map[string]int{"warmSize": warmSize}, nil)
}

func (c *Client) get(path string, result interface{}) error {
	resp, err := c.HTTPClient.Get(c.BaseURL + path)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}

	if result != nil {
		return json.Unmarshal(body, result)
	}
	return nil
}

func (c *Client) post(path string, payload, result interface{}) error {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		body = bytes.NewReader(data)
	}

	resp, err := c.HTTPClient.Post(c.BaseURL+path, "application/json", body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, respBody)
	}

	if result != nil {
		return json.Unmarshal(respBody, result)
	}
	return nil
}
