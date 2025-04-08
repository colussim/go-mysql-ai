package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	configPkg "github.com/colussim/go-mysql-ai/pkg/config"
	_ "github.com/go-sql-driver/mysql"
	"github.com/ollama/ollama/api"
)

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

func generateEmbedding(text, model string) []float64 {

	configPkg.InitLogger()
	// Get the Ollama host from the environment variable or use the default local host
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434" // Default Ollama local server URL
	}

	url, _ := url.Parse(ollamaHost)
	// Create a new Ollama client with the local host
	client := api.NewClient(url, http.DefaultClient)

	// Use the mxbai-embed-large:latest model to generate embeddings

	req := &api.EmbeddingRequest{
		Model:  model,
		Prompt: text,
	}
	subSpinner1 := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	subSpinner1.Prefix = "           Embedding generation... "
	subSpinner1.Start()
	resp, err := client.Embeddings(context.Background(), req)
	if err != nil {
		subSpinner1.Stop()
		fmt.Println()
		configPkg.Log.Fatalf("❌ Error generating embedding: %v", err)
	}
	subSpinner1.Stop()

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

func InsertData(db *sql.DB, pathology string, details configPkg.PathologyDetail, data OpenFDAResponse, model string) error {

	embeddingText := fmt.Sprintf("%s. Description: %s. Symptoms: %s. Treatments: %s.",
		pathology,
		details.Description,
		strings.Join(details.Symptoms, ", "),
		strings.Join(details.Treatments, ", "))

	pathologyEmbedding := generateEmbedding(embeddingText, model)

	// Convert slice to string
	pathologyEmbeddingString, err := float64SliceToString(pathologyEmbedding)
	if err != nil {
		return fmt.Errorf("❌ Error converting pathology embedding to string: %w", err)
	}

	subSpinner1 := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	subSpinner1.Prefix = "           INSERT INTO pathologies... "
	subSpinner1.Start()
	size := len(pathologyEmbeddingString)
	// Insert vector using STRING_TO_VECTOR
	_, err = db.Exec("INSERT INTO pathologies (name, embedding) VALUES (?, STRING_TO_VECTOR(?))", pathology, pathologyEmbeddingString)
	if err != nil {
		subSpinner1.Stop()
		return fmt.Errorf("❌ Error inserting into pathologies table: %w - size vector %d: ", err, size)
	}
	subSpinner1.Stop()

	subSpinner1.Prefix = "           INSERT INTO medicationv... "
	var pathologyID int
	err = db.QueryRow("SELECT id FROM pathologies WHERE name = ?", pathology).Scan(&pathologyID)
	if err != nil {
		return fmt.Errorf("❌ Error fetching pathology ID: %w", err)
	}
	subSpinner1.Start()

	for _, result := range data.Results {
		if len(result.OpenFDA.BrandName) == 0 {
			continue
		}

		medicament := result.OpenFDA.BrandName[0]

		// Information retrieval and concatenation
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
		text := fmt.Sprintf("For this pathology: %s,Medication: %s. Indications: %s. Purpose: %s. Dosage: %s. Warnings: %s. Package Label: %s",
			pathology,
			medicament,
			strings.Join(result.IndicationsAndUsage, ", "),
			strings.Join(result.Purpose, ", "),
			strings.Join(result.DosageAndAdministration, ", "),
			strings.Join(result.Warnings, ", "),
			packageLabel,
		)

		medEmbedding := generateEmbedding(text, model)
		// CConvert slice to string
		medEmbeddingString, err := float64SliceToString(medEmbedding)

		size := len(pathologyEmbeddingString)

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
			medEmbeddingString,
		)
		if err != nil {
			subSpinner1.Stop()
			return fmt.Errorf("❌ Error inserting medication data: %w - size vector %d: ", err, size)
		}
	}
	subSpinner1.Stop()
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
	return nil
}

func RunImport(configPath string, spin *spinner.Spinner) error {

	configPkg.InitLogger()

	spin.Suffix = " Load Config..."
	spin.Color("green", "bold")
	spin.Start()

	config, err := configPkg.LoadConfig(configPath)
	if err != nil {
		spin.Stop()
		fmt.Println()
		configPkg.Log.Fatalf("❌ Error reading config file: %v", err)
		return err
	}
	spin.Stop()

	fmt.Println()
	fmt.Println()
	configPkg.Log.Infof("✅ Config Loaded \n")
	configPkg.Log.Infof("✅ Model use for Embedding generation: %s\n", config.Models.Embedding.Name)
	fmt.Println()

	spin.Suffix = " Load Pathologies..."
	spin.Start()
	pathologies, err := configPkg.LoadPathologies(config.Pathologie.File)
	if err != nil {
		spin.Stop()
		fmt.Println()
		configPkg.Log.Fatalf("❌ Error parsing JSON contents of file pathologies: %v", err)
		return err
	}

	spin.Stop()
	configPkg.Log.Infof("✅ Pathologies Loaded \n")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/health", config.MySQL.User, config.MySQL.Password, config.MySQL.Server, config.MySQL.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println()
		configPkg.Log.Fatalf("❌ Error connecting to MySQL: %v", err)
		return err
	}
	defer db.Close()

	spin.Suffix = " Init Database..."
	spin.Start()

	// Initialize the database by clearing the tables
	err = initDatabase(db)
	if err != nil {
		spin.Stop()
		fmt.Println()
		configPkg.Log.Fatalf("❌ Error initializing database: %v", err)
		return err
	}
	spin.Stop()
	configPkg.Log.Infof("✅ Tables pathologies and medicationv have been cleared.")

	spin.Suffix = " Insert Drug and Pathologies in DB ...\n"
	spin.Start()

	for pathology := range pathologies.Pathologies {
		data, err := fetchMedications(pathology)
		if err != nil {
			spin.Stop()
			fmt.Println()
			configPkg.Log.Fatalf("❌ Error fetching medications for pathology: %s - %v", pathology, err)
			continue
		}
		details := pathologies.Pathologies[pathology]

		err = InsertData(db, pathology, details, data, config.Models.Generation.Name)
		if err != nil {
			spin.Stop()
			fmt.Println()
			configPkg.Log.Fatalf("❌ Error inserting data for pathology: %s - %v", pathology, err)
		}
	}
	spin.Stop()
	configPkg.Log.Infof("✅ Data inserted successfully.")

	return nil
}
