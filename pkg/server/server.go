package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"arr-mcp/pkg/api"
	"arr-mcp/pkg/logger"
)

// ServiceChecker is an interface for checking service health
type ServiceChecker interface {
	// Check returns nil if the service is healthy, otherwise returns an error
	Check() error
	// Name returns the service name
	Name() string
}

// MCPServer represents an MCP-compatible server
type MCPServer struct {
	registry       api.ToolRegistry
	handlers       map[string]api.Handler
	handlersLock   sync.RWMutex
	serviceCheckers []ServiceChecker
	server         *http.Server
	log            *logger.Logger
}

// NewMCPServer creates a new MCP server
func NewMCPServer() *MCPServer {
	return &MCPServer{
		registry: api.NewBasicToolRegistry(),
		handlers: make(map[string]api.Handler),
		serviceCheckers: make([]ServiceChecker, 0),
		log:      logger.New("info", "MCPServer"),
	}
}

// RegisterTool registers a tool and its handler
func (s *MCPServer) RegisterTool(definition api.ToolDefinition, handler api.Handler) {
	s.handlersLock.Lock()
	defer s.handlersLock.Unlock()

	s.registry.RegisterTool(definition)
	s.handlers[definition.Name] = handler
	s.log.Info("Registered tool: %s", definition.Name)
}

// RegisterServiceChecker registers a service health checker
func (s *MCPServer) RegisterServiceChecker(checker ServiceChecker) {
	s.serviceCheckers = append(s.serviceCheckers, checker)
	s.log.Info("Registered health checker for service: %s", checker.Name())
}

// SetLogLevel sets the log level for the server
func (s *MCPServer) SetLogLevel(level string) {
	s.log.SetLevel(level)
	s.log.Info("Log level set to: %s", level)
}

// Start starts the HTTP server on the specified address
func (s *MCPServer) Start(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.HandleHealth)
	mux.HandleFunc("/v1/run", s.HandleRun)
	mux.HandleFunc("/v1/tools", s.HandleListTools)
	mux.HandleFunc("/v1/service-health", s.HandleServiceHealth)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	s.log.Info("Starting HTTP server on %s", addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *MCPServer) Shutdown(ctx context.Context) error {
	s.log.Info("Shutting down server...")
	return s.server.Shutdown(ctx)
}

// HandleRun handles MCP run requests
func (s *MCPServer) HandleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read and parse request
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.log.Error("Failed to read request body: %v", err)
		s.sendErrorResponse(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var mcpRequest api.MCPRequest
	if err := json.Unmarshal(body, &mcpRequest); err != nil {
		s.log.Error("Invalid request format: %v", err)
		s.sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if mcpRequest.ToolName == "" {
		s.sendErrorResponse(w, "Missing tool_name in request", http.StatusBadRequest)
		return
	}

	s.log.Debug("Received request for tool: %s", mcpRequest.ToolName)

	// Process request
	s.handlersLock.RLock()
	handler, exists := s.handlers[mcpRequest.ToolName]
	toolDef, _ := s.registry.GetTool(mcpRequest.ToolName)
	s.handlersLock.RUnlock()

	if !exists {
		s.log.Warn("Unknown tool requested: %s", mcpRequest.ToolName)
		s.sendErrorResponse(w, fmt.Sprintf("Unknown tool: %s", mcpRequest.ToolName), http.StatusBadRequest)
		return
	}

	// Validate parameters against tool definition schema (basic validation)
	if err := s.validateParameters(mcpRequest.Input, toolDef.Parameters); err != nil {
		s.log.Warn("Parameter validation failed for tool %s: %v", mcpRequest.ToolName, err)
		s.sendErrorResponse(w, fmt.Sprintf("Parameter validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// Check if request has a timeout
	ctx := r.Context()
	if mcpRequest.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(mcpRequest.Timeout)*time.Millisecond)
		defer cancel()
	}

	// Handle the request in a goroutine if it supports streaming
	if supportStreamingResponse(handler) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			s.sendErrorResponse(w, "Streaming not supported", http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Transfer-Encoding", "chunked")
		
		// Start processing in a goroutine
		go s.handleStreamingRequest(ctx, w, flusher, mcpRequest, handler)
		return
	}

	// For non-streaming requests, process synchronously
	result, err := handler.HandleRequest(mcpRequest)
	if err != nil {
		s.log.Error("Handler error for tool %s: %v", mcpRequest.ToolName, err)
		s.sendErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.log.Debug("Successfully processed request for tool: %s", mcpRequest.ToolName)
	s.sendSuccessResponse(w, result)
}

// validateParameters validates request parameters against tool schema
func (s *MCPServer) validateParameters(input map[string]interface{}, schema map[string]interface{}) error {
	// For each parameter in the schema
	for paramName, paramSchema := range schema {
		schemaObj, ok := paramSchema.(map[string]interface{})
		if !ok {
			continue // Skip if schema is not an object
		}

		// Check if parameter is required
		required, _ := schemaObj["required"].(bool)
		if required {
			if _, exists := input[paramName]; !exists {
				return fmt.Errorf("required parameter missing: %s", paramName)
			}
		}

		// If parameter exists, validate its type
		if value, exists := input[paramName]; exists {
			expectedType, _ := schemaObj["type"].(string)
			if expectedType != "" {
				// Perform basic type checking
				switch expectedType {
				case "string":
					if _, ok := value.(string); !ok {
						return fmt.Errorf("parameter %s must be a string", paramName)
					}
				case "number":
					if _, ok := value.(float64); !ok {
						return fmt.Errorf("parameter %s must be a number", paramName)
					}
				case "integer":
					// JSON unmarshals numbers as float64
					if num, ok := value.(float64); !ok || float64(int(num)) != num {
						return fmt.Errorf("parameter %s must be an integer", paramName)
					}
				case "boolean":
					if _, ok := value.(bool); !ok {
						return fmt.Errorf("parameter %s must be a boolean", paramName)
					}
				case "array":
					if _, ok := value.([]interface{}); !ok {
						return fmt.Errorf("parameter %s must be an array", paramName)
					}
				case "object":
					if _, ok := value.(map[string]interface{}); !ok {
						return fmt.Errorf("parameter %s must be an object", paramName)
					}
				}
			}
		}
	}
	
	return nil
}

// supportStreamingResponse checks if a handler supports streaming responses
func supportStreamingResponse(handler api.Handler) bool {
	// In a real implementation, you might check if the handler implements a StreamingHandler interface
	// For now, we'll just return false as the basic implementation doesn't support streaming
	return false
}

// handleStreamingRequest handles a streaming request using standard MCP protocol
func (s *MCPServer) handleStreamingRequest(ctx context.Context, w http.ResponseWriter, flusher http.Flusher, req api.MCPRequest, handler api.Handler) {
	// In a real implementation, this would stream partial results
	// For now, we'll just return a final result after processing
	
	result, err := handler.HandleRequest(req)
	if err != nil {
		partialResponse := api.MCPPartialResponse{
			Type:    "partial",
			Content: map[string]interface{}{"error": err.Error()},
			Done:    true,
		}
		json.NewEncoder(w).Encode(partialResponse)
		flusher.Flush()
		return
	}
	
	partialResponse := api.MCPPartialResponse{
		Type:    "partial",
		Content: result,
		Done:    true,
	}
	json.NewEncoder(w).Encode(partialResponse)
	flusher.Flush()
}

// HandleListTools handles requests to list available tools using standard MCP protocol
func (s *MCPServer) HandleListTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tools := s.registry.ListTools()
	s.log.Debug("Returning list of %d tools", len(tools))
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": tools,
	})
}

// HandleHealth handles server health check requests
func (s *MCPServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleServiceHealth checks and reports on the health of connected services
func (s *MCPServer) HandleServiceHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.sendErrorResponse(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	
	results := make(map[string]string)
	allHealthy := true
	
	for _, checker := range s.serviceCheckers {
		serviceName := checker.Name()
		err := checker.Check()
		
		if err != nil {
			results[serviceName] = fmt.Sprintf("unhealthy: %v", err)
			allHealthy = false
		} else {
			results[serviceName] = "healthy"
		}
	}
	
	response := map[string]interface{}{
		"status":   "ok",
		"services": results,
	}
	
	if !allHealthy {
		response["status"] = "degraded"
	}
	
	statusCode := http.StatusOK
	if !allHealthy {
		statusCode = http.StatusServiceUnavailable
	}
	
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

// sendErrorResponse sends an error response in standard MCP format
func (s *MCPServer) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	response := api.MCPErrorResponse{
		Type: "error",
	}
	response.Error.Message = message
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.log.Error("Error encoding error response: %v", err)
	}
}

// sendSuccessResponse sends a success response in standard MCP format
func (s *MCPServer) sendSuccessResponse(w http.ResponseWriter, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	
	response := api.MCPFinalResponse{
		Type:   "final",
		Result: result,
	}
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.log.Error("Error encoding success response: %v", err)
	}
}