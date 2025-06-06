package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arr-mcp/pkg/api"
	"arr-mcp/pkg/arr"
	"arr-mcp/pkg/logger"
	"arr-mcp/pkg/server"
)

// Config holds the server configuration
type Config struct {
	Port           int    `json:"port"`
	Host           string `json:"host"`
	LogLevel       string `json:"logLevel"`
	SonarrURL      string `json:"sonarrUrl"`
	SonarrAPIKey   string `json:"sonarrApiKey"`
	RadarrURL      string `json:"radarrUrl"`
	RadarrAPIKey   string `json:"radarrApiKey"`
	ProwlarrURL    string `json:"prowlarrUrl"`
	ProwlarrAPIKey string `json:"prowlarrApiKey"`
}

// loadConfig loads configuration from command line flags and environment variables
func loadConfig() Config {
	config := Config{
		Port:           8080,
		Host:           "localhost",
		LogLevel:       "info",
		SonarrURL:      os.Getenv("SONARR_URL"),
		SonarrAPIKey:   os.Getenv("SONARR_API_KEY"),
		RadarrURL:      os.Getenv("RADARR_URL"),
		RadarrAPIKey:   os.Getenv("RADARR_API_KEY"),
		ProwlarrURL:    os.Getenv("PROWLARR_URL"),
		ProwlarrAPIKey: os.Getenv("PROWLARR_API_KEY"),
	}

	// Define command line flags
	flag.IntVar(&config.Port, "port", config.Port, "Port to listen on")
	flag.StringVar(&config.Host, "host", config.Host, "Host to listen on")
	flag.StringVar(&config.LogLevel, "log-level", config.LogLevel, "Log level (debug, info, warn, error)")
	flag.StringVar(&config.SonarrURL, "sonarr-url", config.SonarrURL, "Sonarr API URL")
	flag.StringVar(&config.SonarrAPIKey, "sonarr-api-key", config.SonarrAPIKey, "Sonarr API Key")
	flag.StringVar(&config.RadarrURL, "radarr-url", config.RadarrURL, "Radarr API URL")
	flag.StringVar(&config.RadarrAPIKey, "radarr-api-key", config.RadarrAPIKey, "Radarr API Key")
	flag.StringVar(&config.ProwlarrURL, "prowlarr-url", config.ProwlarrURL, "Prowlarr API URL")
	flag.StringVar(&config.ProwlarrAPIKey, "prowlarr-api-key", config.ProwlarrAPIKey, "Prowlarr API Key")
	flag.Parse()
	
	return config
}

// validateConfig validates the configuration and returns an error if invalid
func validateConfig(config Config) error {
	// Validate host
	if config.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Validate port
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	// Validate log level
	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[config.LogLevel] {
		return fmt.Errorf("log level must be one of: debug, info, warn, error")
	}

	// At least one service should be configured
	if (config.SonarrURL == "" || config.SonarrAPIKey == "") &&
		(config.RadarrURL == "" || config.RadarrAPIKey == "") &&
		(config.ProwlarrURL == "" || config.ProwlarrAPIKey == "") {
		return fmt.Errorf("at least one service (Sonarr, Radarr, or Prowlarr) must be configured")
	}

	return nil
}

// getClients creates clients for each ARR application
func getClients(config Config) (sonarr *arr.SonarrClient, radarr *arr.RadarrClient, prowlarr *arr.ProwlarrClient) {
	if config.SonarrURL != "" && config.SonarrAPIKey != "" {
		sonarr = arr.NewSonarrClient(config.SonarrURL, config.SonarrAPIKey)
	}
	
	if config.RadarrURL != "" && config.RadarrAPIKey != "" {
		radarr = arr.NewRadarrClient(config.RadarrURL, config.RadarrAPIKey)
	}
	
	if config.ProwlarrURL != "" && config.ProwlarrAPIKey != "" {
		prowlarr = arr.NewProwlarrClient(config.ProwlarrURL, config.ProwlarrAPIKey)
	}
	
	return
}

// setupServer configures the MCP server with the available tools
func setupServer(sonarrClient *arr.SonarrClient, radarrClient *arr.RadarrClient, prowlarrClient *arr.ProwlarrClient) *server.MCPServer {
	mcpServer := server.NewMCPServer()
	
	// Register Sonarr tools if client is available
	if sonarrClient != nil {
		// Register service health checker
		mcpServer.RegisterServiceChecker(sonarrClient)
		
		// Sonarr Search Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "SonarrSearch",
				Description: "Search for TV shows in Sonarr",
				Parameters: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query for TV shows",
						"required":    true,
					},
				},
			},
			&arr.SonarrSearchHandler{Client: sonarrClient},
		)
		
		// Sonarr List Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "SonarrList",
				Description: "List TV shows in Sonarr",
			},
			&arr.SonarrListHandler{Client: sonarrClient},
		)

		// Sonarr Add Series Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "SonarrAddSeries",
				Description: "Add a new TV series to Sonarr",
				Parameters: map[string]interface{}{
					"seriesData": map[string]interface{}{
						"type":        "object",
						"description": "The TV series data to add (requires tvdbId, title, qualityProfileId, rootFolderPath)",
						"required":    true,
					},
				},
			},
			&arr.SonarrAddSeriesHandler{Client: sonarrClient},
		)

		// Sonarr Get Quality Profiles Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "SonarrGetProfiles",
				Description: "Get quality profiles from Sonarr",
			},
			&arr.SonarrGetProfilesHandler{Client: sonarrClient},
		)

		// Sonarr Get Root Folders Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "SonarrGetRootFolders",
				Description: "Get root folders from Sonarr",
			},
			&arr.SonarrGetRootFoldersHandler{Client: sonarrClient},
		)
	}
	
	// Register Radarr tools if client is available
	if radarrClient != nil {
		// Register service health checker
		mcpServer.RegisterServiceChecker(radarrClient)
		
		// Radarr Search Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "RadarrSearch",
				Description: "Search for movies in Radarr",
				Parameters: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query for movies",
						"required":    true,
					},
				},
			},
			&arr.RadarrSearchHandler{Client: radarrClient},
		)
		
		// Radarr List Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "RadarrList",
				Description: "List movies in Radarr",
			},
			&arr.RadarrListHandler{Client: radarrClient},
		)

		// Radarr Add Movie Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "RadarrAddMovie",
				Description: "Add a new movie to Radarr",
				Parameters: map[string]interface{}{
					"movieData": map[string]interface{}{
						"type":        "object",
						"description": "The movie data to add (requires tmdbId, title, qualityProfileId, rootFolderPath)",
						"required":    true,
					},
				},
			},
			&arr.RadarrAddMovieHandler{Client: radarrClient},
		)

		// Radarr Get Quality Profiles Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "RadarrGetProfiles",
				Description: "Get quality profiles from Radarr",
			},
			&arr.RadarrGetProfilesHandler{Client: radarrClient},
		)

		// Radarr Get Root Folders Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "RadarrGetRootFolders",
				Description: "Get root folders from Radarr",
			},
			&arr.RadarrGetRootFoldersHandler{Client: radarrClient},
		)
	}
	
	// Register Prowlarr tools if client is available
	if prowlarrClient != nil {
		// Register service health checker
		mcpServer.RegisterServiceChecker(prowlarrClient)
		
		// Prowlarr Search Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "ProwlarrSearch",
				Description: "Search for content using Prowlarr indexers",
				Parameters: map[string]interface{}{
					"query": map[string]interface{}{
						"type":        "string",
						"description": "The search query for content",
						"required":    true,
					},
					"categories": map[string]interface{}{
						"type":        "array",
						"description": "Optional category IDs to filter results",
						"items": map[string]interface{}{
							"type": "integer",
						},
					},
				},
			},
			&arr.ProwlarrSearchHandler{Client: prowlarrClient},
		)
		
		// Prowlarr Indexers Tool
		mcpServer.RegisterTool(
			api.ToolDefinition{
				Name:        "ProwlarrIndexers",
				Description: "List Prowlarr indexers",
			},
			&arr.ProwlarrIndexersHandler{Client: prowlarrClient},
		)
	}
	
	return mcpServer
}

func main() {
	// Initialize logger
	log := logger.New("info", "Main")
	
	// Load configuration
	config := loadConfig()
	
	// Set global log level
	logger.SetDefaultLevel(config.LogLevel)
	log.SetLevel(config.LogLevel)
	
	// Validate configuration
	if err := validateConfig(config); err != nil {
		log.Error("Configuration error: %v", err)
		os.Exit(1)
	}
	
	// Create clients
	log.Info("Initializing ARR clients...")
	sonarrClient, radarrClient, prowlarrClient := getClients(config)
	
	// Set up MCP server
	log.Info("Setting up MCP server...")
	mcpServer := setupServer(sonarrClient, radarrClient, prowlarrClient)
	mcpServer.SetLogLevel(config.LogLevel)
	
	// Print server info
	printServerInfo(config, sonarrClient, radarrClient, prowlarrClient)
	
	// Set up signal handling for graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Start server in a goroutine
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	log.Info("Starting ARR MCP server on %s", addr)
	
	go func() {
		if err := mcpServer.Start(addr); err != nil {
			log.Error("Server error: %v", err)
			stop <- syscall.SIGTERM
		}
	}()
	
	// Wait for interrupt signal
	<-stop
	log.Info("Shutdown signal received")
	
	// Create a deadline for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	
	// Initiate graceful shutdown
	if err := mcpServer.Shutdown(ctx); err != nil {
		log.Error("Server shutdown error: %v", err)
	}
	
	log.Info("Server shutdown complete")
}

// printServerInfo prints server information
func printServerInfo(config Config, sonarrClient *arr.SonarrClient, radarrClient *arr.RadarrClient, prowlarrClient *arr.ProwlarrClient) {
	fmt.Println("ARR MCP Server")
	fmt.Println("==============")
	fmt.Printf("Host: %s\n", config.Host)
	fmt.Printf("Port: %d\n", config.Port)
	fmt.Printf("Log Level: %s\n", config.LogLevel)
	fmt.Println("\nAvailable Services:")
	
	if sonarrClient != nil {
		fmt.Println("- Sonarr: Connected")
	} else {
		fmt.Println("- Sonarr: Not configured")
	}
	
	if radarrClient != nil {
		fmt.Println("- Radarr: Connected")
	} else {
		fmt.Println("- Radarr: Not configured")
	}
	
	if prowlarrClient != nil {
		fmt.Println("- Prowlarr: Connected")
	} else {
		fmt.Println("- Prowlarr: Not configured")
	}
	
	fmt.Println("\nAvailable Endpoints:")
	fmt.Println("- /health: Server health check endpoint")
	fmt.Println("- /v1/service-health: Services health check endpoint")
	fmt.Println("- /v1/run: MCP run endpoint")
	fmt.Println("- /v1/tools: List available tools")
	fmt.Println("\nServer URL for MCP clients:")
	fmt.Printf("http://%s:%d\n", config.Host, config.Port)
}