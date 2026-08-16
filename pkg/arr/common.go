package arr

import "context"

// QualityProfile is a quality profile as needed when adding media.
type QualityProfile struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// RootFolder is a library root path as needed when adding media.
type RootFolder struct {
	ID         int    `json:"id"`
	Path       string `json:"path"`
	FreeSpace  int64  `json:"freeSpace,omitempty" jsonschema:"free space in bytes"`
	Accessible bool   `json:"accessible"`
}

// SystemStatus is the trimmed health/version view of a service instance.
type SystemStatus struct {
	Version      string `json:"version,omitempty"`
	AppName      string `json:"appName,omitempty"`
	InstanceName string `json:"instanceName,omitempty"`
}

// ListQualityProfiles returns the quality profiles configured on an instance.
// Sonarr and Radarr share this endpoint shape.
func ListQualityProfiles(ctx context.Context, c *Client) ([]QualityProfile, error) {
	return GetJSON[[]QualityProfile](ctx, c, "/qualityprofile")
}

// ListRootFolders returns the library root folders configured on an instance.
func ListRootFolders(ctx context.Context, c *Client) ([]RootFolder, error) {
	return GetJSON[[]RootFolder](ctx, c, "/rootfolder")
}

// GetSystemStatus returns version and identity information for an instance.
func GetSystemStatus(ctx context.Context, c *Client) (SystemStatus, error) {
	return GetJSON[SystemStatus](ctx, c, "/system/status")
}
