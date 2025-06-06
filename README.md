# ARR-MCP

ARR-MCP is a standard MCP (Model Control Protocol) server designed to integrate with the ARR stack (Sonarr, Radarr, Prowlarr).

## Overview

This MCP server provides a bridge between MCP-compatible clients and popular media management tools in the ARR stack. It implements the standard MCP protocol, enabling any MCP client to interact with these tools through a standardized interface.

## Features

- **Flexible MCP Server**: Implements the standard MCP protocol to allow any compatible client to connect
- **ARR Stack Integration**:
  - **Sonarr**: Search and manage TV shows
  - **Radarr**: Search and manage movies
  - **Prowlarr**: Search across indexers and manage indexers
- **Extensible Architecture**: Easily add new tools and handlers

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/arr-mcp.git
cd arr-mcp
```

2. Build the server:
```bash
go build -o arr-mcp ./cmd/server
```

## Usage

### Running the Server

```bash
./arr-mcp
```

By default, the server runs on `localhost:8080`. You can configure it using command-line flags:

```bash
./arr-mcp --port 9000 --host 0.0.0.0
```

### Environment Variables

Configure the connections to various services using environment variables:

```bash
export SONARR_URL="http://localhost:8989"
export SONARR_API_KEY="your-sonarr-api-key"
export RADARR_URL="http://localhost:7878"
export RADARR_API_KEY="your-radarr-api-key"
export PROWLARR_URL="http://localhost:9696"
export PROWLARR_API_KEY="your-prowlarr-api-key"
```

### Command-line Options

- `--port`: Server port (default: 8080)
- `--host`: Host to bind to (default: localhost)
- `--log-level`: Logging level (default: info)
- `--sonarr-url`: Sonarr API URL
- `--sonarr-api-key`: Sonarr API key
- `--radarr-url`: Radarr API URL
- `--radarr-api-key`: Radarr API key
- `--prowlarr-url`: Prowlarr API URL
- `--prowlarr-api-key`: Prowlarr API key

## Configuring MCP Clients to Use This Server

To use this MCP server with any MCP-compatible client, configure the client to connect to:

```
http://localhost:8080
```

The server implements the standard MCP protocol endpoints required by compliant clients.

## Available Endpoints

- **GET /health**: Health check endpoint
- **POST /v1/run**: MCP run endpoint for executing tools
- **GET /v1/tools**: List available tools

## Available Tools

Depending on your configuration, the following tools will be available:

- **SonarrSearch**: Search for TV shows in Sonarr
- **SonarrList**: List TV shows in Sonarr
- **RadarrSearch**: Search for movies in Radarr
- **RadarrList**: List movies in Radarr
- **ProwlarrSearch**: Search for content using Prowlarr indexers
- **ProwlarrIndexers**: List Prowlarr indexers

## Tool Usage Examples

### SonarrSearch

```json
{
  "tool_name": "SonarrSearch",
  "input": {
    "query": "Breaking Bad"
  }
}
```

### RadarrSearch

```json
{
  "tool_name": "RadarrSearch",
  "input": {
    "query": "Inception"
  }
}
```

### ProwlarrSearch

```json
{
  "tool_name": "ProwlarrSearch",
  "input": {
    "query": "ubuntu 22.04",
    "categories": [5030, 5040]
  }
}
```

## Development

### Project Structure

- `cmd/server/`: Contains the main application entry point
- `pkg/api/`: MCP protocol definitions and interfaces
- `pkg/arr/`: ARR stack integration (Sonarr, Radarr, Prowlarr)
- `pkg/server/`: MCP server implementation

### Adding New Tools

To add a new tool to the server:

1. Create a handler that implements the `api.Handler` interface
2. Register the tool and handler in the `setupServer` function in `main.go`

Example:

```go
// Create a new handler
type MyNewHandler struct {
    // Dependencies here
}

// Implement the HandleRequest method
func (h *MyNewHandler) HandleRequest(req api.MCPRequest) (interface{}, error) {
    // Handle the request
    return result, nil
}

// In setupServer function
mcpServer.RegisterTool(
    api.ToolDefinition{
        Name:        "MyNewTool",
        Description: "Description of my new tool",
        Parameters: map[string]interface{}{
            "param1": map[string]interface{}{
                "type":        "string",
                "description": "Description of parameter",
            },
        },
    },
    &MyNewHandler{},
)
```

### Building from Source

```bash
go mod tidy
go build -o arr-mcp ./cmd/server
```

## License

[MIT License](LICENSE)