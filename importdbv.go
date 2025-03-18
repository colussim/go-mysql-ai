package main

import (
	"log"
	"os"
	"time"

	"github.com/colussim/go-mysql-ai/pkg/tools"

	"github.com/briandowns/spinner"
	_ "github.com/go-sql-driver/mysql"
)

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
	log.Printf("✅ Import completed in %2f.", duration)
	log.Println("✅ Data inserted successfully.")
}
