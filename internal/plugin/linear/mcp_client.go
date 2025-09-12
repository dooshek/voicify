package linear

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/dooshek/voicify/internal/logger"
)

// LinearMCPClient handles communication with Linear MCP server via npx mcp-remote
type LinearMCPClient struct {
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	mu          sync.Mutex
	cachedTools []MCPTool
	toolsCached bool
	pidFile     string
}

// MCPTool represents a tool available through MCP
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPRequest represents a request to MCP server
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a response from MCP server
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ToolCall represents a call to an MCP tool
type ToolCall struct {
	Name       string                 `json:"name"`
	Parameters map[string]interface{} `json:"parameters"`
}


// NewLinearMCPClient creates a new Linear MCP client
func NewLinearMCPClient() (*LinearMCPClient, error) {
	// Setup PID file path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	pidFile := filepath.Join(homeDir, ".config", "voicify", "mcp-remote.pid")

	// Ensure config directory exists
	configDir := filepath.Dir(pidFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	client := &LinearMCPClient{
		mu:          sync.Mutex{},
		cachedTools: nil,
		toolsCached: false,
		pidFile:     pidFile,
	}

	// Check if existing process is running
	if client.isProcessRunning() {
		logger.Debug("Found existing mcp-remote process, attempting to reconnect...")
		// Try to reconnect to existing process - for now just start new one
		client.cleanupOldProcess()
	}

	// Start new process
	if err := client.startProcess(); err != nil {
		return nil, fmt.Errorf("failed to start mcp-remote: %w", err)
	}

	return client, nil
}

// SetupLinearMCP is no longer needed - npx mcp-remote handles authorization
func SetupLinearMCP() error {
	logger.Info("Linear MCP uses npx mcp-remote - no setup required")
	return nil
}

// isProcessRunning checks if mcp-remote process is running based on PID file
func (c *LinearMCPClient) isProcessRunning() bool {
	pidBytes, err := os.ReadFile(c.pidFile)
	if err != nil {
		return false
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return false
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// Send signal 0 to check if process is alive
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// cleanupOldProcess kills old mcp-remote process and removes PID file
func (c *LinearMCPClient) cleanupOldProcess() {
	pidBytes, err := os.ReadFile(c.pidFile)
	if err != nil {
		return
	}

	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		return
	}

	// Kill the process
	if process, err := os.FindProcess(pid); err == nil {
		logger.Debugf("Killing old mcp-remote process with PID %d", pid)
		process.Kill()
	}

	// Remove PID file
	os.Remove(c.pidFile)
}

// startProcess starts new mcp-remote process and saves PID
func (c *LinearMCPClient) startProcess() error {
	// Start npx mcp-remote as a persistent process
	cmd := exec.Command("npx", "-y", "mcp-remote", "https://mcp.linear.app/sse")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mcp-remote: %w", err)
	}

	// Save PID to file
	pid := cmd.Process.Pid
	pidStr := strconv.Itoa(pid)
	if err := os.WriteFile(c.pidFile, []byte(pidStr), 0644); err != nil {
		logger.Warnf("Failed to write PID file: %v", err)
	}

	// Update client
	c.cmd = cmd
	c.stdin = stdin
	c.stdout = stdout

	logger.Debugf("Started npx mcp-remote process with PID %d", pid)

	// Wait for process to initialize
	logger.Debug("Waiting for mcp-remote to initialize...")
	time.Sleep(5 * time.Second)

	return nil
}

// GetAvailableTools retrieves all available MCP tools from Linear MCP server
func (c *LinearMCPClient) GetAvailableTools() ([]MCPTool, error) {
	// Return cached tools if available
	if c.toolsCached {
		logger.Debug("Returning cached MCP tools")
		return c.cachedTools, nil
	}

	logger.Debug("Getting available MCP tools from Linear via npx mcp-remote")

	// Call npx mcp-remote to list tools
	result, err := c.callMCPRemote("tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Parse MCP response
	var mcpResponse MCPResponse
	if err := json.Unmarshal(result, &mcpResponse); err != nil {
		return nil, fmt.Errorf("failed to parse MCP response: %w", err)
	}

	// Extract tools from result
	var tools []MCPTool
	if resultMap, ok := mcpResponse.Result.(map[string]interface{}); ok {
		if toolsArray, ok := resultMap["tools"].([]interface{}); ok {
			for _, toolInterface := range toolsArray {
				if toolMap, ok := toolInterface.(map[string]interface{}); ok {
					tool := MCPTool{
						Name:        toolMap["name"].(string),
						Description: toolMap["description"].(string),
						InputSchema: toolMap["inputSchema"].(map[string]interface{}),
					}
					tools = append(tools, tool)
				}
			}
		}
	}

	// Cache the tools
	c.cachedTools = tools
	c.toolsCached = true

	logger.Debugf("Found and cached %d available MCP tools", len(tools))
	return tools, nil
}

// ExecuteTool executes a specific MCP tool with given parameters
func (c *LinearMCPClient) ExecuteTool(toolName string, parameters map[string]interface{}) (string, error) {
	logger.Debugf("Executing MCP tool: %s with parameters: %+v", toolName, parameters)

	// Prepare tool call parameters
	toolParams := map[string]interface{}{
		"name":      toolName,
		"arguments": parameters,
	}

	// Call npx mcp-remote to execute tool
	result, err := c.callMCPRemote("tools/call", toolParams)
	if err != nil {
		return "", fmt.Errorf("failed to call tool: %w", err)
	}

	// Check for MCP errors in response
	var mcpResponse MCPResponse
	if err := json.Unmarshal(result, &mcpResponse); err == nil {
		if mcpResponse.Error != nil {
			return "", fmt.Errorf("MCP tool error: %s", mcpResponse.Error.Message)
		}
	}

	resultStr := string(result)

	logger.Debugf("MCP tool %s executed successfully", toolName)
	return resultStr, nil
}

// callMCPRemote calls npx mcp-remote with the given method and params via STDIO
func (c *LinearMCPClient) callMCPRemote(method string, params interface{}) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Try up to 2 times with process restart
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			logger.Debugf("Retrying MCP call, attempt %d", attempt+1)
		}

		// Ensure process is running
		if err := c.ensureProcessRunning(); err != nil {
			return nil, fmt.Errorf("failed to ensure process running: %w", err)
		}

		result, err := c.doMCPCall(method, params)
		if err != nil {
			logger.Debugf("MCP call failed on attempt %d: %v", attempt+1, err)
			if attempt == 0 {
				// Force restart on first failure
				c.forceRestart()
				continue
			}
			return nil, err
		}

		return result, nil
	}

	return nil, fmt.Errorf("MCP call failed after 2 attempts")
}

// forceRestart forces a restart of the mcp-remote process
func (c *LinearMCPClient) forceRestart() {
	logger.Debug("Forcing mcp-remote process restart...")
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}

	// Remove PID file
	if c.pidFile != "" {
		os.Remove(c.pidFile)
	}

	c.cmd = nil
	c.stdin = nil
	c.stdout = nil
}

// doMCPCall performs the actual MCP call
func (c *LinearMCPClient) doMCPCall(method string, params interface{}) ([]byte, error) {
	// Prepare MCP request
	request := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  method,
	}

	if params != nil {
		request.Params = params.(map[string]interface{})
	}

	// Convert request to JSON
	requestBytes, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	logger.Debugf("Sending MCP request: %s", string(requestBytes))

	// Send request to stdin
	if _, err := c.stdin.Write(append(requestBytes, '\n')); err != nil {
		return nil, fmt.Errorf("failed to write to stdin: %w", err)
	}

	// Read response from stdout with timeout
	responseCh := make(chan []byte, 1)
	errorCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(c.stdout)
		if scanner.Scan() {
			responseCh <- scanner.Bytes()
		} else {
			if err := scanner.Err(); err != nil {
				errorCh <- fmt.Errorf("failed to read from stdout: %w", err)
			} else {
				errorCh <- fmt.Errorf("no response from mcp-remote")
			}
		}
	}()

	// Wait for response with timeout
	select {
	case response := <-responseCh:
		logger.Debugf("Received MCP response: %s", string(response))
		return response, nil
	case err := <-errorCh:
		return nil, err
	case <-time.After(15 * time.Second): // Increased timeout
		return nil, fmt.Errorf("mcp-remote response timeout")
	}
}

// ensureProcessRunning checks if mcp-remote process is running and restarts if needed
func (c *LinearMCPClient) ensureProcessRunning() error {
	// Check if process is running via PID file
	if !c.isProcessRunning() {
		logger.Debug("mcp-remote process not running, starting new one...")

		// Cleanup old process if any
		c.cleanupOldProcess()

		// Start new process
		if err := c.startProcess(); err != nil {
			return fmt.Errorf("failed to start mcp-remote: %w", err)
		}
	} else {
		logger.Debug("mcp-remote process is running")
	}

	return nil
}

// Close closes the MCP client and terminates the npx process
func (c *LinearMCPClient) Close() error {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.stdout != nil {
		c.stdout.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}

	// Remove PID file
	if c.pidFile != "" {
		os.Remove(c.pidFile)
	}

	return nil
}

