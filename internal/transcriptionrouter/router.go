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

	"github.com/dooshek/voicify/internal/fileops"
	llm "github.com/dooshek/voicify/internal/llm"
	"github.com/dooshek/voicify/internal/logger"
	"github.com/dooshek/voicify/internal/plugin"
	"github.com/dooshek/voicify/internal/state"
	"github.com/dooshek/voicify/internal/types"
)

//go:embed prompts/*.md
var promptFiles embed.FS

func initializeResources() error {
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		return fmt.Errorf("file ops initialization failed: %w", err)
	}

	return fs.WalkDir(promptFiles, "prompts", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(d.Name()) != ".md" {
			return err
		}

		data, err := promptFiles.ReadFile(path)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", path, err)
		}

		destPath := filepath.Join(fileOps.GetPromptsDir(), filepath.Base(path))
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

func New(transcription string) *Router {
	provider, _ := llm.NewProvider(state.Get().GetRouterProvider())

	// Create a slice to hold all actions
	var actions []types.PluginAction

	// Get plugin actions
	fileOps, err := fileops.NewDefaultFileOps()
	var pluginMgr *plugin.Manager

	if err == nil {
		pluginsDir := fileOps.GetPluginsDir()
		pluginMgr = plugin.NewManager(pluginsDir)

		// Load plugins
		if err := pluginMgr.LoadPlugins(); err != nil {
			logger.Errorf("Failed to load plugins: %v", err)
		} else {
			// Get actions from all plugins
			pluginActions := pluginMgr.GetAllActions(transcription)
			actions = append(actions, pluginActions...)
		}
	} else {
		logger.Errorf("Failed to initialize file operations: %v", err)
	}

	// Sort actions by priority (higher priority first)
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].GetMetadata().Priority > actions[j].GetMetadata().Priority
	})

	r := &Router{
		llmProvider: provider,
		actions:     actions,
		pluginMgr:   pluginMgr,
	}

	if err := r.cachePromptTemplate(); err != nil {
		logger.Error("Failed to cache prompt template", err)
	}
	return r
}

func (r *Router) cachePromptTemplate() error {
	fileOps, err := fileops.NewDefaultFileOps()
	if err != nil {
		return fmt.Errorf("fileops creation failed: %w", err)
	}

	promptPath := filepath.Join(fileOps.GetPromptsDir(), "router.md")
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
		if meta.LLMCommands != nil {
			for _, cmd := range *meta.LLMCommands {
				if strings.EqualFold(cmd, actionName) {
					logger.Debugf("Found matching action via LLM command: %s => %s", actionName, meta.Name)
					return a
				}
			}
		}
	}
	logger.Debugf("No action found for: %s", actionName)
	return nil
}

func (r *Router) Route(transcription string) error {
	logger.Debugf("Routing transcription: %s", sanitizeLog(transcription))

	// Run first actions that has LLMAction set to false
	for _, a := range r.actions {
		if a.GetMetadata().LLMCommands == nil || len(*a.GetMetadata().LLMCommands) == 0 {
			logger.Debugf("Executing action: %s", a.GetMetadata().Name)

			if err := a.Execute(transcription); err != nil {
				logger.Errorf("Action %s failed to execute", err, a.GetMetadata().Name)
			}
		}
	}

	// Check if there are any actions with LLMCommands
	hasLLMActions := false
	for _, a := range r.actions {
		if meta := a.GetMetadata(); meta.LLMCommands != nil && len(*meta.LLMCommands) > 0 {
			hasLLMActions = true
			break
		}
	}

	// Skip LLM analysis if there are no actions with LLMCommands
	if !hasLLMActions {
		logger.Debugf("Skipping LLM analysis - no actions with LLMCommands defined")
		return nil
	}

	llmResp, err := r.analyzeWithLLM(transcription)
	if err != nil {
		logger.Error("LLM analysis failed", err)
		return nil
	}

	logger.Debugf("LLM suggested action: %s", llmResp.Action)
	if action := r.findAction(llmResp.Action); action != nil {
		logger.Debugf("Executing LLM-selected action: %s with transcription: %s",
			action.GetMetadata().Name, sanitizeLog(llmResp.TranscriptionWithoutCommand))
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
	actionsDoc := strings.Builder{}

	logger.Debugf("Building LLM actions documentation with %d available actions", len(r.actions))
	for _, a := range r.actions {
		if meta := a.GetMetadata(); meta.LLMCommands != nil && len(*meta.LLMCommands) > 0 {
			commandsStr := strings.Join(*meta.LLMCommands, "|")
			logger.Debugf("Adding action to LLM prompt: %s (commands: %s)", meta.Name, commandsStr)
			actionsDoc.WriteString(fmt.Sprintf("- %s: %s (commands: %s)\n",
				meta.Name, meta.Description, commandsStr))
		}
	}

	prompt := fmt.Sprintf(string(r.promptCache), actionsDoc.String(), transcription)
	logger.Debugf("LLM request prompt: %s", sanitizeLog(prompt))

	req := llm.CompletionRequest{
		Model:       state.Get().Config.LLM.Router.Model,
		Messages:    []llm.ChatCompletionMessage{{Role: "user", Content: prompt}},
		Temperature: float32(state.Get().Config.LLM.Router.Temperature),
	}

	logger.Debugf("Sending completion request with model: %s", req.Model)
	response, err := r.llmProvider.Completion(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	logger.Debugf("Received LLM response: %s", sanitizeLog(response))

	var llmResp llmResponse
	if err := json.Unmarshal([]byte(response), &llmResp); err != nil {
		logger.Error(fmt.Sprintf("Failed to parse LLM response: %s", sanitizeLog(response)), err)
		return nil, fmt.Errorf("LLM response parsing failed: %w", err)
	}

	logger.Debugf("LLM response parsed successfully: action=%s, transcription=%s",
		llmResp.Action, sanitizeLog(llmResp.TranscriptionWithoutCommand))

	return &llmResp, nil
}

func sanitizeLog(input string) string {
	if logger.GetCurrentLevel() != logger.LevelDebug {
		return "[redacted]"
	}
	if len(input) > 100 {
		return input[:100] + "..."
	}
	return input
}
