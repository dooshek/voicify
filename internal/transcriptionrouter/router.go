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
	defaultAct  types.PluginAction
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
	target := strings.ToLower(actionName)
	for _, a := range r.actions {
		if strings.Contains(strings.ToLower(a.GetMetadata().Name), target) {
			return a
		}
	}
	return nil
}

func (r *Router) Route(transcription string) error {
	logger.Debugf("Routing transcription: %s", sanitizeLog(transcription))

	// Run first actions that has LLMAction set to false
	for _, a := range r.actions {
		if a.GetMetadata().LLMCommands == nil || len(*a.GetMetadata().LLMCommands) == 0 {
			logger.Debugf("Executing action: %s", a.GetMetadata().Name)

			if err := a.Execute(transcription); err != nil {
				logger.Errorf("Action[%s]: failed to execute: %v", err, a.GetMetadata().Name)
			}
		}
	}

	llmResp, err := r.analyzeWithLLM(transcription)
	if err != nil {
		logger.Error("LLM analysis failed, using default action", err)
		return r.defaultAct.Execute(transcription)
	}

	if action := r.findAction(llmResp.Action); action != nil {
		return action.Execute(llmResp.TranscriptionWithoutCommand)
	}
	return r.defaultAct.Execute(transcription)
}

func (r *Router) analyzeWithLLM(transcription string) (*llmResponse, error) {
	actionsDoc := strings.Builder{}
	for _, a := range r.actions {
		if meta := a.GetMetadata(); meta.LLMCommands != nil && len(*meta.LLMCommands) > 0 {
			actionsDoc.WriteString(fmt.Sprintf("- %s: %s (commands: %s)\n",
				meta.Name, meta.Description, strings.Join(*meta.LLMCommands, "|")))
		}
	}

	prompt := fmt.Sprintf(string(r.promptCache), actionsDoc.String(), transcription)
	logger.Debugf("LLM request prompt: %s", sanitizeLog(prompt))

	req := llm.CompletionRequest{
		Model:       state.Get().Config.LLM.Router.Model,
		Messages:    []llm.ChatCompletionMessage{{Role: "user", Content: prompt}},
		Temperature: float32(state.Get().Config.LLM.Router.Temperature),
	}

	response, err := r.llmProvider.Completion(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("LLM completion failed: %w", err)
	}

	var llmResp llmResponse
	if err := json.Unmarshal([]byte(response), &llmResp); err != nil {
		logger.Error(fmt.Sprintf("Failed to parse LLM response: %s", sanitizeLog(response)), err)
		return nil, fmt.Errorf("LLM response parsing failed: %w", err)
	}
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
