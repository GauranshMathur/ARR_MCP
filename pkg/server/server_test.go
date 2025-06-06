package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arr-mcp/pkg/api"
)

// MockHandler is a simple mock implementation of api.Handler for testing
type MockHandler struct {
	Response interface{}
	Error    error
}

func (h *MockHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
	return h.Response, h.Error
}

// MockServiceChecker is a simple mock implementation of ServiceChecker for testing
type MockServiceChecker struct {
	ServiceName string
	HealthError error
}

func (c *MockServiceChecker) Name() string {
	return c.ServiceName
}

func (c *MockServiceChecker) Check() error {
	return c.HealthError
}

func TestHandleHealth(t *testing.T) {
	server := NewMCPServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.HandleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if status, ok := response["status"]; !ok || status != "ok" {
		t.Errorf("Expected status to be 'ok', got '%s'", status)
	}
}

func TestHandleListTools(t *testing.T) {
	server := NewMCPServer()
	
	// Register a test tool
	server.RegisterTool(
		api.ToolDefinition{
			Name:        "TestTool",
			Description: "A test tool",
			Parameters: map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "A test parameter",
				},
			},
		},
		&MockHandler{Response: map[string]string{"result": "success"}, Error: nil},
	)

	req := httptest.NewRequest(http.MethodGet, "/v1/tools", nil)
	w := httptest.NewRecorder()

	server.HandleListTools(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response map[string][]api.ToolDefinition
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if tools, ok := response["tools"]; !ok || len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	} else if tools[0].Name != "TestTool" {
		t.Errorf("Expected tool name 'TestTool', got '%s'", tools[0].Name)
	}
}

func TestHandleRun(t *testing.T) {
	server := NewMCPServer()
	
	// Register a test tool
	server.RegisterTool(
		api.ToolDefinition{
			Name:        "TestTool",
			Description: "A test tool",
			Parameters: map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "A test parameter",
				},
			},
		},
		&MockHandler{Response: map[string]string{"result": "success"}, Error: nil},
	)

	// Create a test request
	mcpRequest := api.MCPRequest{
		ToolName: "TestTool",
		Input: map[string]interface{}{
			"param1": "test value",
		},
	}
	
	requestBody, _ := json.Marshal(mcpRequest)
	req := httptest.NewRequest(http.MethodPost, "/v1/run", bytes.NewBuffer(requestBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.HandleRun(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, w.Code)
	}

	var response api.MCPFinalResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if response.Type != "final" {
		t.Errorf("Expected response type 'final', got '%s'", response.Type)
	}
}

func TestHandleServiceHealth(t *testing.T) {
	server := NewMCPServer()
	
	// Register a healthy service
	server.RegisterServiceChecker(&MockServiceChecker{
		ServiceName: "HealthyService",
		HealthError: nil,
	})
	
	// Register an unhealthy service
	server.RegisterServiceChecker(&MockServiceChecker{
		ServiceName: "UnhealthyService",
		HealthError: fmt.Errorf("service is down"),
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/service-health", nil)
	w := httptest.NewRecorder()

	server.HandleServiceHealth(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status code %d, got %d", http.StatusServiceUnavailable, w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to unmarshal response: %v", err)
	}

	if status, ok := response["status"]; !ok || status != "degraded" {
		t.Errorf("Expected status to be 'degraded', got '%s'", status)
	}
	
	if services, ok := response["services"].(map[string]interface{}); !ok {
		t.Errorf("Expected services map in response")
	} else {
		if healthyStatus, ok := services["HealthyService"].(string); !ok || healthyStatus != "healthy" {
			t.Errorf("Expected HealthyService to be 'healthy', got '%s'", healthyStatus)
		}
		
		if unhealthyStatus, ok := services["UnhealthyService"].(string); !ok || unhealthyStatus != "unhealthy: service is down" {
			t.Errorf("Expected UnhealthyService to report error, got '%s'", unhealthyStatus)
		}
	}
}