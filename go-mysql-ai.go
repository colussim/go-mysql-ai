package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

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
	Model struct {
		Name string `json:"name"`
	} `json:"model"`
	Chatbotport struct {
		Port int `json:"port"`
	} `json:"chatbotport"`
}

type PathologyDetail struct {
	Description string   `json:"description"`
	Symptoms    []string `json:"symptoms"`
	Treatments  []string `json:"treatments"`
}

type Pathology struct {
	Pathologies map[string]PathologyDetail `json:"pathologies"`
}

type Medication struct {
	DrugName     string    `json:"drug_name"`
	Indications  string    `json:"indications_and_usage"`
	Purpose      string    `json:"purpose"`
	Dosage       string    `json:"dosage_and_administration"`
	Warnings     string    `json:"warnings"`
	PackageLabel string    `json:"package_label"`
	Embedding    []float64 `json:"embedding"`
}

// Main html page: index.html
var tpl = template.Must(template.ParseFiles("chat.html"))

var logger *logrus.Logger
var db *sql.DB
var pathology *Pathology
var config *Config
var httpPort int

func markdownToHTML2(markdown string) template.HTML {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs
	p := parser.NewWithExtensions(extensions)
	html := md.ToHTML([]byte(markdown), p, nil)
	return template.HTML(string(html))
}

func initDB(config *Config) error {
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

func markdownToHTML(w io.Writer, markdown string) error {
	tmpl, err := template.New("markdown").Parse(strings.ReplaceAll(markdown, "\n", "<br>"))
	if err != nil {
		return err
	}

	return tmpl.Execute(w, nil)
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

	/*var htmlBuffer bytes.Buffer
	err = markdownToHTML(&htmlBuffer, responseMessage)
	if err != nil {
		log.Printf("Error converting response to HTML: %v", err)
		http.Error(w, "Error converting response to HTML", http.StatusInternalServerError)
		return
	}

	response := Response{
		Response: htmlBuffer.String(),
	}
	sendJSONResponse(w, response)*/

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

func generateResponse(input string) (string, error) {
	// Étape 1 : Recovering embedding for pathology
	idpath, err := getPathologyIDByName(input)
	if err != nil {
		return "", fmt.Errorf("❌ Error getting pathology embedding: %w", err)
	}

	// Step 2 : Recommending drugs using embedding
	medications, err := findSimilarMedications(idpath, 10)
	if err != nil {
		return "", fmt.Errorf("❌ Error recommending medications: %w", err)
	}

	// Step 3 : Building a prompt for Ollama
	prompt := buildPromptForOllama(input, medications)

	// Step 4 : Send to Ollama and get a reply
	response, err := sendToOllama(prompt)
	if err != nil {
		return "", fmt.Errorf("❌ Error sending request to Ollama: %w", err)
	}

	// Step 5 : Return the content of the answer

	return response.Content, nil

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

func findSimilarMedications(pathologyID int, limit int) ([]Medication, error) {

	query := `
	SELECT drug_name,purpose,warnings,dosage_and_administration,package_label_principal_display_panel,indications_and_usage,VECTOR_TO_STRING(embedding)
	FROM medicationv
	WHERE pathologie_id = ?
	LIMIT ?`

	rows, err := db.Query(query, pathologyID, limit)
	if err != nil {
		return nil, fmt.Errorf("❌ Error querying medications for pathology: %w", err)
	}
	defer rows.Close()

	var medications []Medication
	for rows.Next() {
		var med Medication
		var embeddingString string

		if err := rows.Scan(&med.DrugName, &med.Indications, &med.Purpose, &med.Dosage, &med.Warnings, &med.PackageLabel, &embeddingString); err != nil {
			return nil, fmt.Errorf("❌ Error scanning row: %w", err)
		}

		// Convertir l'embedding de string à slice de float64
		med.Embedding, err = stringToFloat64Slice(embeddingString)
		if err != nil {
			return nil, fmt.Errorf("❌ Error converting medication embedding to float64 slice: %w", err)
		}

		medications = append(medications, med)
	}
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
	prompt := fmt.Sprintf("For the pathology: '%s', the following medications are available:\n", pathology)
	for _, med := range medications {
		prompt += fmt.Sprintf(
			"- Medication Name: %s\n  Indications: %s\n  Purpose: %s\n  Dosage: %s\n  Package Label: %s\n",
			med.DrugName,
			med.Indications,
			med.Purpose,
			med.Dosage,
			med.Warnings,
			med.PackageLabel)
	}
	prompt += "Based on the above medications, please provide additional recommendations."
	return prompt
}

func sendToOllama(prompt string) (*OllamaResponse, error) {

	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434"
	}
	parsedURL, err := url.Parse(ollamaHost)
	if err != nil {
		return nil, fmt.Errorf("❌ Invalid Ollama host URL: %w", err)
	}

	// Create an Ollama client
	client := api.NewClient(parsedURL, http.DefaultClient)

	// System instructions for model
	systemInstructions := "You are an expert pharmacist. Always respond in English. Provide brief and structured answers."

	// Create a chat request
	request := api.ChatRequest{
		Model: "qwen2.5:0.5b",
		Messages: []api.Message{
			{Role: "system", Content: systemInstructions},
			{Role: "user", Content: prompt},
		},
		Stream: func(b bool) *bool { return &b }(true),
	}

	// Send chat request to Ollama
	var responseContent strings.Builder
	err = client.Chat(context.Background(), &request, func(resp api.ChatResponse) error {
		responseContent.WriteString(resp.Message.Content)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("❌ Error calling Ollama API: %w", err)
	}

	return &OllamaResponse{
		Content: responseContent.String(),
	}, nil
}

func init() {
	var err error

	config, err := LoadConfig(configPath)
	if err != nil {
		logger.Fatal("❌ Error eading config file:", err)
	}
	pathology, err = LoadPathologies(config.Pathologie.File)
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
	//httpPort := config.Chatbotport.Port
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
