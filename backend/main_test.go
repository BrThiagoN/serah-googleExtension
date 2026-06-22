package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// custom error reader to force read errors for testing
type errorReader struct{}

func (errorReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("forced read error")
}

func TestGenerateUUID(t *testing.T) {
	// Success path
	uuid := generateUUID()
	if len(uuid) != 36 {
		t.Errorf("expected uuid length of 36, got %d", len(uuid))
	}
	parts := strings.Split(uuid, "-")
	if len(parts) != 5 {
		t.Errorf("expected 5 segments in uuid, got %d", len(parts))
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"a", 0, "..."},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%d", tt.input, tt.max), func(t *testing.T) {
			got := truncateText(tt.input, tt.max)
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestRespondWithError(t *testing.T) {
	rec := httptest.NewRecorder()
	respondWithError(rec, http.StatusBadRequest, "TEST_ERR", "test message")

	res := rec.Result()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, res.StatusCode)
	}

	if contentType := res.Header.Get("Content-Type"); contentType != "application/json" {
		t.Errorf("expected content type application/json, got %q", contentType)
	}

	var errResp errorResponse
	err := json.NewDecoder(res.Body).Decode(&errResp)
	if err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if errResp.Status != "error" {
		t.Errorf("expected status 'error', got %q", errResp.Status)
	}
	if errResp.Code != "TEST_ERR" {
		t.Errorf("expected code 'TEST_ERR', got %q", errResp.Code)
	}
	if errResp.Message != "test message" {
		t.Errorf("expected message 'test message', got %q", errResp.Message)
	}
}

func TestSetupCORS(t *testing.T) {
	// Test OPTIONS request
	reqOptions := httptest.NewRequest(http.MethodOptions, "/api/verify", nil)
	recOptions := httptest.NewRecorder()
	isOptions := setupCORS(recOptions, reqOptions)

	if !isOptions {
		t.Error("expected setupCORS to return true for OPTIONS request")
	}

	resOptions := recOptions.Result()
	if resOptions.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resOptions.StatusCode)
	}
	if origin := resOptions.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin to be '*', got %q", origin)
	}
	if methods := resOptions.Header.Get("Access-Control-Allow-Methods"); methods != "POST, GET, OPTIONS" {
		t.Errorf("expected Access-Control-Allow-Methods, got %q", methods)
	}

	// Test non-OPTIONS request (POST)
	reqPost := httptest.NewRequest(http.MethodPost, "/api/verify", nil)
	recPost := httptest.NewRecorder()
	isOptionsPost := setupCORS(recPost, reqPost)

	if isOptionsPost {
		t.Error("expected setupCORS to return false for POST request")
	}

	resPost := recPost.Result()
	if origin := resPost.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin to be '*', got %q", origin)
	}
}

func TestHandleHome(t *testing.T) {
	// Test normal GET request
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handleHome(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, res.StatusCode)
	}

	var body map[string]string
	err := json.NewDecoder(res.Body).Decode(&body)
	if err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body["status"] != "online" {
		t.Errorf("expected status 'online', got %q", body["status"])
	}

	// Test OPTIONS preflight request
	reqOptions := httptest.NewRequest(http.MethodOptions, "/", nil)
	recOptions := httptest.NewRecorder()
	handleHome(recOptions, reqOptions)

	if recOptions.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d for OPTIONS on home", http.StatusOK, recOptions.Code)
	}
}

func TestCallGeminiAPI(t *testing.T) {
	// Backup original geminiURL and restore after test
	oldGeminiURL := geminiURL
	defer func() { geminiURL = oldGeminiURL }()

	// Case 1: Success path
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got %s", r.Method)
		}
		if key := r.URL.Query().Get("key"); key != "test-key" {
			t.Errorf("expected api key 'test-key', got %q", key)
		}

		response := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{
								Text: `{"reliability_score":0.95,"verdict":"verdade","explanation":"Texto verídico comprovado por fontes.","sources":[{"title":"G1","url":"https://g1.com","similarity":0.99}]}`,
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	server := httptest.NewServer(successHandler)
	defer server.Close()

	geminiURL = server.URL

	analysis, err := callGeminiAPI("Qualquer texto", "test-key")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if analysis.ReliabilityScore != 0.95 {
		t.Errorf("expected score 0.95, got %f", analysis.ReliabilityScore)
	}
	if analysis.Verdict != "verdade" {
		t.Errorf("expected verdict 'verdade', got %q", analysis.Verdict)
	}
	if len(analysis.Sources) != 1 || analysis.Sources[0].Title != "G1" {
		t.Errorf("expected 1 source with title 'G1', got %v", analysis.Sources)
	}

	// Case 2: HTTP error response
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request format"))
	})
	errorServer := httptest.NewServer(errorHandler)
	defer errorServer.Close()

	geminiURL = errorServer.URL
	_, err = callGeminiAPI("Texto", "test-key")
	if err == nil {
		t.Error("expected error for bad status code from Gemini, got nil")
	}
	if !strings.Contains(err.Error(), "Gemini respondeu com erro") {
		t.Errorf("expected error to mention Gemini response error, got: %v", err)
	}

	// Case 3: Empty candidates response
	emptyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := geminiResponse{
			Candidates: []geminiCandidate{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	emptyServer := httptest.NewServer(emptyHandler)
	defer emptyServer.Close()

	geminiURL = emptyServer.URL
	_, err = callGeminiAPI("Texto", "test-key")
	if err == nil {
		t.Error("expected error for empty candidates from Gemini, got nil")
	}
	if !strings.Contains(err.Error(), "Gemini retornou uma resposta vazia") {
		t.Errorf("expected empty response error, got: %v", err)
	}

	// Case 4: Malformed internal JSON in Gemini response
	malformedJSONHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{
								Text: `{invalid-json`,
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	malformedJSONServer := httptest.NewServer(malformedJSONHandler)
	defer malformedJSONServer.Close()

	geminiURL = malformedJSONServer.URL
	_, err = callGeminiAPI("Texto", "test-key")
	if err == nil {
		t.Error("expected error parsing malformed internal JSON, got nil")
	}
	if !strings.Contains(err.Error(), "erro ao parsear JSON interno") {
		t.Errorf("expected internal json parsing error, got: %v", err)
	}

	// Case 5: Network connection failure
	geminiURL = "http://invalid-domain-name-that-does-not-exist.local"
	_, err = callGeminiAPI("Texto", "test-key")
	if err == nil {
		t.Error("expected network error, got nil")
	}
}

func TestHandleVerify(t *testing.T) {
	// Backup original geminiURL
	oldGeminiURL := geminiURL
	defer func() { geminiURL = oldGeminiURL }()

	// Set up success response mock server
	successHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := geminiResponse{
			Candidates: []geminiCandidate{
				{
					Content: geminiContent{
						Parts: []geminiPart{
							{
								Text: `{"reliability_score":0.80,"verdict":"duvidoso","explanation":"Texto duvidoso.","sources":[]}`,
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	server := httptest.NewServer(successHandler)
	defer server.Close()
	geminiURL = server.URL

	// 1. HTTP method not allowed (GET)
	reqGet := httptest.NewRequest(http.MethodGet, "/api/verify", nil)
	recGet := httptest.NewRecorder()
	handleVerify(recGet, reqGet, "some-key")
	if recGet.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405 Method Not Allowed, got %d", recGet.Code)
	}

	// 2. Request body read error
	reqReadErr := httptest.NewRequest(http.MethodPost, "/api/verify", errorReader{})
	recReadErr := httptest.NewRecorder()
	handleVerify(recReadErr, reqReadErr, "some-key")
	if recReadErr.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for read error, got %d", recReadErr.Code)
	}
	if !strings.Contains(recReadErr.Body.String(), "VAL_400") {
		t.Errorf("expected VAL_400 code in response, got: %s", recReadErr.Body.String())
	}

	// 3. Request body with invalid JSON
	reqInvalidJSON := httptest.NewRequest(http.MethodPost, "/api/verify", strings.NewReader("{invalid-json"))
	recInvalidJSON := httptest.NewRecorder()
	handleVerify(recInvalidJSON, reqInvalidJSON, "some-key")
	if recInvalidJSON.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid JSON, got %d", recInvalidJSON.Code)
	}

	// 4. Empty/whitespace text field
	reqEmptyText := httptest.NewRequest(http.MethodPost, "/api/verify", strings.NewReader(`{"text":"   "}`))
	recEmptyText := httptest.NewRecorder()
	handleVerify(recEmptyText, reqEmptyText, "some-key")
	if recEmptyText.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty text, got %d", recEmptyText.Code)
	}
	if !strings.Contains(recEmptyText.Body.String(), "O campo 'text' é obrigatório") {
		t.Errorf("expected text validation message, got: %s", recEmptyText.Body.String())
	}

	// 5. API key is missing
	reqMissingKey := httptest.NewRequest(http.MethodPost, "/api/verify", strings.NewReader(`{"text":"Fato real"}`))
	recMissingKey := httptest.NewRecorder()
	handleVerify(recMissingKey, reqMissingKey, "")
	if recMissingKey.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for missing key, got %d", recMissingKey.Code)
	}
	if !strings.Contains(recMissingKey.Body.String(), "API_KEY_MISSING") {
		t.Errorf("expected API_KEY_MISSING code, got: %s", recMissingKey.Body.String())
	}

	// 6. Successful verification with real mock AI endpoint
	reqSuccess := httptest.NewRequest(http.MethodPost, "/api/verify", strings.NewReader(`{"text":"Fato real"}`))
	recSuccess := httptest.NewRecorder()
	handleVerify(recSuccess, reqSuccess, "some-key")
	if recSuccess.Code != http.StatusOK {
		t.Errorf("expected 200 OK for successful verification, got %d. Body: %s", recSuccess.Code, recSuccess.Body.String())
	}

	var successResp verifyResponse
	if err := json.Unmarshal(recSuccess.Body.Bytes(), &successResp); err != nil {
		t.Fatalf("failed to parse success response: %v", err)
	}
	if successResp.Status != "success" {
		t.Errorf("expected status 'success', got %q", successResp.Status)
	}
	if successResp.Analysis.Verdict != "duvidoso" {
		t.Errorf("expected verdict 'duvidoso', got %q", successResp.Analysis.Verdict)
	}

	// 7. Gemini call failure (Internal Server Error due to API error)
	errorHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	})
	errorServer := httptest.NewServer(errorHandler)
	defer errorServer.Close()
	geminiURL = errorServer.URL

	reqFail := httptest.NewRequest(http.MethodPost, "/api/verify", strings.NewReader(`{"text":"Fato real"}`))
	recFail := httptest.NewRecorder()
	handleVerify(recFail, reqFail, "some-key")
	if recFail.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 Internal Server Error when API fails, got %d", recFail.Code)
	}
	if !strings.Contains(recFail.Body.String(), "API_CALL_FAILED") {
		t.Errorf("expected API_CALL_FAILED code in body, got: %s", recFail.Body.String())
	}
}

func TestMainFunc(t *testing.T) {
	oldListenAndServe := listenAndServe
	defer func() { listenAndServe = oldListenAndServe }()

	listenAndServe = func(addr string, handler http.Handler) error {
		return nil
	}

	// Disable loading real env files during test to allow testing branches in main()
	oldEnvFiles := envFiles
	envFiles = []string{"non_existent_file_to_skip_loading_env.env"}
	defer func() { envFiles = oldEnvFiles }()

	// Save original env vars to restore later
	oldPort := os.Getenv("PORT")
	oldAPIKey := os.Getenv("GEMINI_API_KEY")
	defer func() {
		os.Setenv("PORT", oldPort)
		os.Setenv("GEMINI_API_KEY", oldAPIKey)
	}()

	// Case 1: PORT and API Key set
	os.Setenv("PORT", "3099")
	os.Setenv("GEMINI_API_KEY", "dummy-key")
	main()

	// Case 2: PORT and API Key empty
	os.Setenv("PORT", "")
	os.Setenv("GEMINI_API_KEY", "")
	main()
}

func TestLoadEnv(t *testing.T) {
	// Backup original env vars
	oldPort := os.Getenv("PORT")
	oldAPIKey := os.Getenv("GEMINI_API_KEY")
	defer func() {
		os.Setenv("PORT", oldPort)
		os.Setenv("GEMINI_API_KEY", oldAPIKey)
	}()

	// Clear environment to make sure loadEnv sets them
	os.Unsetenv("PORT")
	os.Unsetenv("GEMINI_API_KEY")

	// Temporarily override envFiles to point to a test file
	oldEnvFiles := envFiles
	envFiles = []string{".env.test"}
	defer func() {
		envFiles = oldEnvFiles
		_ = os.Remove(".env.test")
	}()

	// Write custom test configurations to .env.test
	testEnv := []byte("PORT=9999\nGEMINI_API_KEY=\"test-env-key\"\n# This is a comment\n\nINVALID_LINE\n")
	err := os.WriteFile(".env.test", testEnv, 0644)
	if err != nil {
		t.Fatalf("failed to write test env file: %v", err)
	}

	loadEnv()

	if os.Getenv("PORT") != "9999" {
		t.Errorf("expected PORT=9999, got %s", os.Getenv("PORT"))
	}
	if os.Getenv("GEMINI_API_KEY") != "test-env-key" {
		t.Errorf("expected GEMINI_API_KEY=test-env-key, got %s", os.Getenv("GEMINI_API_KEY"))
	}
}
