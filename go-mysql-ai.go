package main

import (
	"bytes"
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
	"github.com/ollama/ollama/api"
	"github.com/sirupsen/logrus"
)

type TemplateData struct {
	Messages string
}

const HTTP_PORT = 3001

type Response struct {
	Response string `json:"response"`
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
}

type PathologyDetail struct {
	Description string   `json:"description"`
	Symptoms    []string `json:"symptoms"`
	Treatments  []string `json:"treatments"`
}

type Pathology struct {
	Pathologies map[string]PathologyDetail `json:"pathologies"`
}

// Main html page: index.html
var tpl = template.Must(template.ParseFiles("chat.html"))

var logger *logrus.Logger
var db *sql.DB
var pathology *Pathology

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

func LoadConfig2(filename string) (*Pathology, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var config Pathology
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func extractPathology(input string) string {
	input = strings.ToLower(input)
	for _, pathology := range pathology.Pathologies {
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
		response := Response{Response: "I did not recognize any pathology in your message. The pathologies supported are:" + strings.Join(pathology.Pathologies, ", ")}
		sendJSONResponse(w, response)
		return
	}

	responseMessage, err := generateResponse(extractedPathology)
	if err != nil {
		log.Printf("Error generating response: %v", err)
		http.Error(w, "Error generating response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var htmlBuffer bytes.Buffer
	err = markdownToHTML(&htmlBuffer, responseMessage)
	if err != nil {
		log.Printf("Error converting response to HTML: %v", err)
		http.Error(w, "Error converting response to HTML", http.StatusInternalServerError)
		return
	}

	response := Response{
		Response: htmlBuffer.String(),
	}
	sendJSONResponse(w, response)

	log.Printf("Response sent to client for pathology '%s': %s", extractedPathology, responseMessage)
	/*response := Response{Response: responseMessage}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}

	log.Printf("Response sent to client: %s", responseMessage)
	//w.Header().Set("Content-Type", "application/json")
	//json.NewEncoder(w).Encode(Response{Response: responseMessage})*/
}

/*
	func generateResponse2(input string) (string, error) {
		// Récupérer l'embedding de la pathologie
		embedding, err := getPathologyEmbedding(input)
		if err != nil {
			return "", fmt.Errorf("error getting pathology embedding: %w", err)
		}

		// Recommend medications through Ollama
		medications, err := recommendMedications(embedding)
		if err != nil {
			return "", fmt.Errorf("error recommending medications: %w", err)
		}

		// Return a formatted response
		//return fmt.Sprintf("Recommended medications : %s", strings.Join(medications, ", ")), nil
		//return fmt.Sprintf("Recommended medications : %s", medications), nil
		return medications.Content, nil

}
*/
func generateResponse(input string) (string, error) {
	// Step 1: Retrieve the embedding for the pathology
	embedding, err := getPathologyEmbedding(input)
	if err != nil {
		return "", fmt.Errorf("❌ Error getting pathology embedding: %w", err)
	}

	// Step 2: Recommend medications using the embedding
	response, err := recommendMedications2(embedding)
	if err != nil {
		return "", fmt.Errorf("❌ Error recommending medications: %w", err)
	}

	// Step 3: Return the content of the response
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

func findSimilarMedications(pathologyEmbedding []float64, limit int) ([]string, error) {
	query := `
        SELECT drug_name
        FROM medicationv
        ORDER BY DOT_PRODUCT(embedding, STRING_TO_VECTOR(?)) DESC
        LIMIT ?
    `

	// Convert the pathology embedding to a string
	pathologyEmbeddingString, err := float64SliceToString(pathologyEmbedding)
	if err != nil {
		return nil, fmt.Errorf("❌ Error converting embedding to string: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("❌ Error converting embedding to string: %w", err)
	}

	rows, err := db.Query(query, pathologyEmbeddingString, limit)
	if err != nil {
		return nil, fmt.Errorf("❌ Error querying similar medications: %w", err)
	}
	defer rows.Close()

	medications := []string{}
	for rows.Next() {
		var medication string
		if err := rows.Scan(&medication); err != nil {
			return nil, fmt.Errorf("❌ Error scanning medication name: %w", err)
		}
		medications = append(medications, medication)
	}

	return medications, nil
}

func float64SliceToString(slice []float64) (string, error) {
	var builder strings.Builder
	builder.WriteString("[")
	for i, value := range slice {
		if i > 0 {
			builder.WriteString(",")
		}
		_, err := fmt.Fprintf(&builder, "%f", value)
		if err != nil {
			return "", fmt.Errorf("❌ Error formatting float64 to string: %w", err)
		}
	}
	builder.WriteString("]")
	return builder.String(), nil
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

func recommendMedications(embedding []float64) (string, error) {
	// Step 1: Find similar medications
	medications, err := findSimilarMedications(embedding, 10) // Limit to top 5 medications
	if err != nil {
		return "", fmt.Errorf("❌ Error finding similar medications: %w", err)
	}

	// Step 2: Construct the prompt for Ollama
	medicationList := strings.Join(medications, ", ")
	prompt := fmt.Sprintf("Based on the embedding, the most similar medications are: %s. Provide a detailed recommendation for these medications.", medicationList)

	// Step 3: Send the prompt to Ollama
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434" // Default Ollama local server URL
	}

	parsedURL, err := url.Parse(ollamaHost)
	if err != nil {
		return "", fmt.Errorf("❌ Invalid Ollama host URL: %w", err)
	}

	client := api.NewClient(parsedURL, http.DefaultClient)

	request := api.ChatRequest{
		Model: "qwen2.5:0.5b",
		Messages: []api.Message{
			{Role: "system", Content: "You are an expert pharmacist. Always respond in English."},
			{Role: "user", Content: prompt},
		},
	}

	var responseContent strings.Builder
	err = client.Chat(context.Background(), &request, func(resp api.ChatResponse) error {
		responseContent.WriteString(resp.Message.Content)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("❌ Error calling Ollama API: %w", err)
	}

	return responseContent.String(), nil
}
func recommendMedications2(embedding []float64) (*OllamaResponse, error) {

	// Get the Ollama host from the environment variable or use the default local host
	ollamaHost := os.Getenv("OLLAMA_HOST")
	if ollamaHost == "" {
		ollamaHost = "http://localhost:11434" // Default Ollama local server URL
	}

	// Parse the Ollama host URL
	parsedURL, err := url.Parse(ollamaHost)
	if err != nil {
		return nil, fmt.Errorf("❌ Invalid Ollama host URL: %w", err)
	}

	// Create a new Ollama client
	client := api.NewClient(parsedURL, http.DefaultClient)

	// Convert the embedding to JSON
	embeddingJSON, err := json.Marshal(embedding)
	if err != nil {
		return nil, fmt.Errorf("❌ Error converting embedding to JSON: %w", err)
	}

	systemInstructions := "You are an expert pharmacist. Always respond in English. Provide brief and structured answers."
	question := fmt.Sprintf("Here is the embedding of a pathology: %s, Based on this embedding, recommend appropriate medications. List them as bullet points.", string(embeddingJSON))

	// Create the chat request payload
	request := api.ChatRequest{
		Model: "qwen2.5:0.5b",
		Messages: []api.Message{
			{Role: "system", Content: systemInstructions},
			{Role: "user", Content: question},
		},
		Stream: func(b bool) *bool { return &b }(true),
		//Format: "html",
	}

	// Send the chat request to Ollama
	var responseContent strings.Builder // Declare the variable to store the response content
	err = client.Chat(context.Background(), &request, func(resp api.ChatResponse) error {
		fmt.Print(resp.Message.Content)
		// Capture the response content
		responseContent.WriteString(resp.Message.Content)
		//responseContent = resp.Message.Content

		//fmt.Print(responseContent)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("❌ Error calling Ollama API: %w", err)
	}

	// Parse the response content into a list of medications
	/*medications := []string{}
	if responseContent != "" {
		// Assuming the response content is a comma-separated list of medications
		medications = strings.Split(responseContent, ",")
	}*/
	return &OllamaResponse{
		Content: responseContent.String(),
	}, nil

	//return responseContent, nil
}

func init() {
	var err error
	pathology, err = LoadConfig2("config/pathologies.json")
	if err != nil {
		logger.Fatal("❌ Error loading config pathologies:", err)
	}
}

func main() {

	var port string
	portFlag := flag.String("port", "", fmt.Sprintf("Port on which the server will listen (default is %d)", HTTP_PORT))

	flag.Parse()
	if *portFlag != "" {
		port = *portFlag
	} else {
		port = strconv.Itoa(HTTP_PORT)
	}

	logger = logrus.New()

	config, err := LoadConfig("config/config.json")
	if err != nil {
		logger.Fatalf("❌ Error loading configuration: %v", err)
	}

	// Initialiser la connexion à la base de données
	if err := initDB(config); err != nil {
		logger.Fatalf("❌ Error initializing database: %v", err)
	}
	defer db.Close()

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
