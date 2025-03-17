package tools

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

type Configuration struct {
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

// OpenFDAResponse représente la réponse JSON de l'API OpenFDA
type OpenFDAResponse struct {
	Results []struct {
		EffectiveTime            string   `json:"effective_time"`
		Purpose                  []string `json:"purpose"`
		KeepOutOfReachOfChildren []string `json:"keep_out_of_reach_of_children"`
		WhenUsing                []string `json:"when_using"`
		Questions                []string `json:"questions"`
		PregnancyOrBreastFeeding []string `json:"pregnancy_or_breast_feeding"`
		StorageAndHandling       []string `json:"storage_and_handling"`
		IndicationsAndUsage      []string `json:"indications_and_usage"`
		SetID                    string   `json:"set_id"`
		AskDoctorOrPharmacist    []string `json:"ask_doctor_or_pharmacist"`
		ActiveIngredient         []string `json:"active_ingredient"`
		DosageAndAdministration  []string `json:"dosage_and_administration"`
		InactiveIngredient       []string `json:"inactive_ingredient"`
		Warnings                 []string `json:"warnings"`
		Version                  string   `json:"version"`
		PackageLabel             []string `json:"package_label_principal_display_panel"`
	} `json:"results"`
}

// PathologyList représente la liste des pathologies
type PathologyList struct {
	Pathologies []string `json:"pathologies"`
}

func LoadConfig(filename string) (Configuration, error) {
	var config Configuration

	data, err := os.ReadFile(filename)
	if err != nil {
		return config, fmt.Errorf("erreur lors de la lecture du fichier de configuration : %w", err)
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("erreur lors de la désérialisation du JSON : %w", err)
	}

	return config, nil
}

// ImportData importe les données depuis l'API OpenFDA dans la base de données
func ImportData(db *sql.DB, pathologyFile string) error {
	// Vider la table medicament avant d'importer de nouvelles données
	_, err := db.Exec("DELETE FROM medication")
	if err != nil {
		return fmt.Errorf("Erreur lors de la suppression des données existantes : %w", err)
	}

	data, err := os.ReadFile(pathologyFile)
	if err != nil {
		return fmt.Errorf("Erreur lors de la lecture du fichier : %w", err)
	}

	var pathologyList PathologyList
	if err := json.Unmarshal(data, &pathologyList); err != nil {
		return fmt.Errorf("Erreur lors de la désérialisation du JSON : %w", err)
	}

	for _, pathologie := range pathologyList.Pathologies {
		url := fmt.Sprintf("https://api.fda.gov/drug/label.json?search=indications_and_usage:%s&limit=10", pathologie)

		fmt.Println("URL:", url)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("Erreur lors de la requête à l'API pour %s : %w", pathologie, err)
		}
		defer resp.Body.Close()

		var response OpenFDAResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return fmt.Errorf("Erreur lors de la décodage de la réponse pour %s : %w", pathologie, err)
		}

		for _, item := range response.Results {
			_, err := db.Exec(`
                INSERT INTO medication (
                    effective_time, purpose, keep_out_of_reach_of_children, 
                    when_using, questions, pregnancy_or_breast_feeding, 
                    storage_and_handling, indications_and_usage, set_id, 
                    ask_doctor_or_pharmacist, active_ingredient, 
                    dosage_and_administration, inactive_ingredient, 
                    warnings, version, package_label
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				item.EffectiveTime, getFirst(item.Purpose), getFirst(item.KeepOutOfReachOfChildren),
				getFirst(item.WhenUsing), getFirst(item.Questions), getFirst(item.PregnancyOrBreastFeeding),
				getFirst(item.StorageAndHandling), getFirst(item.IndicationsAndUsage), item.SetID,
				getFirst(item.AskDoctorOrPharmacist), getFirst(item.ActiveIngredient),
				getFirst(item.DosageAndAdministration), getFirst(item.InactiveIngredient),
				getFirst(item.Warnings), item.Version, getFirst(item.PackageLabel),
			)
			if err != nil {
				log.Printf("Erreur lors de l'insertion pour la pathologie %s : %v", pathologie, err)
			}
		}
	}

	return nil
}

func getFirst(slice []string) string {
	if len(slice) > 0 {
		return slice[0]
	}
	return ""
}
