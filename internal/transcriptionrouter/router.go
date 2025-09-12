package transcriptionrouter

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	llm "github.com/dooshek/voicify/internal/llm"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/plugin"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/types"
)

//go:embed prompts/*.md
var promptFiles embed.FS

func initializeResources() error {
	// Create prompts directory in current working directory
	promptsDir := "./prompts"
	if err := os.MkdirAll(promptsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create prompts directory: %w", err)
	}

	return fs.WalkDir(promptFiles, "prompts", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(d.Name()) != ".md" {
			return err
		}

		data, err := promptFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", path, err)
		}

		destPath := filepath.Join(promptsDir, filepath.Base(path))
		return os.WriteFile(destPath, data, 0o644)
	})
}

func init() {
	if err := initializeResources(); err != nil {
		logger.Errorf("Resource initialization failed: %v", err)
	}
}

type llmResponse struct {
	Action                      string `json:"action"`
	TranscriptionWithoutCommand string `json:"transcription_without_command"`
}

type Router struct {
	actions     []types.PluginAction
	llmProvider llm.Provider
	promptCache string
	pluginMgr   *plugin.Manager
}

// GetOrCreateGlobalRouter returns existing global router or creates new one
func GetOrCreateGlobalRouter() *Router {
	// Check if global router exists
	if existingRouter := state.Get().GetRouter(); existingRouter != nil {
		if router, ok := existingRouter.(*Router); ok {
			logger.Debug("Router: Using existing global router")
			return router
		}
	}

	// Create new router
	logger.Debug("Router: Creating new global router")
	router := createNewRouter()
	state.Get().SetRouter(router)
	return router
}

func createNewRouter() *Router {
	routerProvider := state.Get().GetRouterProvider()
	logger.Debugf("Router: Initializing with provider: '%s'", routerProvider)

	var provider llm.Provider
	var providerErr error

	// Only try to create LLM provider if router provider is configured
	if string(routerProvider) != "" {
		provider, providerErr = llm.NewProvider(routerProvider)
		if providerErr != nil {
			logger.Warnf("Router: Failed to create LLM provider for '%s': %v", routerProvider, providerErr)
		} else {
			logger.Debugf("Router: LLM provider created successfully for '%s'", routerProvider)
		}
	} else {
		logger.Infof("Router: No LLM router provider configured - LLM analysis will be skipped")
	}

	// Create a slice to hold all actions
	var actions []types.PluginAction

	// Get plugin actions
	var pluginMgr *plugin.Manager
	pluginMgr = plugin.NewManager()

	// Register all built-in plugins
	logger.Debugf("Router: Attempting to register plugins...")
	if err := plugin.RegisterAllPlugins(pluginMgr); err != nil {
		logger.Errorf("Failed to register plugins: %v", err)
	} else {
		logger.Debugf("Router: Successfully registered plugins")
		// Get actions from all plugins (use empty transcription for initialization)
		pluginActions := pluginMgr.GetAllActions("")
		actions = append(actions, pluginActions...)
		logger.Debugf("Router: Loaded %d plugin actions", len(pluginActions))
	}

	// Sort actions by priority (higher priority first)
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].GetMetadata().Priority > actions[j].GetMetadata().Priority
	})

	logger.Debugf("Router: Total actions loaded: %d", len(actions))
	if len(actions) == 0 {
		logger.Infof("Router: No plugin actions found - only basic routing will be available")
	}

	r := &Router{
		llmProvider: provider,
		actions:     actions,
		pluginMgr:   pluginMgr,
	}

	// Only cache prompt template if we have a working LLM provider
	if provider != nil {
		if err := r.cachePromptTemplate(); err != nil {
			logger.Error("Failed to cache prompt template", err)
		}
	} else {
		logger.Debugf("Router: Skipping prompt template caching - no LLM provider available")
	}

	return r
}

// New returns the global router (for backward compatibility)
func New(transcription string) *Router {
	return GetOrCreateGlobalRouter()
}

func (r *Router) cachePromptTemplate() error {
	promptPath := "./prompts/router.md"
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return fmt.Errorf("prompt template read failed: %w", err)
	}
	r.promptCache = string(data)
	return nil
}

func (r *Router) findAction(actionName string) types.PluginAction {
	logger.Debugf("Looking for action with name: %s", actionName)
	for _, a := range r.actions {
		meta := a.GetMetadata()
		if strings.EqualFold(meta.Name, actionName) {
			logger.Debugf("Found matching action: %s", meta.Name)
			return a
		}
		// LLMRouterPrompt is not used for exact matching - only LLM analysis
	}
	logger.Debugf("No action found for: %s", actionName)
	return nil
}

func (r *Router) Route(transcription string) error {
	logger.Debugf("Router: Starting routing for transcription: %s", transcription)

	// First pass: check which actions want to skip default action
	skipDefaultAction := false
	for _, a := range r.actions {
		if a.GetMetadata().LLMRouterPrompt == nil || *a.GetMetadata().LLMRouterPrompt == "" {
			meta := a.GetMetadata()
			if meta.SkipDefaultAction {
				skipDefaultAction = true
				logger.Debugf("Router: Action %s will skip default action", meta.Name)
			}
		}
	}

	// Second pass: execute actions (skip default if needed)
	nonLLMActionsExecuted := 0
	for _, a := range r.actions {
		if a.GetMetadata().LLMRouterPrompt == nil || *a.GetMetadata().LLMRouterPrompt == "" {
			meta := a.GetMetadata()

			// Skip default action if any plugin has set SkipDefaultAction to true
			if meta.Name == "default" && skipDefaultAction {
				logger.Debug("Router: Skipping default action - another plugin already handled it")
				continue
			}

			logger.Debugf("Router: Executing non-LLM action: %s", meta.Name)
			nonLLMActionsExecuted++

			if err := a.Execute(transcription); err != nil {
				logger.Errorf("Action %s failed to execute", err, meta.Name)
			}
		}
	}
	logger.Debugf("Router: Executed %d non-LLM actions", nonLLMActionsExecuted)

	// Check if there are any actions with LLMRouterPrompt
	hasLLMActions := false
	llmActionCount := 0
	for _, a := range r.actions {
		if meta := a.GetMetadata(); meta.LLMRouterPrompt != nil && *meta.LLMRouterPrompt != "" {
			hasLLMActions = true
			llmActionCount++
		}
	}

	// Skip LLM analysis if there are no actions with LLMRouterPrompt
	if !hasLLMActions {
		logger.Debugf("Router: Skipping LLM analysis - no actions with LLMRouterPrompt defined")
		return nil
	}

	// Skip LLM analysis if no LLM provider is available
	if r.llmProvider == nil {
		logger.Infof("Router: Skipping LLM analysis - no LLM provider configured (found %d actions requiring LLM)", llmActionCount)
		return nil
	}

	logger.Debugf("Router: Starting LLM analysis with %d LLM-enabled actions", llmActionCount)
	llmResp, err := r.analyzeWithLLM(transcription)
	if err != nil {
		logger.Error("LLM analysis failed", err)
		return nil
	}

	logger.Debugf("LLM suggested action: %s", llmResp.Action)
	if action := r.findAction(llmResp.Action); action != nil {
		logger.Debugf("Executing LLM-selected action: %s with transcription: %s",
			action.GetMetadata().Name, llmResp.TranscriptionWithoutCommand)
		err := action.Execute(llmResp.TranscriptionWithoutCommand)
		if err != nil {
			logger.Errorf("LLM-selected action %s failed", err, action.GetMetadata().Name)
		} else {
			logger.Debugf("Action %s: LLM-selected action completed successfully", action.GetMetadata().Name)
		}
		return err
	}
	logger.Debugf("No action executed for this transcription")
	return nil
}

func (r *Router) analyzeWithLLM(transcription string) (*llmResponse, error) {
	logger.Debugf("Router: Starting LLM analysis for transcription: %s", transcription)

	actionsDoc := strings.Builder{}

	logger.Debugf("Building LLM actions documentation with %d available actions", len(r.actions))
	for _, a := range r.actions {
		if meta := a.GetMetadata(); meta.LLMRouterPrompt != nil && *meta.LLMRouterPrompt != "" {
			logger.Debugf("Adding action to LLM prompt: %s", meta.Name)
			actionsDoc.WriteString(fmt.Sprintf("- %s\n", *meta.LLMRouterPrompt))
		}
	}

	prompt := fmt.Sprintf(string(r.promptCache), actionsDoc.String(), transcription)
	logger.Debugf("LLM request prompt: %+v", prompt)

	req := llm.CompletionRequest{
		Model:       state.Get().Config.LLM.Router.Model,
		Messages:    []llm.ChatCompletionMessage{{Role: "user", Content: prompt}},
		Temperature: float32(state.Get().Config.LLM.Router.Temperature),
	}

	response, err := r.llmProvider.Completion(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	logger.Debugf("Received LLM response: %+v", response)

	var llmResp llmResponse
	if err := json.Unmarshal([]byte(response), &llmResp); err != nil {
		logger.Error(fmt.Sprintf("Failed to parse LLM response: %s", response), err)
		return nil, fmt.Errorf("LLM response parsing failed: %w", err)
	}

	logger.Debugf("LLM response parsed successfully: action=%s, transcription=%s",
		llmResp.Action, llmResp.TranscriptionWithoutCommand)

	return &llmResp, nil
}

