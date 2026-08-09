package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show log statistics",
	Run:   runStats,
}

func runStats(cmd *cobra.Command, args []string) {
	resp, err := http.Get(apiURL + "/api/logs/stats")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var stats map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&stats)

	fmt.Println("Log Statistics:")
	fmt.Printf("  Total logs:     %v\n", stats["total_logs"])
	fmt.Printf("  Unique sources: %v\n", stats["unique_sources"])
	fmt.Printf("  Unique levels:  %v\n", stats["unique_levels"])

	if oldest, ok := stats["oldest"].(float64); ok && oldest > 0 {
		fmt.Printf("  Oldest:         %s\n", time.Unix(int64(oldest), 0).Format(time.RFC3339))
	}
	if newest, ok := stats["newest"].(float64); ok && newest > 0 {
		fmt.Printf("  Newest:         %s\n", time.Unix(int64(newest), 0).Format(time.RFC3339))
	}
}
