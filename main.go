package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/sashabaranov/go-openai"
)

const maxToolRounds = 5
const maxRequestBodyBytes = 1 << 20 // 1MB

// Request & Response Schemas
type ErrorRequest struct {
	Log string `json:"log"`
}

type ResponseSchema struct {
	Summary         string   `json:"summary"`
	ConfidenceScore int      `json:"confidence_score"`
	ActionItems     []string `json:"action_items"`
}

// companyDocs holds the contents of all markdown files loaded from the companydocs/ folder at startup.
// Each key is the lowercase filename (without extension), and the value is the file content.
// Safe for concurrent reads since it's only written before the server starts.
var companyDocs map[string]string

// Package-level LLM configuration (initialized once in main)
var (
	llmClient *openai.Client
	llmModel  string
	llmTools  []openai.Tool
)

var markdownFenceRe = regexp.MustCompile("(?s)^```(?:json|JSON)?\\s*\n?(.*?)\\s*```$")

func loadCompanyDocs(dir string) {
	companyDocs = make(map[string]string)

	pattern := filepath.Join(dir, "*.md")
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Printf("[Docs Loader] Failed to glob %s: %v", pattern, err)
		return
	}

	for _, f := range files {
		content, err := os.ReadFile(f)
		if err != nil {
			log.Printf("[Docs Loader] Failed to read %s: %v", f, err)
			continue
		}
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		companyDocs[strings.ToLower(name)] = string(content)
		log.Printf("[Docs Loader] Loaded: %s", filepath.Base(f))
	}

	if len(companyDocs) == 0 {
		log.Printf("[Docs Loader] WARNING: No documents found in %s — the get_company_docs tool will have no data", dir)
	} else {
		log.Printf("[Docs Loader] Total documents loaded: %d", len(companyDocs))
	}
}

// Tool: Company Troubleshooting Docs (loaded from companydocs/*.md)
func getCompanyDocs(query string) string {
	log.Printf("[Tool Execution] Searching docs for query: %s", query)

	queryLower := strings.ToLower(query)
	var matches []string

	for name, content := range companyDocs {
		if strings.Contains(queryLower, name) || strings.Contains(name, queryLower) {
			matches = append(matches, content)
		} else if strings.Contains(strings.ToLower(content), queryLower) {
			matches = append(matches, content)
		}
	}

	if len(matches) > 0 {
		return strings.Join(matches, "\n---\n")
	}
	return "No matching documentation found. Check standard container stdout logs, ensure environment configurations are loaded correctly, and confirm cluster network connectivity."
}

// Tool: Mock Service Health Status Checker
func getServiceStatus(serviceName string) string {
	log.Printf("[Tool Execution] Checking service status for: %s", serviceName)

	services := map[string]string{
		"database":      `{"service": "database", "status": "degraded", "latency_ms": 4500, "connections_used": 98, "connections_max": 100, "last_incident": "Connection pool saturation detected 5m ago"}`,
		"auth-service":  `{"service": "auth-service", "status": "healthy", "latency_ms": 45, "uptime": "99.98%", "last_incident": "none"}`,
		"api-gateway":   `{"service": "api-gateway", "status": "healthy", "latency_ms": 12, "error_rate": "0.01%", "last_incident": "none"}`,
		"cache":         `{"service": "cache", "status": "warning", "latency_ms": 200, "memory_usage": "89%", "eviction_rate": "high", "last_incident": "Memory pressure 15m ago"}`,
		"message-queue": `{"service": "message-queue", "status": "healthy", "latency_ms": 8, "queue_depth": 42, "last_incident": "none"}`,
	}

	nameLower := strings.ToLower(serviceName)
	for key, val := range services {
		if strings.Contains(nameLower, key) || strings.Contains(key, nameLower) {
			return val
		}
	}

	fallback := struct {
		Service string `json:"service"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Service: serviceName,
		Status:  "unknown",
		Message: "Service not found in monitoring registry",
	}
	out, _ := json.Marshal(fallback)
	return string(out)
}

func initLLMClient() {
	apiBase := os.Getenv("LLM_API_BASE")
	apiKey := os.Getenv("LLM_API_KEY")
	llmModel = os.Getenv("LLM_MODEL")
	if llmModel == "" {
		llmModel = "gpt-4o-mini"
	}

	config := openai.DefaultConfig(apiKey)
	if apiBase != "" {
		config.BaseURL = apiBase
	}
	llmClient = openai.NewClientWithConfig(config)

	llmTools = []openai.Tool{
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_company_docs",
				Description: "Retrieve official internal company troubleshooting documentation for specific errors or keywords.",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"query": {
							"type": "string",
							"description": "Error code, keyword, or context to query documentation for."
						}
					},
					"required": ["query"]
				}`),
			},
		},
		{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        "get_service_status",
				Description: "Check the current health status and metrics of an infrastructure service (e.g., database, cache, api-gateway, auth-service, message-queue).",
				Parameters: json.RawMessage(`{
					"type": "object",
					"properties": {
						"service_name": {
							"type": "string",
							"description": "Name of the service to check status for (e.g., database, cache, auth-service)."
						}
					},
					"required": ["service_name"]
				}`),
			},
		},
	}

	log.Printf("[LLM] Client initialized (model=%s, base=%s)", llmModel, apiBase)
}

func main() {
	docsDir := os.Getenv("DOCS_DIR")
	if docsDir == "" {
		docsDir = "companydocs"
	}
	loadCompanyDocs(docsDir)
	initLLMClient()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/analyze", handleAnalyze)
	mux.HandleFunc("/healthz", handleHealthz)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown on SIGTERM/SIGINT
	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("First Responder App listening on port %s...", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	<-done
	log.Println("Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	log.Println("Server stopped")
}

func handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req ErrorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}

	systemInstruction := `You are a DevOps First Responder Agent. Analyze the provided error log.
You have access to the following tools to help you reason about the error:
- 'get_company_docs': Retrieve internal troubleshooting documentation for specific error codes or keywords.
- 'get_service_status': Check the real-time health status of infrastructure services (database, cache, api-gateway, etc.).

Use these tools to gather context before making your diagnosis. You MUST respond strictly with a valid JSON object matching this schema:
{
  "summary": "A 1-sentence summary of the problem.",
  "confidence_score": 85,
  "action_items": ["Action 1", "Action 2"]
}
Do not return any conversational text, markdown wrapping (like ` + "```json" + `), or explanations outside the JSON object.`

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemInstruction},
		{Role: openai.ChatMessageRoleUser, Content: fmt.Sprintf("Error Log: %s", req.Log)},
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// First LLM Call: Let the agent decide which tools to invoke
	resp, err := llmClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    llmModel,
		Messages: messages,
		Tools:    llmTools,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("LLM call failed: %v", err), http.StatusInternalServerError)
		return
	}
	if len(resp.Choices) == 0 {
		http.Error(w, "LLM returned no response", http.StatusInternalServerError)
		return
	}

	message := resp.Choices[0].Message

	// Agentic loop: execute tool calls until the LLM produces a final answer (capped)
	for round := 0; len(message.ToolCalls) > 0 && round < maxToolRounds; round++ {
		messages = append(messages, message)

		for _, toolCall := range message.ToolCalls {
			var result string

			switch toolCall.Function.Name {
			case "get_company_docs":
				var args struct {
					Query string `json:"query"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					log.Printf("[Tool Error] Failed to parse get_company_docs args: %v", err)
					result = "Error: could not parse tool arguments"
				} else {
					result = getCompanyDocs(args.Query)
				}

			case "get_service_status":
				var args struct {
					ServiceName string `json:"service_name"`
				}
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					log.Printf("[Tool Error] Failed to parse get_service_status args: %v", err)
					result = "Error: could not parse tool arguments"
				} else {
					result = getServiceStatus(args.ServiceName)
				}

			default:
				log.Printf("[Tool Error] Unknown tool requested: %s", toolCall.Function.Name)
				result = fmt.Sprintf("Error: unknown tool '%s'", toolCall.Function.Name)
			}

			messages = append(messages, openai.ChatCompletionMessage{
				Role:       openai.ChatMessageRoleTool,
				Content:    result,
				ToolCallID: toolCall.ID,
			})
		}

		// Follow-up LLM call with tool results
		followUp, err := llmClient.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    llmModel,
			Messages: messages,
			Tools:    llmTools,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("LLM follow-up call failed: %v", err), http.StatusInternalServerError)
			return
		}
		if len(followUp.Choices) == 0 {
			http.Error(w, "LLM returned no response", http.StatusInternalServerError)
			return
		}
		message = followUp.Choices[0].Message
	}

	if len(message.ToolCalls) > 0 {
		log.Printf("[Warning] LLM exceeded max tool rounds (%d), forcing response", maxToolRounds)
	}

	// Parse and validate structured output
	cleanJSON := strings.TrimSpace(message.Content)

	// Strip markdown fences robustly using regex
	if matches := markdownFenceRe.FindStringSubmatch(cleanJSON); len(matches) > 1 {
		cleanJSON = strings.TrimSpace(matches[1])
	}

	var validatedOutput ResponseSchema
	w.Header().Set("Content-Type", "application/json")

	if err := json.Unmarshal([]byte(cleanJSON), &validatedOutput); err != nil {
		log.Printf("[Parse Error] Failed to parse LLM output: %v | raw: %s", err, cleanJSON)
		fallback := ResponseSchema{
			Summary:         "Raw parsing error occurred while compiling analysis output.",
			ConfidenceScore: 0,
			ActionItems:     []string{"Escalate to Senior Dev", "Inspect application stdout logs manually"},
		}
		json.NewEncoder(w).Encode(fallback)
		return
	}

	// Marshal the validated struct to ensure only schema fields are returned
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(validatedOutput)
}
