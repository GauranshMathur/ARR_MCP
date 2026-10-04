package server

// --- seerr tool inputs ---

// SeerrListRequestsArgs narrows seerr_list_requests.
type SeerrListRequestsArgs struct {
	InstanceArg
	Filter      string `json:"filter,omitempty" jsonschema:"all, approved, available, pending, processing, unavailable, failed, deleted or completed; omit for all"`
	Sort        string `json:"sort,omitempty" jsonschema:"added or modified; defaults to added, newest first"`
	Take        int    `json:"take,omitempty" jsonschema:"requests per page; defaults to 20"`
	Skip        int    `json:"skip,omitempty" jsonschema:"requests to skip, for the next page: skip = take * pages already read"`
	RequestedBy int    `json:"requestedBy,omitempty" jsonschema:"only requests from this user id, from seerr_list_users"`
}

// SeerrRequestIDArgs identifies one request.
type SeerrRequestIDArgs struct {
	InstanceArg
	RequestID int `json:"requestId" jsonschema:"request id from seerr_list_requests"`
}

// SeerrSearchArgs is the input for seerr_search.
type SeerrSearchArgs struct {
	InstanceArg
	Query string `json:"query" jsonschema:"title to search for"`
	Page  int    `json:"page,omitempty" jsonschema:"page number starting at 1"`
}

// SeerrDiscoverArgs is the input for seerr_discover.
type SeerrDiscoverArgs struct {
	InstanceArg
	Kind string `json:"kind" jsonschema:"trending, movies (popular movies) or tv (popular shows)"`
	Page int    `json:"page,omitempty" jsonschema:"page number starting at 1"`
}

// SeerrMediaArgs is the input for seerr_get_media.
type SeerrMediaArgs struct {
	InstanceArg
	MediaType string `json:"mediaType" jsonschema:"movie or tv"`
	TMDBID    int    `json:"tmdbId" jsonschema:"TMDB id, as in a request's tmdbId or a search result"`
}

// SeerrListUsersArgs pages seerr_list_users.
type SeerrListUsersArgs struct {
	InstanceArg
	Take int `json:"take,omitempty" jsonschema:"users per page; defaults to 20"`
	Skip int `json:"skip,omitempty" jsonschema:"users to skip, for the next page"`
}

// SeerrListIssuesArgs narrows seerr_list_issues.
type SeerrListIssuesArgs struct {
	InstanceArg
	Filter      string `json:"filter,omitempty" jsonschema:"open, resolved or all; Seerr defaults to open"`
	Sort        string `json:"sort,omitempty" jsonschema:"added or modified; defaults to added"`
	Take        int    `json:"take,omitempty" jsonschema:"issues per page; defaults to 20"`
	Skip        int    `json:"skip,omitempty" jsonschema:"issues to skip, for the next page"`
	RequestedBy int    `json:"requestedBy,omitempty" jsonschema:"only issues raised by this user id"`
}

// SeerrIssueIDArgs identifies one issue.
type SeerrIssueIDArgs struct {
	InstanceArg
	IssueID int `json:"issueId" jsonschema:"issue id from seerr_list_issues"`
}

// SeerrCreateRequestArgs is the input for seerr_create_request.
type SeerrCreateRequestArgs struct {
	InstanceArg
	MediaType  string `json:"mediaType" jsonschema:"movie or tv"`
	TMDBID     int    `json:"tmdbId" jsonschema:"TMDB id from seerr_search or seerr_discover"`
	Seasons    []int  `json:"seasons,omitempty" jsonschema:"season numbers to request; tv only, and either this or allSeasons is required for tv"`
	AllSeasons bool   `json:"allSeasons,omitempty" jsonschema:"request every season; tv only"`
	Is4K       bool   `json:"is4k,omitempty" jsonschema:"request the 4K version; needs a 4K Sonarr or Radarr behind Seerr"`
	UserID     int    `json:"userId,omitempty" jsonschema:"file the request as this user id from seerr_list_users; omit to file it as the administrator"`
}

// SeerrCommentIssueArgs is the input for seerr_comment_issue.
type SeerrCommentIssueArgs struct {
	InstanceArg
	IssueID int    `json:"issueId" jsonschema:"issue id from seerr_list_issues"`
	Message string `json:"message"`
}

// SeerrSetIssueStatusArgs is the input for seerr_set_issue_status.
type SeerrSetIssueStatusArgs struct {
	InstanceArg
	IssueID int    `json:"issueId" jsonschema:"issue id from seerr_list_issues"`
	Status  string `json:"status" jsonschema:"resolved to close the issue, open to reopen it"`
}
