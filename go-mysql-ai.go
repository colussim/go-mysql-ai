package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	configPkg "github.com/colussim/go-mysql-ai/pkg/config"
	_ "github.com/go-sql-driver/mysql"
	md "github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/ollama/ollama/api"
	"github.com/sirupsen/logrus"
)

type TemplateData struct {
	Messages string
}

const configPath = "config/config.json"

type Response struct {
	Response string `json:"response"`
}

type Response1 struct {
	Response template.HTML `json:"response"`
}

type OllamaResponse struct {
	Content string `json:"content"`
}

type Medication struct {
	DrugName        string    `json:"drug_name"`
	Indications     string    `json:"indications_and_usage"`
	Purpose         string    `json:"purpose"`
	Dosage          string    `json:"dosage_and_administration"`
	Warnings        string    `json:"warnings"`
	PackageLabel    string    `json:"package_label"`
	Embedding       []float64 `json:"embedding"`
	SimilarityScore float64
}

// Main html page: index.html
var tpl = template.Must(template.ParseFiles("dist/templates/chat.html"))

var logger *logrus.Logger
var db *sql.DB
var pathology *configPkg.Pathology
var config *configPkg.Config
var httpPort int

func CosineSimilarity(vec1, vec2 []float64) float64 {
	var dotProduct, normA, normB float64

	for i := range vec1 {
		dotProduct += vec1[i] * vec2[i]
		normA += vec1[i] * vec1[i]
		normB += vec2[i] * vec2[i]
	}

	// Avoid division by zero
	if normA == 0 || normB == 0 {
		return 0.0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

func markdownToHTML2(markdown string) template.HTML {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	html := md.ToHTML([]byte(markdown), p, nil)
	return template.HTML(string(html))
}

func initDB(config *configPkg.Config) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/health", config.MySQL.User, config.MySQL.Password, config.MySQL.Server, config.MySQL.Port)
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("❌ Database connection error: %w", err)
	}

	// Vérifiez la connexion
	if err := db.Ping(); err != nil {
		return fmt.Errorf("❌ Error verifying database connection: %w", err)
	}

	return nil
}

func extractPathology(input string) string {
	input = strings.ToLower(input)

	for pathology := range pathology.Pathologies {
		if strings.Contains(input, strings.ToLower(pathology)) {
			return pathology
		}
	}

	return ""
}

func sendJSONResponse(w http.ResponseWriter, response Response) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("❌ Error encoding response: %v", err)
		http.Error(w, "❌ Error encoding response", http.StatusInternalServerError)
	}
}

func sendJSONResponse2(w http.ResponseWriter, response Response1) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("❌ Error encoding response: %v", err)
		http.Error(w, "❌ Error encoding response", http.StatusInternalServerError)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	tpl.Execute(w, nil)
}

func chatHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	message := r.Form.Get("message")

	extractedPathology := extractPathology(message)
	if extractedPathology == "" {
		pathologiesList := make([]string, 0, len(pathology.Pathologies))
		for name := range pathology.Pathologies {
			pathologiesList = append(pathologiesList, name)
		}
		response := Response{Response: "I did not recognize any pathology in your message. The pathologies supported are:" + strings.Join(pathologiesList, ", ")}
		sendJSONResponse(w, response)
		return
	}

	responseMessage, err := generateResponse(extractedPathology)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		http.Error(w, "Error generating response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	htmlResponse := markdownToHTML2(responseMessage)

	response := Response1{
		Response: htmlResponse,
	}
	sendJSONResponse2(w, response)

	log.Printf("Response sent to client for pathology '%s': %s", extractedPathology, responseMessage)

}

func getPathologyIDByName(pathologyName string) (int, error) {
	var id int
	err := db.QueryRow("SELECT id FROM pathologies WHERE name = ?", strings.ToLower(pathologyName)).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("❌ Error retrieving pathology ID: %w", err)
	}
	return id, nil
}

func getPathologyIDAndEmbeddingByName(pathologyName string) (int, string, error) {
	var id int
	var embedding string

	// SQL query to retrieve ID and embedding
	err := db.QueryRow("SELECT id, VECTOR_TO_STRING(embedding) FROM pathologies WHERE name = ?", strings.ToLower(pathologyName)).Scan(&id, &embedding)
	if err != nil {
		return 0, "", fmt.Errorf("❌ Error retrieving pathology ID and embedding: %w", err)
	}

	// Returns the ID and embedding as a string
	return id, embedding, nil
}

func generateResponse(pathologyName string) (string, error) {
	// Step 1: Retrieve the pathology ID
	pathologyID, embeddingP, err := getPathologyIDAndEmbeddingByName(pathologyName)
	if err != nil {
		return "", fmt.Errorf("❌ Error getting pathology ID: %w", err)
	}

	// Step 2: Retrieve the embeddings for medications
	embeddings, err := findSimilarMedications(pathologyName, pathologyID, 3, embeddingP)
	if err != nil {
		return "", fmt.Errorf("❌ Error retrieving medication embeddings: %w", err)
	}

	// Step 3: Send the embeddings to Ollama and get a reply
	response, err := sendToOllama(embeddings, pathologyName)
	if err != nil {
		return "", fmt.Errorf("❌ Error sending request to Ollama: %w", err)
	}

	// Step 4: Return the content of the answer
	return response, nil
}

func getPathologyEmbedding(pathology string) ([]float64, error) {
	var embeddingString string

	err := db.QueryRow("SELECT VECTOR_TO_STRING(embedding) FROM pathologies WHERE name = ?", strings.ToLower(pathology)).Scan(&embeddingString)
	if err != nil {
		return nil, fmt.Errorf("❌ Error retrieving embedding vector record: %w", err)
	}

	// Convert embedding string to float64 slice
	embedding, err := stringToFloat64Slice(embeddingString)
	if err != nil {
		return nil, err
	}

	return embedding, nil
}

func findSimilarMedications(pathologyName string, pathologyID int, limit int, embeddingP string) ([]Medication, error) {

	pathologyEmbedding, err := stringToFloat64Slice(embeddingP)
	if err != nil {
		return nil, fmt.Errorf("❌ Error converting pathology embedding to float64 slice: %w", err)
	}

	query := `
    SELECT 
		drug_name,
		purpose,
		warnings,
		dosage_and_administration,
		package_label_principal_display_panel,
		indications_and_usage,
		VECTOR_TO_STRING(embedding)
	FROM medicationv
	WHERE pathologie_id = ?`

	//rows, err := db.Query(query, pathologyID, limit)
	rows, err := db.Query(query, pathologyID)
	if err != nil {
		return nil, fmt.Errorf("❌ Error querying medications for pathology: %w", err)
	}
	defer rows.Close()

	var medications []Medication
	for rows.Next() {
		var med Medication
		var embeddingString string

		if err := rows.Scan(&med.DrugName, &med.Purpose, &med.Warnings, &med.Dosage, &med.PackageLabel, &med.Indications, &embeddingString); err != nil {
			return nil, fmt.Errorf("❌ Error scanning row: %w", err)
		}

		// Convert embedding from string to float64 slice
		med.Embedding, err = stringToFloat64Slice(embeddingString)
		if err != nil {
			return nil, fmt.Errorf("❌ Error converting medication embedding to float64 slice: %w", err)
		}

		// Add similarity score for debugging if needed
		med.SimilarityScore = CosineSimilarity(pathologyEmbedding, med.Embedding)

		medications = append(medications, med)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("❌ Error iterating over rows: %w", err)
	}

	// Sort drugs by similarity score
	sort.Slice(medications, func(i, j int) bool {
		return medications[i].SimilarityScore > medications[j].SimilarityScore
	})

	// View best medicines
	/*	fmt.Println("Top Medications for Pathology: ", pathologyName)
		for i := 0; i < len(medications) && i < limit; i++ {
			med := medications[i]
			fmt.Printf("Rank: %d\n", i+1)
			fmt.Printf("Medication Name: %s\n", med.DrugName)
			fmt.Printf("Similarity Score: %.4f\n", med.SimilarityScore)
			fmt.Printf("Purpose: %s\n", med.Purpose)
			fmt.Printf("Indications: %s\n", med.Indications)
			fmt.Printf("Dosage: %s\n", med.Dosage)
			fmt.Println("-----")
		}*/

	return medications, nil
}

func stringToFloat64Slice(embeddingString string) ([]float64, error) {
	// Remove the brackets if present
	embeddingString = strings.Trim(embeddingString, "[]")

	// Split the string by commas
	parts := strings.Split(embeddingString, ",")

	// Create a slice of float64
	embedding := make([]float64, len(parts))

	for i, part := range parts {
		var value float64
		_, err := fmt.Sscanf(part, "%f", &value)
		if err != nil {
			return nil, fmt.Errorf("❌ Error converting string to float64: %w", err)
		}
		embedding[i] = value

	}

	return embedding, nil
}

func buildPromptForOllama(pathology string, medications []Medication) string {
	prompt := fmt.Sprintf("For this pathology: %s, the following medications are available:\n", pathology)
	for _, med := range medications {
		prompt += fmt.Sprintf(
			"- Medication Name: %s\n  Indications: %s\n  Purpose: %s\n  Dosage: %s\n  Warnings: %s\n  Package Label: %s\n",
			med.DrugName,
			med.Indications,
			med.Purpose,
			med.Dosage,
			med.Warnings,
			med.PackageLabel,
		)
	}
	prompt += config.Model.Prompt
	//prompt += "Please analyze the medications listed below and recommend at least two for this pathology, displaying dosage and indications."
	return prompt
}

func decodeEmbeddingToText(embedding []float64) string {
	// Example: Use a simple mapping for demonstration purposes
	// In a real-world scenario, you would use a model or more complex logic
	if len(embedding) == 0 {
		return "No embedding provided."
	}

	// Example logic: Check the first value of the embedding
	if embedding[0] > 0.5 {
		return "This embedding corresponds to a positive recommendation."
	} else if embedding[0] < -0.5 {
		return "This embedding corresponds to a negative recommendation."
	} else {
		return "This embedding corresponds to a neutral recommendation."
	}
}

func sendToOllama(medicaments []Medication, pathology string) (string, error) {
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	parsedURL, err := url.Parse(ollamaHost)
	if err != nil {
		return "", fmt.Errorf("❌ Invalid Ollama host URL: %w", err)
	}

	client := api.NewClient(parsedURL, http.DefaultClient)

	prompt := buildPromptForOllama(pathology, medicaments)

	chatRequest := api.ChatRequest{
		Model: config.Model.Name,
		Messages: []api.Message{
			{Role: "system", Content: "You are an expert pharmacist. Always respond in English."},
			{Role: "user", Content: prompt},
		},
		Stream: func(b bool) *bool { return &b }(true),
	}

	var responseContent strings.Builder
	err = client.Chat(context.Background(), &chatRequest, func(resp api.ChatResponse) error {
		responseContent.WriteString(resp.Message.Content)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("❌ Error calling Ollama API: %w", err)
	}

	return responseContent.String(), nil
}

func init() {
	var err error

	config, err = configPkg.LoadConfig(configPath)
	if err != nil {
		logger.Fatal("❌ Error eading config file:", err)
	}
	pathology, err = configPkg.LoadPathologies(config.Pathologie.File)
	if err != nil {
		logger.Fatal("❌ Error loading config pathologies:", err)
	}

	// Initialize database connection
	if err := initDB(config); err != nil {
		logger.Fatalf("❌ Error initializing database: %v", err)
	}
	httpPort = config.Chatbotport.Port
}

func main() {

	var port string
	portFlag := flag.String("port", "", fmt.Sprintf("Port on which the server will listen (default is %d)", httpPort))

	flag.Parse()
	if *portFlag != "" {
		port = *portFlag
	} else {
		port = strconv.Itoa(httpPort)
	}

	logger = logrus.New()

	fs := http.FileServer(http.Dir("dist"))

	mux := http.NewServeMux()
	mux.Handle("/dist/", http.StripPrefix("/dist/", fs))
	mux.HandleFunc("/", indexHandler)
	mux.HandleFunc("/chat", chatHandler)

	go func() {
		err := http.ListenAndServe(":"+port, mux)
		if err != nil {
			if opErr, ok := err.(*net.OpError); ok && opErr.Op == "listen" {
				logger.Fatalf("❌ The port %s is already in use. Please use another port", port)
			} else {
				logger.Fatalf("❌ Unexpected HTTP service startup error : %v", err)
			}
		}

	}()
	logger.Infof("✅ HTTP service started on port %s\n", port)
	select {}

}
