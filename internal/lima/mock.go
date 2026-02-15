package lima

import (
	"context"
	"fmt"
)

// MockClient implements the Client interface for testing.
type MockClient struct {
	Instances map[string]*Instance
	ShellFn   func(ctx context.Context, opts ShellOptions) (string, error)
	CreateErr error
	CloneErr  error
	StartErr  error
	StopErr   error
	DeleteErr error
	CopyErr   error
}

func NewMockClient() *MockClient {
	return &MockClient{
		Instances: make(map[string]*Instance),
	}
}

func (m *MockClient) Create(ctx context.Context, opts CreateOptions) error {
	if m.CreateErr != nil {
		return m.CreateErr
	}
	m.Instances[opts.Name] = &Instance{
		Name:   opts.Name,
		Status: StatusStopped,
		Arch:   "aarch64",
		CPUs:   opts.CPUs,
	}
	if opts.Start {
		m.Instances[opts.Name].Status = StatusRunning
	}
	return nil
}

func (m *MockClient) Clone(ctx context.Context, opts CloneOptions) error {
	if m.CloneErr != nil {
		return m.CloneErr
	}
	src, ok := m.Instances[opts.Source]
	if !ok {
		return fmt.Errorf("source %q not found", opts.Source)
	}
	status := StatusStopped
	if opts.Start {
		status = StatusRunning
	}
	m.Instances[opts.Target] = &Instance{
		Name:   opts.Target,
		Status: status,
		Arch:   src.Arch,
		CPUs:   src.CPUs,
		Memory: src.Memory,
	}
	return nil
}

func (m *MockClient) Start(ctx context.Context, name string) error {
	if m.StartErr != nil {
		return m.StartErr
	}
	inst, ok := m.Instances[name]
	if !ok {
		return fmt.Errorf("instance %q not found", name)
	}
	inst.Status = StatusRunning
	return nil
}

func (m *MockClient) Stop(ctx context.Context, name string) error {
	if m.StopErr != nil {
		return m.StopErr
	}
	inst, ok := m.Instances[name]
	if !ok {
		return fmt.Errorf("instance %q not found", name)
	}
	inst.Status = StatusStopped
	return nil
}

func (m *MockClient) Delete(ctx context.Context, name string, force bool) error {
	if m.DeleteErr != nil {
		return m.DeleteErr
	}
	delete(m.Instances, name)
	return nil
}

func (m *MockClient) List(ctx context.Context) ([]Instance, error) {
	result := make([]Instance, 0, len(m.Instances))
	for _, inst := range m.Instances {
		result = append(result, *inst)
	}
	return result, nil
}

func (m *MockClient) Get(ctx context.Context, name string) (*Instance, error) {
	inst, ok := m.Instances[name]
	if !ok {
		return nil, fmt.Errorf("instance %q not found", name)
	}
	return inst, nil
}

func (m *MockClient) Shell(ctx context.Context, opts ShellOptions) (string, error) {
	if m.ShellFn != nil {
		return m.ShellFn(ctx, opts)
	}
	return "", nil
}

func (m *MockClient) Copy(ctx context.Context, opts CopyOptions) error {
	return m.CopyErr
}
