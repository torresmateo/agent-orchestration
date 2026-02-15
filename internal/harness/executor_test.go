package harness

import (
	"testing"
)

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		tool     string
		prompt   string
		wantNil  bool
		wantArgs []string
	}{
		{
			tool:     "claude-code",
			prompt:   "Fix the bug",
			wantArgs: []string{"claude", "--dangerously-skip-permissions", "-p", "Fix the bug"},
		},
		{
			tool:     "opencode",
			prompt:   "Add tests",
			wantArgs: []string{"opencode", "run", "--prompt", "Add tests"},
		},
		{
			tool:     "amp",
			prompt:   "Refactor",
			wantArgs: []string{"amp", "run", "Refactor"},
		},
		{
			tool:     "cline",
			prompt:   "Add feature",
			wantArgs: []string{"cline", "--task", "Add feature"},
		},
		{
			tool:    "unknown",
			prompt:  "Do stuff",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.tool, func(t *testing.T) {
			args := buildCommand(tt.tool, tt.prompt)
			if tt.wantNil {
				if args != nil {
					t.Errorf("expected nil, got %v", args)
				}
				return
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("expected %d args, got %d: %v", len(tt.wantArgs), len(args), args)
			}
			for i, want := range tt.wantArgs {
				if args[i] != want {
					t.Errorf("arg[%d]: expected %q, got %q", i, want, args[i])
				}
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	if truncate("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if truncate("hello world", 5) != "hello..." {
		t.Errorf("expected 'hello...', got '%s'", truncate("hello world", 5))
	}
}
