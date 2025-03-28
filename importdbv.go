package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/colussim/go-mysql-ai/pkg/tools"

	"github.com/briandowns/spinner"
	_ "github.com/go-sql-driver/mysql"
)

func formatDuration(duration time.Duration) string {

	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

func main() {

	spin := spinner.New(spinner.CharSets[35], 100*time.Millisecond)
	spin.Suffix = " Import Data..."
	spin.Color("green", "bold")
	spin.Start()
	startTime := time.Now()

	err := tools.RunImport("config/config.json")
	if err != nil {
		log.Fatalf("❌ Error Import Data : %v", err)
		spin.Stop()
		os.Exit(1)
	}

	spin.Stop()
	duration := time.Since(startTime)

	log.Printf("✅ Import completed in %s\n", formatDuration(duration))
	log.Println("✅ Data inserted successfully.")
}
