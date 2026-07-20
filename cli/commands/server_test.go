package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewServerCommand(t *testing.T) {
	cmd := NewServerCommand()
	assert.Equal(t, "server", cmd.Use)
	assert.Contains(t, cmd.Short, "Start the LagerKV server")
	output := captureOutput(func() {
		cmd.Run(cmd, []string{})
	})
	assert.Contains(t, output, "Starting LagerKV server")
	assert.Contains(t, output, "Server commands are not implemented")
	assert.Contains(t, output, "Use 'lagerkvd' binary")
}
