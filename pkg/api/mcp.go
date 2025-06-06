package api

// MCPRequest represents the request structure for MCP API
type MCPRequest struct {
	Input       map[string]interface{} `json:"input"`
	ToolName    string                 `json:"tool_name"`
	RequestID   string                 `json:"request_id"`
	Timeout     int                    `json:"timeout,omitempty"`
	AccessToken string                 `json:"access_token,omitempty"`
}

// MCPResponse represents the response structure for standard MCP API
type MCPResponse struct {
	Type   string      `json:"type"` // "final" or "partial"
	Result interface{} `json:"result"`
}

// MCPFinalResponse represents a final response
type MCPFinalResponse struct {
	Type   string      `json:"type"` // Always "final"
	Result interface{} `json:"result"`
}

// MCPPartialResponse represents a partial (streaming) response
type MCPPartialResponse struct {
	Type    string      `json:"type"` // Always "partial"
	Content interface{} `json:"content"`
	Done    bool        `json:"done"`
}

// MCPErrorResponse represents an error response
type MCPErrorResponse struct {
	Type  string `json:"type"` // Always "error"
	Error struct {
		Message string `json:"message"`
		Code    string `json:"code,omitempty"`
	} `json:"error"`
}

// ToolDefinition represents a tool that can be used through the MCP
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

// ToolRegistry is an interface for registering and retrieving tools
type ToolRegistry interface {
	RegisterTool(tool ToolDefinition)
	GetTool(name string) (ToolDefinition, bool)
	ListTools() []ToolDefinition
}

// Handler is an interface for handling tool requests
type Handler interface {
	HandleRequest(request MCPRequest) (interface{}, error)
}

// BasicToolRegistry is a simple implementation of ToolRegistry
type BasicToolRegistry struct {
	tools map[string]ToolDefinition
}

// NewBasicToolRegistry creates a new registry
func NewBasicToolRegistry() *BasicToolRegistry {
	return &BasicToolRegistry{
		tools: make(map[string]ToolDefinition),
	}
}

// RegisterTool registers a tool in the registry
func (r *BasicToolRegistry) RegisterTool(tool ToolDefinition) {
	r.tools[tool.Name] = tool
}

// GetTool retrieves a tool from the registry
func (r *BasicToolRegistry) GetTool(name string) (ToolDefinition, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// ListTools returns all registered tools
func (r *BasicToolRegistry) ListTools() []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}