package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
)

// Request & Response Schemas
type ErrorRequest struct {
	Log string `json:"log"`
}

type ResponseSchema struct {
	Summary         string   `json:"summary"`
	ConfidenceScore int      `json:"confidence_score"`
	ActionItems     []string `json:"action_items"`
}

// Tool Definition: Mock Company Troubleshooting Docs
func getCompanyDocs(query string) string {
	log.Printf("[Tool Execution] Searching docs for query: %s", query)
	
	docs := map[string]string{
		"500": "Internal Server Error: Often caused by database connection timeouts, broken downstream microservices, or panic states. Action: Check database connection pools, verify upstream service health, and restart the affected pod.",
		"403": "Forbidden: Permission denied. Typically a misconfigured RBAC policy, expired JWT token, or incorrect API gateway routing headers. Action: Verify service account roles, refresh identity tokens, and audit network policies.",
		"db":  "Database Failure: Connection pool exhausted. Action: Scale up database replicas or adjust connection limits in environment variables.",
	}

	queryLower := strings.ToLower(query)
	for key, val := range docs {
		if strings.Contains(queryLower, key) {
			return val
		}
	}
	return "Generic Error Guide: Check standard container stdout logs, ensure environment configurations are loaded correctly, and confirm cluster network connectivity."
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/analyze", handleAnalyze)

	log.Printf("First Responder App listening on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST requests are allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ErrorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON input", http.StatusBadRequest)
		return
	}

	// Configure LLM Client (Supports OpenAI, LocalAI, Ollama via OpenAI compatibility layer)
	apiBase := os.Getenv("LLM_API_BASE") // e.g., http://ollama-service.default.svc.cluster.local:11434/v1
	apiKey := os.Getenv("LLM_API_KEY")   // Optional for local deployments
	modelName := os.Getenv("LLM_MODEL")  // e.g., llama3, gpt-4o-mini
	if modelName == "" {
		modelName = "gpt-4o-mini" // fallback default
	}

	config := openai.DefaultConfig(apiKey)
	if apiBase != "" {
		config.BaseURL = apiBase
	}
	client := openai.NewClientWithConfig(config)

	// Define the tool signature for the LLM
	tools := []openai.Tool{
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
	}

	// System prompt enforcing the required structured output schema
	systemInstruction := `You are a DevOps First Responder Agent. Analyze the provided error log. 
You can use the 'get_company_docs' tool to lookup historical steps for resolving this error. 
You MUST respond strictly with a valid JSON object matching this schema:
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// First LLM Call: Let it decide whether to call a tool
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    modelName,
		Messages: messages,
		Tools:    tools,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("LLM call failed: %v", err), http.StatusInternalServerError)
		return
	}

	message := resp.Choices[0].Message

	// If LLM requested a tool call, execute it
	if len(message.ToolCalls) > 0 {
		messages = append(messages, message) // append assistant response containing tool call request

		for _, toolCall := range message.ToolCalls {
			if toolCall.Function.Name == "get_company_docs" {
				var args struct {
					Query string `json:"query"`
				}
				_ = json.Unmarshal([]byte(toolCall.Function.Arguments), &args)
				
				// Execute Tool
				docResult := getCompanyDocs(args.Query)

				// Feed tool result back to the LLM
				messages = append(messages, openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    docResult,
					ToolCallID: toolCall.ID,
				})
			}
		}

		// Second LLM Call: Generate final answer with the context retrieved from the tool
		finalResp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    modelName,
			Messages: messages,
		})
		if err != nil {
			http.Error(w, fmt.Sprintf("LLM final evaluation failed: %v", err), http.StatusInternalServerError)
			return
		}
		message = finalResp.Choices[0].Message
	}

	// Parse and validate structured output
	var validatedOutput ResponseSchema
	cleanJSON := strings.TrimSpace(message.Content)
	// Strip markdown blocks if the LLM ignored formatting rules
	cleanJSON = strings.TrimPrefix(cleanJSON, "```json")
	cleanJSON = strings.TrimSuffix(cleanJSON, "```")
	cleanJSON = strings.TrimSpace(cleanJSON)

	w.Header().Set("Content-Type", "application/json")
	if err := json.Unmarshal([]byte(cleanJSON), &validatedOutput); err != nil {
		// If formatting failed, construct a fallback JSON safely rather than returning broken text
		fallback := ResponseSchema{
			Summary:         "Raw parsing error occurred while compiling analysis output.",
			ConfidenceScore: 0,
			ActionItems:     []string{"Escalate to Senior Dev", "Inspect application stdout logs manually"},
		}
		json.NewEncoder(w).Encode(fallback)
		return
	}

	// Write successful structured response back to client
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(cleanJSON))
}
