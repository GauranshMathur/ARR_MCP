package server

import "github.com/GauranshMathur/ARR_MCP/pkg/arr"

// --- synology tool inputs ---

// SynologyPageArgs pages synology_list_shares.
type SynologyPageArgs struct {
	InstanceArg
	Limit  int `json:"limit,omitempty" jsonschema:"shares per page; defaults to 25, at most 100"`
	Offset int `json:"offset,omitempty" jsonschema:"shares to skip, for paging; defaults to 0"`
}

// SynologyListFilesArgs selects and pages one folder.
type SynologyListFilesArgs struct {
	InstanceArg
	Path   string `json:"path" jsonschema:"absolute File Station path starting with a share, e.g. /NAS/Movies; see synology_list_shares. No .. segments"`
	Limit  int    `json:"limit,omitempty" jsonschema:"entries per page; defaults to 25, at most 100"`
	Offset int    `json:"offset,omitempty" jsonschema:"entries to skip, for paging; defaults to 0"`
	Sort   string `json:"sort,omitempty" jsonschema:"name (default), size, user, group, mtime, atime, ctime, crtime, posix or type"`
	Desc   *bool  `json:"desc,omitempty" jsonschema:"largest or newest first; defaults to false"`
	Type   string `json:"type,omitempty" jsonschema:"all (default), file or dir"`
}

// SynologyPathArgs names one file or folder.
type SynologyPathArgs struct {
	InstanceArg
	Path string `json:"path" jsonschema:"absolute File Station path starting with a share, e.g. /NAS/Movies; see synology_list_shares. No .. segments"`
}

// SynologySearchArgs describes a name search below one folder.
type SynologySearchArgs struct {
	InstanceArg
	Path      string `json:"path" jsonschema:"folder to search below: absolute, starting with a share, e.g. /NAS. No .. segments"`
	Pattern   string `json:"pattern" jsonschema:"text to look for in file and folder names; case-insensitive, matched anywhere in the name"`
	Type      string `json:"type,omitempty" jsonschema:"all (default), file or dir"`
	Recursive *bool  `json:"recursive,omitempty" jsonschema:"search sub-folders too; defaults to true"`
	Limit     int    `json:"limit,omitempty" jsonschema:"matches to return; defaults to 25, at most 100"`
}

// --- synology tool outputs ---

// SynologyDiskList wraps the disks.
type SynologyDiskList struct {
	Disks []arr.SynologyDisk `json:"disks"`
	Count int                `json:"count"`
}
