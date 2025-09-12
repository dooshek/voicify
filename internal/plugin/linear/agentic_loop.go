package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dooshek/voicify/internal/llm"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/tts"
)

// AgenticLoopState represents the current state of the agentic loop
type AgenticLoopState string

const (
	StateIdle            AgenticLoopState = "idle"
	StateAnalyzing       AgenticLoopState = "analyzing"
	StateAskingQuestion  AgenticLoopState = "asking_question"
	StateAnswering       AgenticLoopState = "answering"
	StateWaitingResponse AgenticLoopState = "waiting_response"
	StateExecutingTools  AgenticLoopState = "executing_tools"
	StateCompleted       AgenticLoopState = "completed"
	StateError           AgenticLoopState = "error"
)


// AgenticLoop manages the conversational flow using MCP tools
type AgenticLoop struct {
	mu                 sync.RWMutex
	state              AgenticLoopState
	ttsManager         *tts.Manager
	mcpClient          *LinearMCPClient
	availableTools     []MCPTool
	conversationHistory []string
	userIntent         string
	currentQuestion    string
	questionSpoken     bool
	ctx                context.Context
	cancel             context.CancelFunc
}

// NewAgenticLoop creates a new agentic loop instance
func NewAgenticLoop() (*AgenticLoop, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Get TTS manager from global state
	ttsManagerInterface := state.Get().GetTTSManager()
	ttsManager, ok := ttsManagerInterface.(*tts.Manager)
	if !ok {
		cancel()
		return nil, fmt.Errorf("TTS manager not available or wrong type")
	}

	// Get or create global MCP client for Linear
	mcpClient, err := getOrCreateGlobalMCPClient()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to get Linear MCP client: %w", err)
	}

	loop := &AgenticLoop{
		state:               StateIdle,
		ttsManager:          ttsManager,
		mcpClient:           mcpClient,
		conversationHistory: make([]string, 0),
		ctx:                 ctx,
		cancel:              cancel,
	}

	logger.Debug("Agentic loop initialized")
	return loop, nil
}

// getOrCreateGlobalMCPClient returns existing global MCP client or creates new one
func getOrCreateGlobalMCPClient() (*LinearMCPClient, error) {
	// Check if global MCP client exists
	if existingClient := state.Get().GetLinearMCPClient(); existingClient != nil {
		if client, ok := existingClient.(*LinearMCPClient); ok {
			logger.Debug("Using existing global Linear MCP client")
			return client, nil
		}
	}

	// Create new MCP client
	logger.Debug("Creating new global Linear MCP client")
	client, err := NewLinearMCPClient()
	if err != nil {
		return nil, err
	}

	// Store in global state
	state.Get().SetLinearMCPClient(client)
	return client, nil
}

// Start begins the agentic loop process
func (al *AgenticLoop) Start(initialTranscription string) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.state != StateIdle {
		return fmt.Errorf("agentic loop is already running")
	}

	logger.Infof("Starting agentic loop with transcription: %s", initialTranscription)

	// Store user intent
	al.userIntent = initialTranscription
	al.conversationHistory = append(al.conversationHistory, fmt.Sprintf("User: %s", initialTranscription))

	// Start the analysis process
	al.state = StateAnalyzing
	go al.runLoop()

	return nil
}

// Stop stops the agentic loop
func (al *AgenticLoop) Stop() {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.state == StateIdle {
		return
	}

	logger.Debug("Stopping agentic loop")
	al.cancel()
	al.state = StateIdle
}

// GetState returns the current state of the loop
func (al *AgenticLoop) GetState() AgenticLoopState {
	al.mu.RLock()
	defer al.mu.RUnlock()
	return al.state
}

// ProcessResponse processes user's voice response
func (al *AgenticLoop) ProcessResponse(response string) error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.state != StateWaitingResponse {
		return fmt.Errorf("not waiting for response, current state: %s", al.state)
	}

	logger.Debugf("Processing response: %s", response)

	// Add to conversation history
	al.conversationHistory = append(al.conversationHistory, fmt.Sprintf("User: %s", response))

	// Move to analysis state
	al.state = StateAnalyzing
	// Don't start new goroutine - the existing runLoop will continue

	return nil
}

// runLoop runs the main loop logic
func (al *AgenticLoop) runLoop() {
	for {
		select {
		case <-al.ctx.Done():
			logger.Debug("Agentic loop context cancelled")
			return
		default:
		}

		al.mu.Lock()
		currentState := al.state
		al.mu.Unlock()

		switch currentState {
		case StateAnalyzing:
			al.handleAnalyzing()
		case StateAskingQuestion:
			al.handleAskingQuestion()
		case StateAnswering:
			al.handleAnswering()
		case StateExecutingTools:
			al.handleExecutingTools()
		case StateCompleted:
			logger.Debug("Agentic loop completed successfully")
			al.handleCompletion()
			return
		case StateError:
			logger.Debug("Agentic loop ended with error")
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// handleAnalyzing handles the analysis state using LLM
func (al *AgenticLoop) handleAnalyzing() {
	logger.Debug("Analyzing conversation and determining next action")

	al.mu.Lock()
	conversationHistory := make([]string, len(al.conversationHistory))
	copy(conversationHistory, al.conversationHistory)
	userIntent := al.userIntent
	al.mu.Unlock()

	// Get available MCP tools
	tools, err := al.mcpClient.GetAvailableTools()
	if err != nil {
		logger.Errorf("Failed to get available tools: %v", err)
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
		return
	}

	// Use LLM to analyze conversation and determine next action
	nextAction, question, err := al.analyzeWithLLM(conversationHistory, userIntent, tools)
	if err != nil {
		logger.Errorf("LLM analysis failed: %v", err)
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
		return
	}

	logger.Debugf("LLM determined next action: %s", nextAction)

	switch nextAction {
	case "ask_question":
		// Check if this is a question or an answer to user's question
		if strings.Contains(strings.ToLower(question), "znalazłem") ||
		   strings.Contains(strings.ToLower(question), "jest") ||
		   strings.Contains(strings.ToLower(question), "ticketów") {
			// This is an answer, not a question
			al.mu.Lock()
			al.currentQuestion = question
			al.questionSpoken = false
			al.state = StateAnswering
			al.mu.Unlock()
		} else {
			// This is a question to user
			al.mu.Lock()
			al.currentQuestion = question
			al.questionSpoken = false
			al.state = StateAskingQuestion
			al.mu.Unlock()
		}
	case "execute_tools":
		al.mu.Lock()
		al.state = StateExecutingTools
		al.mu.Unlock()
	case "complete":
		logger.Info("Agentic loop task completed successfully")
		al.mu.Lock()
		al.state = StateCompleted
		al.mu.Unlock()
	default:
		logger.Warnf("Unknown action: %s", nextAction)
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
	}
}

// handleAskingQuestion handles asking questions via TTS
func (al *AgenticLoop) handleAskingQuestion() {
	al.mu.Lock()
	question := al.currentQuestion
	questionSpoken := al.questionSpoken
	al.mu.Unlock()

	if question == "" {
		logger.Warn("No question to ask")
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
		return
	}

	// Skip if question was already spoken
	if questionSpoken {
		logger.Debug("Question already spoken, staying in waiting state")
		return
	}

	logger.Debugf("Speaking question: %s", question)

	// Use TTS to ask the question
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	question = fmt.Sprintf("Powiedz dokładnie tylko to co jest w cudzysłowie: \"%s\"", question)
	if err := al.ttsManager.Speak(ctx, question); err != nil {
		logger.Errorf("Failed to speak question: %v", err)
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
		return
	}

	// Add to conversation history and mark as spoken
	al.mu.Lock()
	al.conversationHistory = append(al.conversationHistory, fmt.Sprintf("Assistant: %s", question))
	al.questionSpoken = true
	al.state = StateWaitingResponse
	al.mu.Unlock()

	logger.Debug("Question spoken, waiting for user response")
}

// handleAnswering handles providing answers to user via TTS
func (al *AgenticLoop) handleAnswering() {
	al.mu.Lock()
	answer := al.currentQuestion // currentQuestion now contains the answer
	answerSpoken := al.questionSpoken
	al.mu.Unlock()

	if answer == "" {
		logger.Warn("No answer to provide")
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
		return
	}

	// Skip if answer was already spoken
	if answerSpoken {
		logger.Debug("Answer already spoken, completing task")
		al.mu.Lock()
		al.state = StateCompleted
		al.mu.Unlock()
		return
	}

	logger.Debugf("Speaking answer: %s", answer)

	// Use TTS to provide the answer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	answer = fmt.Sprintf("Powiedz dokładnie tylko to co jest w cudzysłowie: \"%s\"", answer)
	if err := al.ttsManager.Speak(ctx, answer); err != nil {
		logger.Errorf("Failed to speak answer: %v", err)
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
		return
	}

	// Add to conversation history and mark as spoken
	al.mu.Lock()
	al.conversationHistory = append(al.conversationHistory, fmt.Sprintf("Assistant: %s", answer))
	al.questionSpoken = true
	al.state = StateWaitingResponse // Wait for user's next input instead of completing
	al.mu.Unlock()

	logger.Debug("Answer spoken, waiting for user's next input")
}

// handleExecutingTools handles executing MCP tools
func (al *AgenticLoop) handleExecutingTools() {
	logger.Debug("Executing MCP tools")

	al.mu.Lock()
	conversationHistory := make([]string, len(al.conversationHistory))
	copy(conversationHistory, al.conversationHistory)
	userIntent := al.userIntent
	al.mu.Unlock()

	// Use LLM to determine which tools to execute and with what parameters
	toolsToExecute, err := al.determineToolsToExecute(conversationHistory, userIntent)
	if err != nil {
		logger.Errorf("Failed to determine tools to execute: %v", err)
		al.mu.Lock()
		al.state = StateError
		al.mu.Unlock()
		return
	}

	// Execute the tools
	results := make([]string, 0)
	for _, toolCall := range toolsToExecute {
		logger.Debugf("Executing tool: %s with params: %+v", toolCall.Name, toolCall.Parameters)

		result, err := al.mcpClient.ExecuteTool(toolCall.Name, toolCall.Parameters)
		if err != nil {
			logger.Errorf("Failed to execute tool %s", err, toolCall.Name)
			// Add error to conversation history so LLM can learn from it
			errorMsg := fmt.Sprintf("OSTATNI BŁĄD wykonania narzędzia %s: %s", toolCall.Name, err.Error())
			results = append(results, errorMsg)
			continue
		}

		results = append(results, fmt.Sprintf("Tool %s result: %s", toolCall.Name, result))
		logger.Debugf("Tool %s executed successfully", toolCall.Name)
	}

	// Add results to conversation history
	al.mu.Lock()
	for _, result := range results {
		al.conversationHistory = append(al.conversationHistory, result)
	}
	al.state = StateAnalyzing
	al.mu.Unlock()

	// Continue analysis with tool results - but don't start new goroutine!
	// The current runLoop will continue in the next iteration
}

// handleCompletion handles the completion state and provides voice summary
func (al *AgenticLoop) handleCompletion() {
	al.mu.Lock()
	conversationHistory := make([]string, len(al.conversationHistory))
	copy(conversationHistory, al.conversationHistory)
	userIntent := al.userIntent
	al.mu.Unlock()

	// Generate completion summary using LLM
	summary, err := al.generateCompletionSummary(conversationHistory, userIntent)
	if err != nil {
		logger.Errorf("Failed to generate completion summary: %v", err)
		summary = "Zadanie zostało zakończone."
	}

	logger.Infof("Agentic loop completed: %s", summary)

	// Speak the summary
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := al.ttsManager.Speak(ctx, summary); err != nil {
		logger.Errorf("Failed to speak completion summary: %v", err)
	}

	logger.Debug("Completion summary spoken")
}

// generateCompletionSummary generates a summary of what was accomplished
func (al *AgenticLoop) generateCompletionSummary(conversationHistory []string, userIntent string) (string, error) {
	conversation := strings.Join(conversationHistory, "\n")

	prompt := fmt.Sprintf(`Przeanalizuj konwersację i napisz BARDZO KRÓTKĄ odpowiedź (maksymalnie 1 zdanie, 10-15 słów).

Historia konwersacji:
%s

Intencja użytkownika: %s

Napisz tylko krótkie potwierdzenie co zostało zrobione. PRZYKŁADY dobrych odpowiedzi:
- "Znalazłam 23 tickety."
- "Zaktualizowałam ticket PIL-521."
- "Utworzyłam nowy ticket."

NIE pisz długich podsumowań. Odpowiedz tylko krótkim potwierdzeniem.`, conversation, userIntent)

	// Use LLM provider
	llmProvider, err := llm.NewProvider(state.Get().GetRouterProvider())
	if err != nil {
		return "", fmt.Errorf("failed to create LLM provider: %w", err)
	}

	req := llm.CompletionRequest{
		Model:       state.Get().GetRouterModel(),
		Messages:    []llm.ChatCompletionMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3,
	}

	response, err := llmProvider.Completion(context.Background(), req)
	if err != nil {
		return "", fmt.Errorf("LLM completion failed: %w", err)
	}

	return strings.TrimSpace(response), nil
}

// analyzeWithLLM uses LLM to analyze conversation and determine next action
func (al *AgenticLoop) analyzeWithLLM(conversationHistory []string, userIntent string, tools []MCPTool) (string, string, error) {
	// Build prompt for LLM
	prompt := al.buildAnalysisPrompt(conversationHistory, userIntent, tools)

	// Use LLM provider from state
	llmProvider, err := llm.NewProvider(state.Get().GetRouterProvider())
	if err != nil {
		return "", "", fmt.Errorf("failed to create LLM provider: %w", err)
	}

	req := llm.CompletionRequest{
		Model:       state.Get().GetRouterModel(),
		Messages:    []llm.ChatCompletionMessage{{Role: "user", Content: prompt}},
		Temperature: 0.7,
	}

	response, err := llmProvider.Completion(context.Background(), req)
	if err != nil {
		return "", "", fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse response
	var analysis struct {
		Action   string `json:"action"`
		Question string `json:"question,omitempty"`
		Reason   string `json:"reason"`
	}

	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		return "", "", fmt.Errorf("failed to parse LLM response: %w", err)
	}

	logger.Debugf("LLM analysis: %+v", analysis)
	return analysis.Action, analysis.Question, nil
}

// buildAnalysisPrompt builds the prompt for LLM analysis
func (al *AgenticLoop) buildAnalysisPrompt(conversationHistory []string, userIntent string, tools []MCPTool) string {
	toolsList := ""
	for _, tool := range tools {
		toolsList += fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description)
	}

	conversation := strings.Join(conversationHistory, "\n")

	// Determine if we're in the middle of a conversation
	hasUserQuestions := strings.Contains(strings.ToLower(userIntent), "ile") ||
		strings.Contains(strings.ToLower(userIntent), "poszukaj") ||
		strings.Contains(strings.ToLower(userIntent), "znajdź") ||
		strings.Contains(strings.ToLower(userIntent), "pokaż")

	statusNote := ""
	if hasUserQuestions {
		statusNote = "\n🔴 STAN: ŚRODEK KONWERSACJI - Użytkownik zadał pytanie i oczekuje odpowiedzi. NIE KOŃCZ dopóki nie udzielisz pełnej odpowiedzi!"
	} else {
		statusNote = "\n🟢 STAN: NOWE ZADANIE - Możesz rozpocząć nowe zadanie lub zakończyć jeśli wszystko zostało wykonane."
	}

	return fmt.Sprintf(`Jesteś asystentem AI pomagającym użytkownikowi w tworzeniu i zarządzaniu ticketami w Linear.%s

Dostępne narzędzia MCP:
%s

Historia konwersacji:
%s

Intencja użytkownika: %s

Przeanalizuj konwersację i zdecyduj co robić dalej. Możesz:
1. "ask_question" - zadać pytanie użytkownikowi (jeśli potrzebujesz więcej informacji)
2. "execute_tools" - wykonać narzędzia MCP (jeśli masz wystarczająco informacji)
3. "complete" - zakończyć proces (TYLKO jeśli odpowiedziałeś na WSZYSTKIE pytania użytkownika i zadanie zostało w pełni wykonane)

KRYTYCZNE ZASADY:
- Jeśli użytkownik zadał pytanie (np. "ile ticketów?"), MUSISZ na nie odpowiedzieć
- Jeśli wykonałeś narzędzia i masz wyniki, MUSISZ je przeanalizować i odpowiedzieć użytkownikowi
- Po udzieleniu odpowiedzi, zaproponuj dalsze akcje (np. "Czy chcesz usunąć te duplikaty?" lub "Czy mam coś zrobić z tymi ticketami?")
- NIE używaj "complete" dopóki użytkownik nie powie że kończy (np. "dziękuję", "to wszystko", "koniec")
- Kontynuuj konwersację - zawsze pytaj co dalej po udzieleniu odpowiedzi
- ANALIZUJ BŁĘDY: Jeśli widzisz "BŁĄD wykonania narzędzia" w historii, przeanalizuj błąd i spróbuj inne podejście z poprawnymi parametrami

STYL KOMUNIKACJI:
- Odpowiedzi KRÓTKIE i ZWIĘZŁE (maksymalnie 1 zdanie)
- IMPORTANT:NIE używaj długich wyjaśnień, nie zadawaj zbędnych pytań oprócz, na końcu zdania zapytaj co dalej w stylu "coś jeszcze?", albo "podać?", albo "co dalej?"
- PRZYKŁADY dobrych odpowiedzi: "Znalazłam 23 tickety. Czy je usunąć?", "Ticket został utworzony. Coś jeszcze?"

Odpowiedz w formacie JSON:
{
  "action": "ask_question|execute_tools|complete",
  "question": "pytanie do użytkownika (jeśli action=ask_question) LUB odpowiedź na pytanie użytkownika (jeśli masz już wyniki), krótkie, precyzyjne",
  "reason": "uzasadnienie decyzji"
}

IMPORTANT: ZAWSZE odpowiadaj w formacie JSON beż zbędnych innych znaków na początku i na końcu, bez komentarzy.
`, statusNote, toolsList, conversation, userIntent)
}

// determineToolsToExecute uses LLM to determine which tools to execute
func (al *AgenticLoop) determineToolsToExecute(conversationHistory []string, userIntent string) ([]ToolCall, error) {
	// Get available tools
	tools, err := al.mcpClient.GetAvailableTools()
	if err != nil {
		return nil, err
	}

	// Build prompt for tool selection
	prompt := al.buildToolSelectionPrompt(conversationHistory, userIntent, tools)

	// Use LLM provider
	llmProvider, err := llm.NewProvider(state.Get().GetRouterProvider())
	if err != nil {
		return nil, fmt.Errorf("failed to create LLM provider: %w", err)
	}

	req := llm.CompletionRequest{
		Model:       state.Get().GetRouterModel(),
		Messages:    []llm.ChatCompletionMessage{{Role: "user", Content: prompt}},
		Temperature: 0.3, // Lower temperature for more consistent tool selection
	}

	response, err := llmProvider.Completion(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	// Parse response
	var toolCalls struct {
		Tools []ToolCall `json:"tools"`
	}

	// Clean response (remove markdown backticks if present)
	cleanResponse := strings.TrimSpace(response)
	if strings.HasPrefix(cleanResponse, "```json") {
		cleanResponse = strings.TrimPrefix(cleanResponse, "```json")
	}
	if strings.HasPrefix(cleanResponse, "```") {
		cleanResponse = strings.TrimPrefix(cleanResponse, "```")
	}
	if strings.HasSuffix(cleanResponse, "```") {
		cleanResponse = strings.TrimSuffix(cleanResponse, "```")
	}
	cleanResponse = strings.TrimSpace(cleanResponse)

	logger.Debugf("Cleaned LLM response: %s", cleanResponse)

	if err := json.Unmarshal([]byte(cleanResponse), &toolCalls); err != nil {
		logger.Errorf("Failed to parse tool calls", err)
		logger.Debugf("Failed response: %s", cleanResponse)
		return nil, fmt.Errorf("failed to parse tool calls: %w", err)
	}

	return toolCalls.Tools, nil
}

// buildToolSelectionPrompt builds the prompt for tool selection
func (al *AgenticLoop) buildToolSelectionPrompt(conversationHistory []string, userIntent string, tools []MCPTool) string {
	toolsList := ""
	for _, tool := range tools {
		toolsList += fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description)
	}

	conversation := strings.Join(conversationHistory, "\n")

	return fmt.Sprintf(`Jesteś asystentem AI pomagającym użytkownikowi w tworzeniu i zarządzaniu i raportowaniu ticketami w Linear.

Dostępne narzędzia MCP:
%s

Historia konwersacji:
%s

Intencja użytkownika: %s

Na podstawie konwersacji i intencji użytkownika, wybierz które narzędzia MCP wykonać i z jakimi parametrami

Odpowiedz w formacie JSON:
{
  "tools": [
    {
      "name": "nazwa_narzędzia",
      "parameters": {
        "param1": "wartość1",
        "param2": "wartość2"
      }
    }
  ]
}

IMPORTANT: ZAWSZE odpowiadaj w formacie JSON beż zbędnych innych znaków na początku i na końcu, bez komentarzy.`, toolsList, conversation, userIntent)
}
