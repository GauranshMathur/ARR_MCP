# Testing the ARR-MCP Server

This document provides instructions for testing the ARR-MCP server.

## Unit Tests

Run the unit tests with:

```bash
go test ./...
```

## Manual Testing

### Starting the Server

1. Build the server:
```bash
go build -o arr-mcp ./cmd/server
```

2. Start the server with test configuration:
```bash
./arr-mcp --host localhost --port 8080 --log-level debug
```

### Testing Endpoints

#### Health Check

```bash
curl http://localhost:8080/health
```

Expected response:
```json
{"status":"ok"}
```

#### Service Health Check

```bash
curl http://localhost:8080/v1/service-health
```

Expected response (if no services are configured):
```json
{"services":{},"status":"ok"}
```

#### List Tools

```bash
curl http://localhost:8080/v1/tools
```

Expected response (if no services are configured):
```json
{"tools":[]}
```

#### Test Tool Execution

If Sonarr is configured:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"tool_name":"SonarrSearch","input":{"query":"Breaking Bad"}}' http://localhost:8080/v1/run
```

If Radarr is configured:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"tool_name":"RadarrSearch","input":{"query":"Inception"}}' http://localhost:8080/v1/run
```

If Prowlarr is configured:

```bash
curl -X POST -H "Content-Type: application/json" -d '{"tool_name":"ProwlarrSearch","input":{"query":"ubuntu","categories":[5030,5040]}}' http://localhost:8080/v1/run
```

## Testing with Claude Code

To use this MCP server with Claude Code:

```bash
claude mcp add ARRStack http://localhost:8080
```

Then in a Claude Code session, you can use tools like:

```
/mcp ARRStack.SonarrSearch query:"Breaking Bad"
```

or

```
/mcp ARRStack.RadarrSearch query:"Inception"
```

or 

```
/mcp ARRStack.ProwlarrSearch query:"ubuntu" categories:[5030, 5040]
```