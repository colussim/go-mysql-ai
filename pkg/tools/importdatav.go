package importdatav

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/ollama/ollama-go"
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

func LoadConfig(filename string) (*Config, error) {
	file, err := ioutil.ReadFile(filename)
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
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var pathologies Pathologies
	if err := json.Unmarshal(file, &pathologies); err != nil {
		return nil, err
	}
	return &pathologies, nil
}

func generateEmbedding(text string) []float32 {
	client := ollama.NewClient("")
	resp, _ := client.GenerateEmbedding("mistral", text)
	return resp.Vector
}

func fetchMedications(pathology string) OpenFDAResponse {
	eencodedPathology := url.QueryEscape(pathology)
	url := fmt.Sprintf("https://api.fda.gov/drug/label.json?search=indications_and_usage:%s&limit=50", encodedPathology)
	resp, _ := http.Get(url)
	body, _ := ioutil.ReadAll(resp.Body)
	var data OpenFDAResponse
	json.Unmarshal(body, &data)
	return data
}

func InsertData(db *sql.DB, pathology string, data OpenFDAResponse) {
	pathologyEmbedding := generateEmbedding(pathology)

	_, err := db.Exec("DELETE FROM pathologies")
	if err != nil {
		return fmt.Errorf("❌ Error deleting existing data from pathologies table : %w", err)
	}

	_, err := db.Exec("INSERT INTO pathologies (nom, embedding) VALUES (?, ?)", pathology, pathologyEmbedding)
	if err != nil {
		fmt.Println("❌ Error insert pathologies table:", err)
		return
	}

	_, err := db.Exec("DELETE FROM medicationv")
	if err != nil {
		return fmt.Errorf("❌ Error deleting existing data from medicationv table : %w", err)
	}

	var pathologyID int
	db.QueryRow("SELECT id FROM pathologies WHERE nom = ?", pathology).Scan(&pathologyID)

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
		_, err := db.Exec("INSERT INTO medicationv (nom, description, pathologie_id, embedding) VALUES (?, ?, ?, ?)", medicament, text, pathologyID, medEmbedding)
		if err != nil {
			fmt.Println("❌ Error insert medicationv table:", err)
		}
	}
}

func RunImport(configPath string) {
	config, err := LoadConfig(configPath)
	if err != nil {
		fmt.Println("❌ Error reading config file:", err)
		return
	}
	pathologies, err := LoadPathologies(config.Pathologie.File)
	if err != nil {
		fmt.Println("❌ Error parsing JSON contents of file pathologies:", err)
		return
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/ai_meds", config.MySQL.User, config.MySQL.Password, config.MySQL.Server, config.MySQL.Port)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Println("❌ Error connexion MySQL:", err)
		return
	}
	defer db.Close()

	for _, pathology := range pathologies.Pathologies {
		data := fetchMedications(pathology)
		InsertData(db, pathology, data)
	}

	//fmt.Println("Importation terminée !")
}
