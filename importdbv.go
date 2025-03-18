package main

import (
	"database/sql"
	"log"
	"time"

	"github.com/colussim/go-mysql-ai/pkg/tools"

	"github.com/briandowns/spinner"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	// Charger la configuration depuis le fichier de config
	config, err := tools.RunImport("config/config.json")
	if err != nil {
		log.Fatalf("Erreur lors du chargement de la configuration : %v", err)
	}

	// Construire le DSN à partir de la configuration
	dsn := config.MySQL.User + ":" + config.MySQL.Password + "@tcp(" + config.MySQL.Server + ":" + config.MySQL.Port + ")/health"
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Erreur lors de la connexion à la base de données : %v", err)
	}
	defer db.Close()

	spin := spinner.New(spinner.CharSets[35], 100*time.Millisecond)
	spin.Suffix = " Import Data..."
	spin.Color("green", "bold")
	spin.Start()
	startTime := time.Now()

	// Importer les données depuis l'API OpenFDA
	err = tools.ImportData(db, config.Pathologie.File)
	if err != nil {
		log.Fatalf("Erreur lors de l'importation des données : %v", err)
	}

	spin.Stop()
	duration := time.Since(startTime) // Calculer la durée
	log.Printf("Importation terminée en %v.", duration)
	log.Println("Données insérées avec succès.")
}
