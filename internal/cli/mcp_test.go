package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestIsMCPServeInvocation(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{[]string{"mcp", "serve"}, true},
		{[]string{"mcp", "status"}, false},
		{[]string{"mcp"}, false},
		{nil, false},
	}
	for _, tt := range tests {
		if got := isMCPServeInvocation(tt.args); got != tt.want {
			t.Errorf("isMCPServeInvocation(%v) = %v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestUpdateCheckSkipsAnnotatedCommands(t *testing.T) {
	cmd := &cobra.Command{Use: "serve", Annotations: map[string]string{skipUpdateCheckAnnotation: "true"}}
	// No context is set: reaching the update logic would panic on a nil context.
	if err := ensureUpToDate(cmd); err != nil {
		t.Fatalf("ensureUpToDate = %v", err)
	}
}
