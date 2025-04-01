package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
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

type PathologyDetail struct {
	Description string   `json:"description"`
	Symptoms    []string `json:"symptoms"`
	Treatments  []string `json:"treatments"`
}

type Pathology struct {
	Pathologies map[string]PathologyDetail `json:"pathologies"`
}

type OpenFDAResponse struct {
	Results []struct {
		OpenFDA struct {
			BrandName        []string `json:"brand_name"`
			ActiveIngredient []string `json:"active_ingredient"`
		} `json:"openfda"`
		InactiveIngredient                []string `json:"inactive_ingredient"`
		IndicationsAndUsage               []string `json:"indications_and_usage"`
		Purpose                           []string `json:"purpose"`
		KeepOutOfReachOfChildren          []string `json:"keep_out_of_reach_of_children"`
		Warnings                          []string `json:"warnings"`
		SPLProductDataElements            []string `json:"spl_product_data_elements"`
		DosageAndAdministration           []string `json:"dosage_and_administration"`
		PregnancyOrBreastFeeding          []string `json:"pregnancy_or_breast_feeding"`
		PackageLabelPRincipalDisplayPanel []string `json:"package_label_principal_display_panel"`
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

func LoadPathologies(filename string) (*Pathology, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var pathologies Pathology
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
	url := fmt.Sprintf("https://api.fda.gov/drug/label.json?search=indications_and_usage:%s+AND+_exists_:openfda.brand_name&limit=50", encodedPathology)

	resp, err := http.Get(url)
	if err != nil {
		return OpenFDAResponse{}, fmt.Errorf("❌ Error fetching medications: %w", err)
	}
	defer resp.Body.Close()

	// Check if the response status code is 200 (OK)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body) // Read the body for debugging purposes
		return OpenFDAResponse{}, fmt.Errorf("❌ API returned status %d: %s", resp.StatusCode, string(body))
	}

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

func float64SliceToString(values []float64) (string, error) {
	for _, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return "", fmt.Errorf("invalid value detected in vector: %v", v)
		}
	}

	var str []string
	for _, v := range values {
		str = append(str, fmt.Sprintf("%f", v))
	}
	return "[" + strings.Join(str, ",") + "]", nil
}

func InsertData(db *sql.DB, pathology string, details PathologyDetail, data OpenFDAResponse) error {

	embeddingText := fmt.Sprintf("%s. Description: %s. Symptoms: %s. Treatments: %s.",
		pathology,
		details.Description,
		strings.Join(details.Symptoms, ", "),
		strings.Join(details.Treatments, ", "))

	pathologyEmbedding := generateEmbedding(embeddingText)

	// Convert slice to string
	pathologyEmbeddingString, err := float64SliceToString(pathologyEmbedding)
	if err != nil {
		return fmt.Errorf("❌ Error converting pathology embedding to string: %w", err)
	}

	// Insert vector using STRING_TO_VECTOR
	_, err = db.Exec("INSERT INTO pathologies (name, embedding) VALUES (?, STRING_TO_VECTOR(?))", pathology, pathologyEmbeddingString)
	if err != nil {
		return fmt.Errorf("❌ Error inserting into pathologies table: %w", err)
	}

	var pathologyID int
	err = db.QueryRow("SELECT id FROM pathologies WHERE name = ?", pathology).Scan(&pathologyID)
	if err != nil {
		return fmt.Errorf("❌ Error fetching pathology ID: %w", err)
	}

	for _, result := range data.Results {
		if len(result.OpenFDA.BrandName) == 0 {
			continue
		}

		medicament := result.OpenFDA.BrandName[0]

		// Récupération et concaténation des informations
		indications := strings.Join(result.IndicationsAndUsage, ". ")
		purpose := strings.Join(result.Purpose, ". ")

		dosage := strings.Join(result.DosageAndAdministration, ". ")
		inactiveIngredients := strings.Join(result.InactiveIngredient, ", ")
		keepOutOfReach := strings.Join(result.KeepOutOfReachOfChildren, ". ")
		warnings := strings.Join(result.Warnings, ". ")
		splProductData := strings.Join(result.SPLProductDataElements, ". ")
		pregnancy := strings.Join(result.PregnancyOrBreastFeeding, ". ")
		packageLabel := strings.Join(result.PackageLabelPRincipalDisplayPanel, ". ")

		//text := fmt.Sprintf("Medication: %s. Indications: %s. Purpose: %s. Active Ingredients: %s. Dosage: %s. Warnings: %s. Package Label: %s",
		text := fmt.Sprintf("Medication: %s. Indications: %s. Purpose: %s. Dosage: %s. Warnings: %s. Package Label: %s",
			medicament,
			strings.Join(result.IndicationsAndUsage, ", "),
			strings.Join(result.Purpose, ", "),
			strings.Join(result.DosageAndAdministration, ", "),
			strings.Join(result.Warnings, ", "),
			packageLabel,
		)

		medEmbedding := generateEmbedding(text)
		// Convertir le slice en string
		medEmbeddingString, err := float64SliceToString(medEmbedding)
		if err != nil {
			return fmt.Errorf("❌ Error converting medication embedding to string: %w", err)
		}

		// Insertion dans la base de données
		_, err = db.Exec(`INSERT INTO medicationv (
            pathologie_id,
			drug_name,
            inactive_ingredient,
            purpose,
            keep_out_of_reach_of_children,
            warnings,
            spl_product_data_elements,
            dosage_and_administration,
            pregnancy_or_breast_feeding,
            package_label_principal_display_panel,
            indications_and_usage,
            embedding
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, STRING_TO_VECTOR(?))`,
			pathologyID,
			medicament,
			inactiveIngredients,
			purpose,
			keepOutOfReach,
			warnings,
			splProductData,
			dosage,
			pregnancy,
			packageLabel,
			indications,
			medEmbeddingString, // Vecteur d'embedding converti
		)
		if err != nil {
			return fmt.Errorf("❌ Error inserting medication data: %w", err)
		}
	}
	return nil
}

func initDatabase(db *sql.DB) error {

	// Delete all data from the medicationv table
	_, err := db.Exec("DELETE FROM medicationv")
	if err != nil {
		return fmt.Errorf("❌ Error deleting data from medicationv table: %w", err)
	}

	// Delete all data from the pathologies table
	_, err = db.Exec("DELETE FROM pathologies")
	if err != nil {
		return fmt.Errorf("❌ Error deleting data from pathologies table: %w", err)
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

	for pathology := range pathologies.Pathologies {
		data, err := fetchMedications(pathology)
		if err != nil {
			fmt.Println("❌ Error fetching medications for pathology:", pathology, err)
			continue
		}
		details := pathologies.Pathologies[pathology]

		err = InsertData(db, pathology, details, data)
		if err != nil {
			fmt.Println("❌ Error inserting data for pathology:", pathology, err)
		}
	}

	return nil
}
