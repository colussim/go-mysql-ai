package main

import (
	"fmt"
	"os"
	"time"

	configPkg "github.com/colussim/go-mysql-ai/pkg/config"
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

	configPkg.InitLogger()

	spin := spinner.New(spinner.CharSets[35], 100*time.Millisecond)
	startTime := time.Now()

	err := tools.RunImport("config/config.json", spin)
	if err != nil {
		spin.Stop()
		fmt.Println()
		configPkg.Log.Fatalf("❌ Error Import Data : %v", err)

		os.Exit(1)
	}

	spin.Stop()
	duration := time.Since(startTime)
	fmt.Println()
	configPkg.Log.Infof("✅ Import completed in %s\n", formatDuration(duration))

}
