package cli

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

var tailCmd = &cobra.Command{
	Use:   "tail",
	Short: "Tail logs in real-time",
	Run:   runTail,
}

func runTail(cmd *cobra.Command, args []string) {
	wsURL := "ws" + apiURL[4:] + "/api/logs/tail"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WebSocket connect failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			var entry map[string]interface{}
			if err := conn.ReadJSON(&entry); err != nil {
				return
			}

			ts := time.Unix(int64(entry["timestamp"].(float64)), 0)
			fmt.Printf("[%s] %s [%s] %s\n",
				ts.Format("15:04:05"),
				entry["level"],
				entry["source"],
				entry["message"],
			)
		}
	}()

	fmt.Println("Tailing logs... (Ctrl+C to stop)")

	select {
	case <-done:
	case <-interrupt:
		conn.Close()
	}
}
