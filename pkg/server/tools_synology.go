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

// --- synology write tool inputs ---

// SynologyCreateFolderArgs names the parent folder and the new folder's name.
type SynologyCreateFolderArgs struct {
	InstanceArg
	Path string `json:"path" jsonschema:"existing parent folder, absolute and starting with a share, e.g. /NAS/Scratch; must be inside the instance's allowedPaths. No .. segments"`
	Name string `json:"name" jsonschema:"name of the new folder: one name, no slash"`
}

// SynologyRenameArgs names the entry and its new name.
type SynologyRenameArgs struct {
	InstanceArg
	Path    string `json:"path" jsonschema:"file or folder to rename, absolute, e.g. /NAS/Scratch/old; must be inside the instance's allowedPaths. No .. segments"`
	NewName string `json:"new_name" jsonschema:"the new name: one name, no slash. The entry stays in its folder"`
}

// SynologyCopyMoveArgs describes a copy or move.
type SynologyCopyMoveArgs struct {
	InstanceArg
	Sources     []string `json:"sources" jsonschema:"files and folders to copy or move, absolute; every one must be inside the instance's allowedPaths. No .. segments"`
	Destination string   `json:"destination" jsonschema:"existing folder to copy or move them into; must be inside the instance's allowedPaths"`
	Operation   string   `json:"operation" jsonschema:"copy or move. A move removes each source after it is copied"`
	Overwrite   bool     `json:"overwrite,omitempty" jsonschema:"replace files that already exist in the destination; defaults to false, which skips them"`
}

// SynologyDeleteArgs names what to delete.
type SynologyDeleteArgs struct {
	InstanceArg
	Paths     []string `json:"paths" jsonschema:"files and folders to delete, absolute, 1 to 10, no duplicates; each must be strictly inside the instance's allowedPaths (not an allowed path itself, not a share root, not in a #recycle folder). No .. segments"`
	Recursive bool     `json:"recursive,omitempty" jsonschema:"also delete everything inside a folder; defaults to false, in which case DSM errors on a folder that contains anything"`
}
