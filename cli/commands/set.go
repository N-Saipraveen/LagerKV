package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/N-Saipraveen/LagerKV/cli/output"
	servertypes "github.com/N-Saipraveen/LagerKV/types"
	"github.com/spf13/cobra"
)

// NewSetCommand creates a new set command
func NewSetCommand() *cobra.Command {
	var ttlStr string

	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a key-value pair",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]
			addr := os.Getenv("LOGKV_ADDR")
			if addr == "" {
				addr = "http://localhost:8080"
			}

			client := &http.Client{Timeout: 10 * time.Second}
			payload := servertypes.SetRequest{
				Key:   key,
				Value: value,
				TTL:   ttlStr,
			}
			body, _ := json.Marshal(payload)

			req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/v1/kv", addr), bytes.NewReader(body))
			if err != nil {
				output.Error(fmt.Sprintf("Failed to create request: %v", err))
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				output.Error(fmt.Sprintf("Failed to connect to server at %s\n %v", addr, err))
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusNoContent {
				output.Error(fmt.Sprintf("Server error: %s", resp.Status))
				return
			}

			if ttlStr != "" {
				output.Success(fmt.Sprintf("Set %s = %s (TTL: %s)", key, value, ttlStr))
			} else {
				output.Success(fmt.Sprintf("Set %s = %s", key, value))
			}
		},
	}

	cmd.Flags().StringVar(&ttlStr, "ttl", "", "time to live (e.g. 60s, 5m, 1h or integer seconds)")
	return cmd
}
