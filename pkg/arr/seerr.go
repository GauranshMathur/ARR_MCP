package arr

import (
	"context"
	"fmt"
	"strings"
)

// Seerr facts that shape this file, all checked against a live 3.4.1:
//   - Lists answer {pageInfo:{page,pages,pageSize,results}, results:[...]} and
//     take take/skip, not a page number. Search and discover are TMDB-shaped
//     instead: {page,totalPages,totalResults,results}.
//   - A request carries only TMDB ids, never a title; media.status is the
//     availability of the title, request.status the state of the request.
//   - Requests made with the API key act as the original administrator unless
//     the body names a userId.
//   - Embedded users carry email, permission bits and media-server ids, none of
//     which a projection keeps.

// seerrDefaultTake bounds a list when the caller does not say.
const seerrDefaultTake = 20

// seerrRequestStatuses names MediaRequestStatus. The shipped spec lists only
// the first three; FAILED and COMPLETED come from the running code.
var seerrRequestStatuses = map[int]string{1: "PENDING", 2: "APPROVED", 3: "DECLINED", 4: "FAILED", 5: "COMPLETED"}

// seerrMediaStatuses names MediaStatus. The shipped spec says 6 is DELETED;
// the running code has BLOCKLISTED at 6 and DELETED at 7.
var seerrMediaStatuses = map[int]string{
	1: "UNKNOWN", 2: "PENDING", 3: "PROCESSING", 4: "PARTIALLY_AVAILABLE",
	5: "AVAILABLE", 6: "BLOCKLISTED", 7: "DELETED",
}

var seerrIssueTypes = map[int]string{1: "VIDEO", 2: "AUDIO", 3: "SUBTITLES", 4: "OTHER"}
var seerrIssueStatuses = map[int]string{1: "OPEN", 2: "RESOLVED"}

// seerrName names an enum value, keeping unknown ones visible rather than
// guessing, since a newer Seerr may add some.
func seerrName(names map[int]string, v int) string {
	if n, ok := names[v]; ok {
		return n
	}
	return fmt.Sprintf("UNKNOWN(%d)", v)
}

func seerrMediaStatus(v int) string { return seerrName(seerrMediaStatuses, v) }

// seerrOneOf rejects a value outside the set Seerr accepts, so the model sees
// the valid values instead of a bare 400.
func seerrOneOf(field, v string, allowed ...string) error {
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return fmt.Errorf("%s %q must be one of: %s", field, v, strings.Join(allowed, ", "))
}

// seerrPaging builds the take/skip query shared by the list endpoints.
func seerrPaging(take, skip int) Query {
	if take <= 0 {
		take = seerrDefaultTake
	}
	q := Query{"take": itoa(take)}
	if skip > 0 {
		q["skip"] = itoa(skip)
	}
	return q
}

// SeerrPageInfo reports where a take/skip page sits in the whole list.
type SeerrPageInfo struct {
	Page     int `json:"page"`
	Pages    int `json:"pages"`
	PageSize int `json:"pageSize"`
	Results  int `json:"results"`
}

// SeerrUserRef names a user without their contact details.
type SeerrUserRef struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type rawSeerrUser struct {
	ID           int    `json:"id"`
	DisplayName  string `json:"displayName"`
	RequestCount int    `json:"requestCount"`
}

func (u *rawSeerrUser) ref() *SeerrUserRef {
	if u == nil {
		return nil
	}
	return &SeerrUserRef{ID: u.ID, Name: u.DisplayName}
}

// SeerrStatus reports an instance's version and library size.
type SeerrStatus struct {
	Version         string `json:"version"`
	UpdateAvailable bool   `json:"updateAvailable"`
	RestartRequired bool   `json:"restartRequired"`
	TotalMediaItems int    `json:"totalMediaItems"`
	TotalRequests   int    `json:"totalRequests"`
	Timezone        string `json:"timezone,omitempty"`
}

// SeerrGetStatus combines /status, which knows about updates, with
// /settings/about, which knows the totals. Both are read with the key.
func SeerrGetStatus(ctx context.Context, c *Client) (SeerrStatus, error) {
	st, err := GetJSON[struct {
		Version         string `json:"version"`
		UpdateAvailable bool   `json:"updateAvailable"`
		RestartRequired bool   `json:"restartRequired"`
	}](ctx, c, "/status")
	if err != nil {
		return SeerrStatus{}, err
	}
	about, err := GetJSON[struct {
		TotalMediaItems int    `json:"totalMediaItems"`
		TotalRequests   int    `json:"totalRequests"`
		TZ              string `json:"tz"`
	}](ctx, c, "/settings/about")
	if err != nil {
		return SeerrStatus{}, err
	}
	return SeerrStatus{
		Version: st.Version, UpdateAvailable: st.UpdateAvailable, RestartRequired: st.RestartRequired,
		TotalMediaItems: about.TotalMediaItems, TotalRequests: about.TotalRequests, Timezone: about.TZ,
	}, nil
}

// SeerrCounts is the number of requests in each state.
type SeerrCounts struct {
	Total      int `json:"total"`
	Movie      int `json:"movie"`
	TV         int `json:"tv"`
	Pending    int `json:"pending" jsonschema:"waiting for approval"`
	Approved   int `json:"approved"`
	Declined   int `json:"declined"`
	Processing int `json:"processing"`
	Available  int `json:"available"`
	Completed  int `json:"completed"`
}

// SeerrRequestCounts reads /request/count.
func SeerrRequestCounts(ctx context.Context, c *Client) (SeerrCounts, error) {
	return GetJSON[SeerrCounts](ctx, c, "/request/count")
}

// SeerrSeason is one requested season and how far it has got.
type SeerrSeason struct {
	Number int    `json:"number"`
	Status string `json:"status"`
}

// SeerrRequest is the trimmed view of a media request. It carries TMDB ids,
// not a title: look the title up with seerr_get_media.
type SeerrRequest struct {
	ID          int           `json:"id"`
	Status      string        `json:"status" jsonschema:"state of the request: PENDING, APPROVED, DECLINED, FAILED or COMPLETED"`
	Type        string        `json:"type" jsonschema:"movie or tv"`
	Is4K        bool          `json:"is4k,omitempty"`
	CreatedAt   string        `json:"createdAt"`
	UpdatedAt   string        `json:"updatedAt,omitempty"`
	TMDBID      int           `json:"tmdbId,omitempty" jsonschema:"pass to seerr_get_media for the title"`
	TVDBID      int           `json:"tvdbId,omitempty"`
	MediaStatus string        `json:"mediaStatus,omitempty" jsonschema:"availability of the title itself, as opposed to the request"`
	RequestedBy *SeerrUserRef `json:"requestedBy,omitempty"`
	Seasons     []SeerrSeason `json:"seasons,omitempty"`
}

type rawSeerrRequest struct {
	ID        int    `json:"id"`
	Status    int    `json:"status"`
	Type      string `json:"type"`
	Is4K      bool   `json:"is4k"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Media     *struct {
		TMDBID int  `json:"tmdbId"`
		TVDBID *int `json:"tvdbId"`
		Status int  `json:"status"`
	} `json:"media"`
	RequestedBy *rawSeerrUser `json:"requestedBy"`
	Seasons     []struct {
		SeasonNumber int `json:"seasonNumber"`
		Status       int `json:"status"`
	} `json:"seasons"`
}

func (r rawSeerrRequest) trim() SeerrRequest {
	out := SeerrRequest{
		ID: r.ID, Status: seerrName(seerrRequestStatuses, r.Status), Type: r.Type, Is4K: r.Is4K,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestedBy: r.RequestedBy.ref(),
	}
	if r.Media != nil {
		out.TMDBID = r.Media.TMDBID
		if r.Media.TVDBID != nil {
			out.TVDBID = *r.Media.TVDBID
		}
		out.MediaStatus = seerrMediaStatus(r.Media.Status)
	}
	for _, s := range r.Seasons {
		out.Seasons = append(out.Seasons, SeerrSeason{Number: s.SeasonNumber, Status: seerrName(seerrRequestStatuses, s.Status)})
	}
	return out
}

// SeerrRequestQuery narrows SeerrListRequests. Zero values are omitted.
type SeerrRequestQuery struct {
	Filter      string
	Sort        string
	Take        int
	Skip        int
	RequestedBy int
}

// SeerrRequestPage is one page of requests.
type SeerrRequestPage struct {
	Requests []SeerrRequest `json:"requests"`
	Count    int            `json:"count"`
	Total    int            `json:"total" jsonschema:"requests matching the filter across all pages"`
	Page     int            `json:"page"`
	Pages    int            `json:"pages"`
}

// SeerrListRequests lists requests, newest first unless sorted otherwise.
func SeerrListRequests(ctx context.Context, c *Client, in SeerrRequestQuery) (SeerrRequestPage, error) {
	q := seerrPaging(in.Take, in.Skip)
	if in.Filter != "" {
		if err := seerrOneOf("filter", in.Filter, "all", "approved", "available", "pending", "processing",
			"unavailable", "failed", "deleted", "completed"); err != nil {
			return SeerrRequestPage{}, err
		}
		q["filter"] = in.Filter
	}
	if in.Sort != "" {
		if err := seerrOneOf("sort", in.Sort, "added", "modified"); err != nil {
			return SeerrRequestPage{}, err
		}
		q["sort"] = in.Sort
	}
	if in.RequestedBy > 0 {
		q["requestedBy"] = itoa(in.RequestedBy)
	}
	raw, err := GetJSON[struct {
		PageInfo SeerrPageInfo     `json:"pageInfo"`
		Results  []rawSeerrRequest `json:"results"`
	}](ctx, c, "/request", q)
	if err != nil {
		return SeerrRequestPage{}, err
	}
	out := SeerrRequestPage{
		Requests: make([]SeerrRequest, 0, len(raw.Results)), Count: len(raw.Results),
		Total: raw.PageInfo.Results, Page: raw.PageInfo.Page, Pages: raw.PageInfo.Pages,
	}
	for _, r := range raw.Results {
		out.Requests = append(out.Requests, r.trim())
	}
	return out, nil
}

// SeerrGetRequest reads one request.
func SeerrGetRequest(ctx context.Context, c *Client, id int) (SeerrRequest, error) {
	raw, err := GetJSON[rawSeerrRequest](ctx, c, "/request/"+itoa(id))
	return raw.trim(), err
}

// SeerrResult is one search or discover hit.
type SeerrResult struct {
	TMDBID       int     `json:"tmdbId" jsonschema:"pass to seerr_get_media or seerr_create_request"`
	MediaType    string  `json:"mediaType" jsonschema:"movie, tv or person"`
	Title        string  `json:"title"`
	ReleaseDate  string  `json:"releaseDate,omitempty"`
	VoteAverage  float64 `json:"voteAverage,omitempty"`
	Availability string  `json:"availability,omitempty" jsonschema:"omitted when the title has never been requested"`
}

// SeerrResultPage is one page of search or discover results.
type SeerrResultPage struct {
	Results      []SeerrResult `json:"results"`
	Count        int           `json:"count"`
	Page         int           `json:"page"`
	TotalPages   int           `json:"totalPages"`
	TotalResults int           `json:"totalResults"`
}

// seerrResults reads a TMDB-shaped page. Movies carry title and releaseDate,
// shows name and firstAirDate, people only a name.
func seerrResults(ctx context.Context, c *Client, path string, q Query) (SeerrResultPage, error) {
	raw, err := GetJSON[struct {
		Page         int `json:"page"`
		TotalPages   int `json:"totalPages"`
		TotalResults int `json:"totalResults"`
		Results      []struct {
			ID           int     `json:"id"`
			MediaType    string  `json:"mediaType"`
			Title        string  `json:"title"`
			Name         string  `json:"name"`
			ReleaseDate  string  `json:"releaseDate"`
			FirstAirDate string  `json:"firstAirDate"`
			VoteAverage  float64 `json:"voteAverage"`
			MediaInfo    *struct {
				Status int `json:"status"`
			} `json:"mediaInfo"`
		} `json:"results"`
	}](ctx, c, path, q)
	if err != nil {
		return SeerrResultPage{}, err
	}
	out := SeerrResultPage{
		Results: make([]SeerrResult, 0, len(raw.Results)), Count: len(raw.Results),
		Page: raw.Page, TotalPages: raw.TotalPages, TotalResults: raw.TotalResults,
	}
	for _, r := range raw.Results {
		item := SeerrResult{TMDBID: r.ID, MediaType: r.MediaType, Title: r.Title, ReleaseDate: r.ReleaseDate, VoteAverage: r.VoteAverage}
		if item.Title == "" {
			item.Title = r.Name
		}
		if item.ReleaseDate == "" {
			item.ReleaseDate = r.FirstAirDate
		}
		if r.MediaInfo != nil {
			item.Availability = seerrMediaStatus(r.MediaInfo.Status)
		}
		out.Results = append(out.Results, item)
	}
	return out, nil
}

// SeerrSearch searches movies, shows and people by title.
func SeerrSearch(ctx context.Context, c *Client, query string, page int) (SeerrResultPage, error) {
	if strings.TrimSpace(query) == "" {
		return SeerrResultPage{}, fmt.Errorf("query must not be empty")
	}
	if page <= 0 {
		page = 1
	}
	return seerrResults(ctx, c, "/search", Query{"query": query, "page": itoa(page)})
}

// SeerrDiscover lists trending, popular movies or popular shows.
func SeerrDiscover(ctx context.Context, c *Client, kind string, page int) (SeerrResultPage, error) {
	if err := seerrOneOf("kind", kind, "trending", "movies", "tv"); err != nil {
		return SeerrResultPage{}, err
	}
	if page <= 0 {
		page = 1
	}
	return seerrResults(ctx, c, "/discover/"+kind, Query{"page": itoa(page)})
}

// SeerrRequestRef is a request against a title, without who made it.
type SeerrRequestRef struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// SeerrMediaSeason is one season of a show.
type SeerrMediaSeason struct {
	Number   int    `json:"number"`
	Name     string `json:"name,omitempty"`
	Episodes int    `json:"episodes"`
}

// SeerrMedia is the trimmed detail of a movie or show.
type SeerrMedia struct {
	TMDBID           int                `json:"tmdbId"`
	MediaType        string             `json:"mediaType"`
	Title            string             `json:"title"`
	ReleaseDate      string             `json:"releaseDate,omitempty"`
	Status           string             `json:"status,omitempty" jsonschema:"production status, e.g. Released or Ended"`
	Overview         string             `json:"overview,omitempty"`
	Genres           []string           `json:"genres,omitempty"`
	VoteAverage      float64            `json:"voteAverage,omitempty"`
	IMDBID           string             `json:"imdbId,omitempty"`
	RuntimeMinutes   int                `json:"runtimeMinutes,omitempty"`
	NumberOfSeasons  int                `json:"numberOfSeasons,omitempty"`
	NumberOfEpisodes int                `json:"numberOfEpisodes,omitempty"`
	Seasons          []SeerrMediaSeason `json:"seasons,omitempty"`
	Availability     string             `json:"availability,omitempty" jsonschema:"omitted when the title has never been requested"`
	Requests         []SeerrRequestRef  `json:"requests,omitempty"`
}

// SeerrGetMedia reads a movie or show's detail, with its availability and the
// requests made for it. The upstream payload also carries credits, videos,
// watch providers and keywords, none of which are kept.
func SeerrGetMedia(ctx context.Context, c *Client, mediaType string, id int) (SeerrMedia, error) {
	if err := seerrOneOf("mediaType", mediaType, "movie", "tv"); err != nil {
		return SeerrMedia{}, err
	}
	raw, err := GetJSON[struct {
		ID               int     `json:"id"`
		Title            string  `json:"title"`
		Name             string  `json:"name"`
		ReleaseDate      string  `json:"releaseDate"`
		FirstAirDate     string  `json:"firstAirDate"`
		Status           string  `json:"status"`
		Overview         string  `json:"overview"`
		VoteAverage      float64 `json:"voteAverage"`
		IMDBID           string  `json:"imdbId"`
		Runtime          int     `json:"runtime"`
		NumberOfSeasons  int     `json:"numberOfSeasons"`
		NumberOfEpisodes int     `json:"numberOfEpisodes"`
		Genres           []struct {
			Name string `json:"name"`
		} `json:"genres"`
		Seasons []struct {
			SeasonNumber int    `json:"seasonNumber"`
			Name         string `json:"name"`
			EpisodeCount int    `json:"episodeCount"`
		} `json:"seasons"`
		MediaInfo *struct {
			Status   int `json:"status"`
			Requests []struct {
				ID     int `json:"id"`
				Status int `json:"status"`
			} `json:"requests"`
		} `json:"mediaInfo"`
	}](ctx, c, "/"+mediaType+"/"+itoa(id))
	if err != nil {
		return SeerrMedia{}, err
	}
	out := SeerrMedia{
		TMDBID: raw.ID, MediaType: mediaType, Title: raw.Title, ReleaseDate: raw.ReleaseDate,
		Status: raw.Status, Overview: raw.Overview, VoteAverage: raw.VoteAverage, IMDBID: raw.IMDBID,
		RuntimeMinutes: raw.Runtime, NumberOfSeasons: raw.NumberOfSeasons, NumberOfEpisodes: raw.NumberOfEpisodes,
	}
	if out.Title == "" {
		out.Title = raw.Name
	}
	if out.ReleaseDate == "" {
		out.ReleaseDate = raw.FirstAirDate
	}
	for _, g := range raw.Genres {
		out.Genres = append(out.Genres, g.Name)
	}
	for _, s := range raw.Seasons {
		out.Seasons = append(out.Seasons, SeerrMediaSeason{Number: s.SeasonNumber, Name: s.Name, Episodes: s.EpisodeCount})
	}
	if raw.MediaInfo != nil {
		out.Availability = seerrMediaStatus(raw.MediaInfo.Status)
		for _, r := range raw.MediaInfo.Requests {
			out.Requests = append(out.Requests, SeerrRequestRef{ID: r.ID, Status: seerrName(seerrRequestStatuses, r.Status)})
		}
	}
	return out, nil
}

// SeerrUser is a Seerr account, without contact details.
type SeerrUser struct {
	ID           int    `json:"id"`
	DisplayName  string `json:"displayName"`
	RequestCount int    `json:"requestCount"`
}

// SeerrUserPage is one page of users.
type SeerrUserPage struct {
	Users []SeerrUser `json:"users"`
	Count int         `json:"count"`
	Total int         `json:"total"`
}

// SeerrListUsers lists accounts with how many requests each has made. Email
// addresses, permission bits and media server ids are not returned.
func SeerrListUsers(ctx context.Context, c *Client, take, skip int) (SeerrUserPage, error) {
	raw, err := GetJSON[struct {
		PageInfo SeerrPageInfo  `json:"pageInfo"`
		Results  []rawSeerrUser `json:"results"`
	}](ctx, c, "/user", seerrPaging(take, skip))
	if err != nil {
		return SeerrUserPage{}, err
	}
	out := SeerrUserPage{Users: make([]SeerrUser, 0, len(raw.Results)), Count: len(raw.Results), Total: raw.PageInfo.Results}
	for _, u := range raw.Results {
		out.Users = append(out.Users, SeerrUser(u))
	}
	return out, nil
}

// SeerrIssueComment is one comment on an issue.
type SeerrIssueComment struct {
	ID        int    `json:"id"`
	User      string `json:"user"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// SeerrIssue is a problem report against a title. Like a request it carries
// TMDB ids rather than a title.
type SeerrIssue struct {
	ID             int                 `json:"id"`
	Type           string              `json:"type" jsonschema:"VIDEO, AUDIO, SUBTITLES or OTHER"`
	Status         string              `json:"status" jsonschema:"OPEN or RESOLVED"`
	MediaType      string              `json:"mediaType,omitempty"`
	TMDBID         int                 `json:"tmdbId,omitempty" jsonschema:"pass to seerr_get_media for the title"`
	ProblemSeason  int                 `json:"problemSeason,omitempty"`
	ProblemEpisode int                 `json:"problemEpisode,omitempty"`
	CreatedBy      *SeerrUserRef       `json:"createdBy,omitempty"`
	CreatedAt      string              `json:"createdAt"`
	UpdatedAt      string              `json:"updatedAt,omitempty"`
	CommentCount   int                 `json:"commentCount"`
	Comments       []SeerrIssueComment `json:"comments,omitempty" jsonschema:"only on seerr_get_issue and after a comment"`
}

type rawSeerrIssue struct {
	ID             int           `json:"id"`
	IssueType      int           `json:"issueType"`
	Status         int           `json:"status"`
	ProblemSeason  int           `json:"problemSeason"`
	ProblemEpisode int           `json:"problemEpisode"`
	CreatedAt      string        `json:"createdAt"`
	UpdatedAt      string        `json:"updatedAt"`
	CreatedBy      *rawSeerrUser `json:"createdBy"`
	Media          *struct {
		MediaType string `json:"mediaType"`
		TMDBID    int    `json:"tmdbId"`
	} `json:"media"`
	Comments []struct {
		ID        int           `json:"id"`
		Message   string        `json:"message"`
		CreatedAt string        `json:"createdAt"`
		User      *rawSeerrUser `json:"user"`
	} `json:"comments"`
}

func (r rawSeerrIssue) trim(withComments bool) SeerrIssue {
	out := SeerrIssue{
		ID: r.ID, Type: seerrName(seerrIssueTypes, r.IssueType), Status: seerrName(seerrIssueStatuses, r.Status),
		ProblemSeason: r.ProblemSeason, ProblemEpisode: r.ProblemEpisode, CreatedBy: r.CreatedBy.ref(),
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, CommentCount: len(r.Comments),
	}
	if r.Media != nil {
		out.MediaType, out.TMDBID = r.Media.MediaType, r.Media.TMDBID
	}
	if withComments {
		for _, cm := range r.Comments {
			name := ""
			if cm.User != nil {
				name = cm.User.DisplayName
			}
			out.Comments = append(out.Comments, SeerrIssueComment{ID: cm.ID, User: name, Message: cm.Message, CreatedAt: cm.CreatedAt})
		}
	}
	return out
}

// SeerrIssueQuery narrows SeerrListIssues. Zero values are omitted, so the
// filter is Seerr's own default, open.
type SeerrIssueQuery struct {
	Filter      string
	Sort        string
	Take        int
	Skip        int
	RequestedBy int
}

// SeerrIssuePage is one page of issues, without their comments.
type SeerrIssuePage struct {
	Issues []SeerrIssue `json:"issues"`
	Count  int          `json:"count"`
	Total  int          `json:"total" jsonschema:"issues matching the filter across all pages"`
}

// SeerrListIssues lists issues. Seerr shows only open ones unless asked.
func SeerrListIssues(ctx context.Context, c *Client, in SeerrIssueQuery) (SeerrIssuePage, error) {
	q := seerrPaging(in.Take, in.Skip)
	if in.Filter != "" {
		if err := seerrOneOf("filter", in.Filter, "all", "open", "resolved"); err != nil {
			return SeerrIssuePage{}, err
		}
		q["filter"] = in.Filter
	}
	if in.Sort != "" {
		if err := seerrOneOf("sort", in.Sort, "added", "modified"); err != nil {
			return SeerrIssuePage{}, err
		}
		q["sort"] = in.Sort
	}
	if in.RequestedBy > 0 {
		q["requestedBy"] = itoa(in.RequestedBy)
	}
	raw, err := GetJSON[struct {
		PageInfo SeerrPageInfo   `json:"pageInfo"`
		Results  []rawSeerrIssue `json:"results"`
	}](ctx, c, "/issue", q)
	if err != nil {
		return SeerrIssuePage{}, err
	}
	out := SeerrIssuePage{Issues: make([]SeerrIssue, 0, len(raw.Results)), Count: len(raw.Results), Total: raw.PageInfo.Results}
	for _, r := range raw.Results {
		out.Issues = append(out.Issues, r.trim(false))
	}
	return out, nil
}

// SeerrGetIssue reads one issue with its comments.
func SeerrGetIssue(ctx context.Context, c *Client, id int) (SeerrIssue, error) {
	raw, err := GetJSON[rawSeerrIssue](ctx, c, "/issue/"+itoa(id))
	return raw.trim(true), err
}

// SeerrNewRequest is a request to add a movie or show.
type SeerrNewRequest struct {
	MediaType  string
	MediaID    int
	Seasons    []int
	AllSeasons bool
	Is4K       bool
	// UserID files the request for another user. Zero leaves it with the API
	// key's user, the original administrator.
	UserID int
}

// SeerrCreateRequest files a request. Seerr approves it at once when the
// requesting user has auto-approve, which the administrator does.
func SeerrCreateRequest(ctx context.Context, c *Client, in SeerrNewRequest) (SeerrRequest, error) {
	if err := seerrOneOf("mediaType", in.MediaType, "movie", "tv"); err != nil {
		return SeerrRequest{}, err
	}
	if in.MediaID <= 0 {
		return SeerrRequest{}, fmt.Errorf("mediaId must be a TMDB id from seerr_search")
	}
	hasSeasons := len(in.Seasons) > 0 || in.AllSeasons
	switch {
	case in.MediaType == "movie" && hasSeasons:
		return SeerrRequest{}, fmt.Errorf("seasons only apply to tv requests")
	case in.MediaType == "tv" && !hasSeasons:
		return SeerrRequest{}, fmt.Errorf("a tv request needs seasons or allSeasons")
	case len(in.Seasons) > 0 && in.AllSeasons:
		return SeerrRequest{}, fmt.Errorf("pass seasons or allSeasons, not both")
	}

	body := map[string]any{"mediaType": in.MediaType, "mediaId": in.MediaID}
	if in.AllSeasons {
		body["seasons"] = "all"
	} else if len(in.Seasons) > 0 {
		body["seasons"] = in.Seasons
	}
	if in.Is4K {
		body["is4k"] = true
	}
	if in.UserID > 0 {
		body["userId"] = in.UserID
	}
	return seerrRequestPost(ctx, c, "/request", body)
}

func seerrRequestPost(ctx context.Context, c *Client, path string, body any) (SeerrRequest, error) {
	resp, err := c.Post(ctx, path, body)
	if err != nil {
		return SeerrRequest{}, err
	}
	var raw rawSeerrRequest
	if err := unmarshal(resp, &raw); err != nil {
		return SeerrRequest{}, err
	}
	return raw.trim(), nil
}

// SeerrSetRequestStatus approves or declines a request.
func SeerrSetRequestStatus(ctx context.Context, c *Client, id int, action string) (SeerrRequest, error) {
	if err := seerrOneOf("action", action, "approve", "decline"); err != nil {
		return SeerrRequest{}, err
	}
	return seerrRequestPost(ctx, c, "/request/"+itoa(id)+"/"+action, nil)
}

// SeerrRetryRequest sends a failed request to Sonarr or Radarr again.
func SeerrRetryRequest(ctx context.Context, c *Client, id int) (SeerrRequest, error) {
	return seerrRequestPost(ctx, c, "/request/"+itoa(id)+"/retry", nil)
}

func seerrIssuePost(ctx context.Context, c *Client, path string, body any) (SeerrIssue, error) {
	resp, err := c.Post(ctx, path, body)
	if err != nil {
		return SeerrIssue{}, err
	}
	var raw rawSeerrIssue
	if err := unmarshal(resp, &raw); err != nil {
		return SeerrIssue{}, err
	}
	return raw.trim(true), nil
}

// SeerrCommentIssue adds a comment and returns the issue with it.
func SeerrCommentIssue(ctx context.Context, c *Client, id int, message string) (SeerrIssue, error) {
	if strings.TrimSpace(message) == "" {
		return SeerrIssue{}, fmt.Errorf("message must not be empty")
	}
	return seerrIssuePost(ctx, c, "/issue/"+itoa(id)+"/comment", map[string]string{"message": message})
}

// SeerrSetIssueStatus resolves or reopens an issue.
func SeerrSetIssueStatus(ctx context.Context, c *Client, id int, status string) (SeerrIssue, error) {
	if err := seerrOneOf("status", status, "open", "resolved"); err != nil {
		return SeerrIssue{}, err
	}
	return seerrIssuePost(ctx, c, "/issue/"+itoa(id)+"/"+status, nil)
}

// SeerrDeleteRequest removes a request. Seerr answers 204.
func SeerrDeleteRequest(ctx context.Context, c *Client, id int) error {
	_, err := c.Delete(ctx, "/request/"+itoa(id))
	return err
}
