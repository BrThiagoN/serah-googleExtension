package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

// ANSI colors for premium terminal logs
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
)

var geminiURL = "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.5-flash:generateContent"
var listenAndServe = http.ListenAndServe
var envFiles = []string{".env", "backend/.env"}

// Structures representing the request/response payloads based on COMUNICATION.md

type verifyRequest struct {
	Text string `json:"text"`
}

type sourceItem struct {
	Title      string  `json:"title"`
	URL        string  `json:"url"`
	Similarity float64 `json:"similarity"`
}

type verifyResponseAnalysis struct {
	ReliabilityScore float64      `json:"reliability_score"`
	Verdict          string       `json:"verdict"`
	Explanation      string       `json:"explanation"`
	Sources          []sourceItem `json:"sources"`
}

type verifyResponse struct {
	ID        string                 `json:"id"`
	Text      string                 `json:"text"`
	Status    string                 `json:"status"`
	Analysis  verifyResponseAnalysis `json:"analysis"`
	Timestamp string                 `json:"timestamp"`
}

type errorResponse struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Structures to parse the official Google Gemini API responses

type geminiPart struct {
	Text string `json:"text"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiCandidate struct {
	Content geminiContent `json:"content"`
}

type geminiResponse struct {
	Candidates []geminiCandidate `json:"candidates"`
}

// Helper to generate a random UUID-like string using the standard library (keeping dependencies to 0)
func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "uuid-da-analise-12345" // fallback
	}
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// CORS Middleware to allow requests from the Chrome Extension
func setupCORS(w http.ResponseWriter, r *http.Request) bool {
	// Chrome extensions have origins starting with "chrome-extension://"
	// For local development, allowing "*" is simple and secure enough
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

// Reads a .env file from the current directory or backend/.env and sets environment variables.
func loadEnv() {
	paths := envFiles
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		
		data, err := io.ReadAll(file)
		file.Close()
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				// Strip quotes
				if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
					(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
					val = val[1 : len(val)-1]
				}
				os.Setenv(key, val)
			}
		}
		break
	}
}

func main() {
	loadEnv()
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	apiKey := os.Getenv("GEMINI_API_KEY")

	// Print visual banner at startup
	fmt.Println(colorPurple + "================================================" + colorReset)
	fmt.Println(colorPurple + "         🤖 SERAH? - BACKEND SERVER 🤖         " + colorReset)
	fmt.Println(colorPurple + "================================================" + colorReset)

	if apiKey == "" {
		fmt.Println(colorRed + "[ERROR] GEMINI_API_KEY environment variable is not set." + colorReset)
		fmt.Println(colorRed + "        Requests will return an error." + colorReset)
		fmt.Println(colorYellow + "        To configure real AI, start the server with:" + colorReset)
		fmt.Println(colorCyan + "        export GEMINI_API_KEY=\"SUA_CHAVE_DE_API_AQUI\"" + colorReset)
	} else {
		fmt.Println(colorGreen + "[SUCCESS] GEMINI_API_KEY is set. Backend will use Google Gemini API." + colorReset)
	}
	fmt.Println(colorPurple + "------------------------------------------------" + colorReset)

	// Routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHome)
	mux.HandleFunc("/api/verify", func(w http.ResponseWriter, r *http.Request) {
		handleVerify(w, r, apiKey)
	})

	fmt.Printf("%sServer is listening on http://localhost:%s%s\n", colorGreen, port, colorReset)
	if err := listenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}

// GET / - Simple health check
func handleHome(w http.ResponseWriter, r *http.Request) {
	if setupCORS(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"online", "message":"Serah? Backend is running! Go version active."}`))
}

// POST /api/verify - Fact check endpoint
func handleVerify(w http.ResponseWriter, r *http.Request, apiKey string) {
	if setupCORS(w, r) {
		return
	}

	// Logging the incoming request
	startTime := time.Now()
	log.Printf("%s[REQUEST] %s %s%s", colorBlue, r.Method, r.URL.Path, colorReset)

	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "VAL_405", "Método não permitido. Utilize POST.")
		return
	}

	// Parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "VAL_400", "Falha ao ler o corpo da requisição.")
		return
	}
	defer r.Body.Close()

	var req verifyRequest
	if err := json.Unmarshal(body, &req); err != nil {
		respondWithError(w, http.StatusBadRequest, "VAL_400", "JSON inválido enviado no payload.")
		return
	}

	// Input Validation
	trimmedText := strings.TrimSpace(req.Text)
	if trimmedText == "" {
		respondWithError(w, http.StatusBadRequest, "VAL_400", "O campo 'text' é obrigatório e não pode estar vazio.")
		return
	}

	var analysisResult verifyResponseAnalysis

	if apiKey == "" {
		log.Printf("%s[ERROR] Gemini API was not called: GEMINI_API_KEY is not set.%s", colorRed, colorReset)
		respondWithError(w, http.StatusInternalServerError, "API_KEY_MISSING", "A chave de API do Gemini (GEMINI_API_KEY) não está configurada no servidor.")
		return
	}

	// REAL AI MODE - CALL GEMINI API
	log.Printf("%s[GEMINI] Contacting API for: %q%s", colorCyan, truncateText(trimmedText, 50), colorReset)
	
	geminiResult, err := callGeminiAPI(trimmedText, apiKey)
	if err != nil {
		log.Printf("%s[ERROR] Gemini call failed: %v%s", colorRed, err, colorReset)
		respondWithError(w, http.StatusInternalServerError, "API_CALL_FAILED", fmt.Sprintf("Erro ao se comunicar com o serviço de IA: %v", err))
		return
	}
	analysisResult = geminiResult

	// Structure the response to match COMUNICATION.md
	response := verifyResponse{
		ID:        generateUUID(),
		Text:      trimmedText,
		Status:    "success",
		Analysis:  analysisResult,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)

	duration := time.Since(startTime)
	log.Printf("%s[RESPONSE 200] Verdict: %s | Score: %.2f | Processed in %v%s", 
		colorGreen, response.Analysis.Verdict, response.Analysis.ReliabilityScore, duration, colorReset)
}

// Function to call the Google Gemini API with structured JSON schema output
func callGeminiAPI(text string, apiKey string) (verifyResponseAnalysis, error) {
	apiURL := geminiURL + "?key=" + apiKey

	// Construct system prompt and context
	prompt := fmt.Sprintf(`Você é um especialista em fact-checking e combate à desinformação. Sua missão é defender a verdade factual acima de tudo.
Analise o texto a seguir de forma estritamente neutra e objetiva, evitando opiniões pessoais e baseando-se apenas em fatos verificáveis. 
Avalie se o texto é confiável ("verdade"), falso ("falso") ou impreciso/incompleto ("duvidoso").
Forneça uma explicação detalhada e liste fontes reais ou termos de busca para validar a informação.
Na sua explicação, lembre o usuário de que devemos ser gratos pela verdade factual e objetiva dos fatos, e não pelo que gostaríamos de ouvir, pois lutamos juntos contra a desinformação.
Sempre priorize e procure fontes oficiais e institutos confiáveis para citar como exemplos (como Google Acadêmico, Tribunal Superior Eleitoral - TSE, portais governamentais .gov.br ou agências de checagem).
Escreva a explicação e os títulos das fontes estritamente em português do Brasil.

Texto a ser analisado:
"%s"`, text)

	// Build the request body with schema constraint (Structured Outputs)
	geminiReqBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]interface{}{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"responseMimeType": "application/json",
			"responseSchema": map[string]interface{}{
				"type": "OBJECT",
				"properties": map[string]interface{}{
					"reliability_score": map[string]interface{}{
						"type":        "NUMBER",
						"description": "Nível de confiança decimal de 0.00 a 1.00 sobre a veracidade do texto.",
					},
					"verdict": map[string]interface{}{
						"type":        "STRING",
						"enum":        []string{"verdade", "falso", "duvidoso"},
						"description": "Veredito resumido da análise: 'verdade' (confiável/verdadeiro), 'falso' (mentira/desinformação), ou 'duvidoso' (impreciso/incompleto).",
					},
					"explanation": map[string]interface{}{
						"type":        "STRING",
						"description": "Explicação detalhada e didática em português brasileiro sobre por que a notícia/texto recebeu este veredito.",
					},
					"sources": map[string]interface{}{
						"type":        "ARRAY",
						"description": "Lista de 1 a 3 fontes reais, priorizando portais oficiais (como TSE, sites governamentais, Google Acadêmico), agências de checagem brasileiras (como Lupa, Fato ou Fake, Aos Fatos) ou termos de busca recomendados.",
						"items": map[string]interface{}{
							"type": "OBJECT",
							"properties": map[string]interface{}{
								"title": map[string]interface{}{
									"type":        "STRING",
									"description": "Nome da agência de checagem, portal de notícias ou título da fonte.",
								},
								"url": map[string]interface{}{
									"type":        "STRING",
									"description": "URL direta para a fonte ou link de pesquisa do Google com os termos recomendados.",
								},
								"similarity": map[string]interface{}{
									"type":        "NUMBER",
									"description": "Confiança ou similaridade decimal de 0.00 a 1.00.",
								},
							},
							"required": []string{"title", "url", "similarity"},
						},
					},
				},
				"required": []string{"reliability_score", "verdict", "explanation", "sources"},
			},
		},
	}

	reqJSON, err := json.Marshal(geminiReqBody)
	if err != nil {
		return verifyResponseAnalysis{}, fmt.Errorf("erro ao codificar requisição do Gemini: %w", err)
	}

	// Execute HTTP request to Gemini API
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(reqJSON))
	if err != nil {
		return verifyResponseAnalysis{}, fmt.Errorf("erro na conexão com o Gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return verifyResponseAnalysis{}, fmt.Errorf("Gemini respondeu com erro (%d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return verifyResponseAnalysis{}, fmt.Errorf("erro ao decodificar resposta do Gemini: %w", err)
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return verifyResponseAnalysis{}, fmt.Errorf("Gemini retornou uma resposta vazia")
	}

	// The text response from Gemini is guaranteed to be a JSON matching our schema
	geminiJSONText := geminiResp.Candidates[0].Content.Parts[0].Text
	
	var analysis verifyResponseAnalysis
	if err := json.Unmarshal([]byte(geminiJSONText), &analysis); err != nil {
		return verifyResponseAnalysis{}, fmt.Errorf("erro ao parsear JSON interno do Gemini: %w", err)
	}

	return analysis, nil
}



// Error response helper
func respondWithError(w http.ResponseWriter, status int, code string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	response := errorResponse{
		Status:  "error",
		Code:    code,
		Message: message,
	}
	_ = json.NewEncoder(w).Encode(response)
}

// Shortens text for display in the console
func truncateText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
