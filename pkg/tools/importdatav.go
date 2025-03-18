package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	//"github.com/ollama/ollama-go"
	"github.com/ollama/ollama/api"
)

type Config struct {
	MySQL struct {
		User     string `json:"user"`
		Password string `json:"password"`
		Server   string `json:"server"`
		Port     string `json:"port"`
		TypeAuth string `json:"type_auth"`
	} `json:"mysql"`
	Pathologie struct {
		File string `json:"file"`
	} `json:"pathologie"`
}

type Pathologies struct {
	Pathologies []string `json:"pathologies"`
}

type OpenFDAResponse struct {
	Results []struct {
		OpenFDA struct {
			BrandName        []string `json:"brand_name"`
			ActiveIngredient []string `json:"active_ingredient"`
		} `json:"openfda"`
		IndicationsAndUsage []string `json:"indications_and_usage"`
		Purpose             []string `json:"purpose"`
		DosageAndAdmin      []string `json:"dosage_and_administration"`
	} `json:"results"`
}

var (
	FALSE = false
	TRUE  = true
)

func LoadConfig(filename string) (*Config, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func LoadPathologies(filename string) (*Pathologies, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var pathologies Pathologies
	if err := json.Unmarshal(file, &pathologies); err != nil {
		return nil, err
	}
	return &pathologies, nil
}

func generateEmbedding(text string) []float64 {

	// Get the Ollama host from the environment variable or use the default local host
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434" // Default Ollama local server URL
	}

	url, _ := url.Parse(ollamaHost)
	// Create a new Ollama client with the local host
	client := api.NewClient(url, http.DefaultClient)

	// Use the qwen2.5:0.5b model to generate embeddings

	req := &api.EmbeddingRequest{
		Model:  "qwen2.5:0.5b",
		Prompt: text,
	}
	resp, err := client.Embeddings(context.Background(), req)
	if err != nil {
		log.Fatalln("❌ Error generating embedding:", err)
	}

	return resp.Embedding

}

func fetchMedications(pathology string) (OpenFDAResponse, error) {
	encodedPathology := url.QueryEscape(pathology)
	url := fmt.Sprintf("https://api.fda.gov/drug/label.json?search=indications_and_usage:%s&limit=50", encodedPathology)
	resp, err := http.Get(url)
	if err != nil {
		return OpenFDAResponse{}, fmt.Errorf("❌ Error fetching medications: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return OpenFDAResponse{}, fmt.Errorf("❌ Error reading response body: %w", err)
	}

	var data OpenFDAResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return OpenFDAResponse{}, fmt.Errorf("❌ Error unmarshalling response: %w", err)
	}
	return data, nil
}

func InsertData(db *sql.DB, pathology string, data OpenFDAResponse) error {
	// Generate embedding for the pathology
	pathologyEmbedding := generateEmbedding(pathology)

	// Insert the pathology into the database
	_, err := db.Exec("INSERT INTO pathologies (nom, embedding) VALUES (?, ?)", pathology, pathologyEmbedding)
	if err != nil {
		return fmt.Errorf("❌ Error inserting into pathologies table: %w", err)
	}

	// Fetch the pathology ID
	var pathologyID int
	err = db.QueryRow("SELECT id FROM pathologies WHERE nom = ?", pathology).Scan(&pathologyID)
	if err != nil {
		return fmt.Errorf("❌ Error fetching pathology ID: %w", err)
	}

	// Insert medications
	for _, result := range data.Results {
		if len(result.OpenFDA.BrandName) == 0 {
			continue
		}
		medicament := result.OpenFDA.BrandName[0]
		text := fmt.Sprintf("%s. Purpose: %s. Active ingredients: %s. Dosage: %s",
			strings.Join(result.IndicationsAndUsage, " "),
			strings.Join(result.Purpose, " "),
			strings.Join(result.OpenFDA.ActiveIngredient, " "),
			strings.Join(result.DosageAndAdmin, " "))
		medEmbedding := generateEmbedding(text)

		// Insert the medication into the database
		_, err = db.Exec("INSERT INTO medicationv (nom, description, pathologie_id, embedding) VALUES (?, ?, ?, ?)", medicament, text, pathologyID, medEmbedding)
		if err != nil {
			fmt.Println("❌ Error inserting into medicationv table:", err)
		}
	}
	return nil
}

func initDatabase(db *sql.DB) error {
	// Delete all data from the pathologies table
	_, err := db.Exec("DELETE FROM pathologies")
	if err != nil {
		return fmt.Errorf("❌ Error deleting data from pathologies table: %w", err)
	}

	// Delete all data from the medicationv table
	_, err = db.Exec("DELETE FROM medicationv")
	if err != nil {
		return fmt.Errorf("❌ Error deleting data from medicationv table: %w", err)
	}

	fmt.Println("✅ Tables pathologies and medicationv have been cleared.")
	return nil
}

func RunImport(configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		fmt.Println("❌ Error reading config file:", err)
		return err
	}
	pathologies, err := LoadPathologies(config.Pathologie.File)
	if err != nil {
		fmt.Println("❌ Error parsing JSON contents of file pathologies:", err)
		return err
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/health", config.MySQL.User, config.MySQL.Password, config.MySQL.Server, config.MySQL.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("❌ Error connecting to MySQL:", err)
		return err
	}
	defer db.Close()

	// Initialize the database by clearing the tables
	err = initDatabase(db)
	if err != nil {
		fmt.Println("❌ Error initializing database:", err)
		return err
	}

	for _, pathology := range pathologies.Pathologies {
		data, err := fetchMedications(pathology)
		if err != nil {
			fmt.Println("❌ Error fetching medications for pathology:", pathology, err)
			continue
		}
		err = InsertData(db, pathology, data)
		if err != nil {
			fmt.Println("❌ Error inserting data for pathology:", pathology, err)
		}
	}

	return nil
}
