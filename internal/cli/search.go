package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

var (
	limit   int
	offset  int
	from    string
	to      string
	levels  string
	sources string

	searchCmd = &cobra.Command{
		Use:   "search [query]",
		Short: "Search logs",
		Args:  cobra.MaximumNArgs(1),
		Run:   runSearch,
	}
)

func init() {
	searchCmd.Flags().IntVar(&limit, "limit", 50, "Result limit")
	searchCmd.Flags().IntVar(&offset, "offset", 0, "Result offset")
	searchCmd.Flags().StringVar(&from, "from", "", "Start time (unix timestamp or RFC3339)")
	searchCmd.Flags().StringVar(&to, "to", "", "End time (unix timestamp or RFC3339)")
	searchCmd.Flags().StringVar(&levels, "levels", "", "Filter levels (comma-separated)")
	searchCmd.Flags().StringVar(&sources, "sources", "", "Filter sources (comma-separated)")
}

func runSearch(cmd *cobra.Command, args []string) {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	params.Set("limit", strconv.Itoa(limit))
	if offset > 0 {
		params.Set("offset", strconv.Itoa(offset))
	}
	if from != "" {
		if ts, err := parseTime(from); err == nil {
			params.Set("from", strconv.FormatInt(ts, 10))
		}
	}
	if to != "" {
		if ts, err := parseTime(to); err == nil {
			params.Set("to", strconv.FormatInt(ts, 10))
		}
	}
	if levels != "" {
		params.Set("levels", levels)
	}
	if sources != "" {
		params.Set("sources", sources)
	}

	resp, err := http.Get(apiURL + "/api/logs/search?" + params.Encode())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if results, ok := result["results"].([]interface{}); ok {
		for _, r := range results {
			entry := r.(map[string]interface{})
			ts := time.Unix(int64(entry["timestamp"].(float64)), 0)
			fmt.Printf("[%s] %s [%s] #%v: %s\n",
				ts.Format("2006-01-02 15:04:05"),
				entry["level"],
				entry["source"],
				entry["id"],
				entry["message"],
			)
		}
		fmt.Printf("\nTotal: %v\n", result["count"])
	}
}

func parseTime(s string) (int64, error) {
	if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
		return ts, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), nil
	}
	return 0, fmt.Errorf("invalid time format")
}
