package arr

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// The profile and configuration resources are written back the way they were
// read: fetched into a map[string]any, mutated, and sent whole. A typed round
// trip would drop every key this package does not model and silently reset it
// on the instance, and these resources are full of such keys -- Radarr's
// quality profiles carry a language object Sonarr's do not, and
// colonReplacementFormat is an integer on Sonarr and a string on Radarr, so no
// single struct decodes both.

// --- quality profiles ---

// QualityProfileItem is one entry in a profile's quality list: either a single
// quality or a named group of qualities the profile treats as equivalent.
type QualityProfileItem struct {
	Name    string   `json:"name"`
	Allowed bool     `json:"allowed"`
	Members []string `json:"members,omitempty" jsonschema:"the qualities in this group, worst first"`
}

// FormatScore is the score one custom format contributes in a profile.
type FormatScore struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
}

// QualityProfileDetail is one quality profile with the qualities it accepts and
// the custom format scores it applies. The cutoff is reported by name because
// upstream stores it as an id a caller cannot act on, and the format scores are
// trimmed to the formats actually scored: a real profile lists every custom
// format on the instance, most of them sitting at zero.
type QualityProfileDetail struct {
	ID                    int                  `json:"id"`
	Name                  string               `json:"name"`
	UpgradeAllowed        bool                 `json:"upgradeAllowed" jsonschema:"whether a better release replaces one already on disk"`
	Cutoff                string               `json:"cutoff,omitempty" jsonschema:"the quality or group upgrading stops at"`
	MinFormatScore        int                  `json:"minFormatScore"`
	CutoffFormatScore     int                  `json:"cutoffFormatScore"`
	MinUpgradeFormatScore int                  `json:"minUpgradeFormatScore,omitempty"`
	Language              string               `json:"language,omitempty" jsonschema:"Radarr only"`
	Items                 []QualityProfileItem `json:"items" jsonschema:"every quality the profile knows, worst first"`
	FormatScores          []FormatScore        `json:"formatScores,omitempty" jsonschema:"custom formats scored non-zero; the rest are omitted"`
}

// rawQualityItem mirrors one entry of an upstream profile's quality list. A
// group carries its own id and name and nests its members in items; a single
// quality carries neither and describes itself in quality.
type rawQualityItem struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Allowed bool   `json:"allowed"`
	Quality *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"quality"`
	Items []rawQualityItem `json:"items"`
}

// name returns the group name, or the quality's own name for a single quality.
func (r rawQualityItem) name() string {
	if r.Name != "" {
		return r.Name
	}
	if r.Quality != nil {
		return r.Quality.Name
	}
	return ""
}

// id returns the id a cutoff would use to name this entry.
//
// For a single quality that is the quality's own id, not the id of the quality
// *definition* that wraps it: WEBDL-480p is definition 4 and quality 8, and a
// cutoff set to the definition id points at a different quality entirely.
func (r rawQualityItem) id() int {
	if r.Quality != nil {
		return r.Quality.ID
	}
	return r.ID
}

// rawQualityProfile mirrors the upstream quality profile resource.
type rawQualityProfile struct {
	ID                    int              `json:"id"`
	Name                  string           `json:"name"`
	UpgradeAllowed        bool             `json:"upgradeAllowed"`
	Cutoff                int              `json:"cutoff"`
	MinFormatScore        int              `json:"minFormatScore"`
	CutoffFormatScore     int              `json:"cutoffFormatScore"`
	MinUpgradeFormatScore int              `json:"minUpgradeFormatScore"`
	Items                 []rawQualityItem `json:"items"`
	Language              *struct {
		Name string `json:"name"`
	} `json:"language"`
	FormatItems []struct {
		Name  string `json:"name"`
		Score int    `json:"score"`
	} `json:"formatItems"`
}

// toDetail projects an upstream profile, naming the cutoff and dropping the
// custom formats the profile leaves at zero.
func (r rawQualityProfile) toDetail() QualityProfileDetail {
	out := QualityProfileDetail{
		ID: r.ID, Name: r.Name, UpgradeAllowed: r.UpgradeAllowed,
		MinFormatScore: r.MinFormatScore, CutoffFormatScore: r.CutoffFormatScore,
		MinUpgradeFormatScore: r.MinUpgradeFormatScore,
		Items:                 make([]QualityProfileItem, 0, len(r.Items)),
	}
	if r.Language != nil {
		out.Language = r.Language.Name
	}
	for _, item := range r.Items {
		entry := QualityProfileItem{Name: item.name(), Allowed: item.Allowed}
		for _, member := range item.Items {
			entry.Members = append(entry.Members, member.name())
			if member.id() == r.Cutoff {
				out.Cutoff = member.name()
			}
		}
		if item.id() == r.Cutoff {
			out.Cutoff = entry.Name
		}
		out.Items = append(out.Items, entry)
	}
	for _, format := range r.FormatItems {
		if format.Score != 0 {
			out.FormatScores = append(out.FormatScores, FormatScore{Name: format.Name, Score: format.Score})
		}
	}
	return out
}

// GetQualityProfile returns one quality profile in full.
func GetQualityProfile(ctx context.Context, c *Client, id int) (QualityProfileDetail, error) {
	raw, err := GetJSON[rawQualityProfile](ctx, c, "/qualityprofile/"+itoa(id))
	if err != nil {
		return QualityProfileDetail{}, err
	}
	return raw.toDetail(), nil
}

// QualityProfileCreate describes a new quality profile. Qualities are named
// rather than numbered, so a caller never has to guess an id.
type QualityProfileCreate struct {
	// Name identifies the profile in the library.
	Name string
	// Allowed names the qualities and quality groups the profile accepts.
	Allowed []string
	// Cutoff names the quality upgrading stops at; empty picks the best of
	// Allowed.
	Cutoff string
	// UpgradeAllowed decides whether a better release replaces one on disk.
	UpgradeAllowed *bool
}

// CreateQualityProfile adds a quality profile, starting from the instance's own
// schema so the qualities, groups and per-service settings come from the
// service rather than from assumptions about which ones exist.
func CreateQualityProfile(ctx context.Context, c *Client, in QualityProfileCreate) (QualityProfileDetail, error) {
	if strings.TrimSpace(in.Name) == "" {
		return QualityProfileDetail{}, fmt.Errorf("no profile name given")
	}
	if len(in.Allowed) == 0 {
		return QualityProfileDetail{}, fmt.Errorf(
			"no allowed qualities given; name them as the list_quality_definitions tool reports them")
	}

	profile, err := GetJSON[map[string]any](ctx, c, "/qualityprofile/schema")
	if err != nil {
		return QualityProfileDetail{}, err
	}
	profile["name"] = in.Name
	if in.UpgradeAllowed != nil {
		profile["upgradeAllowed"] = *in.UpgradeAllowed
	}

	qualities, order := profileQualities(profile)
	best, err := setQualitiesAllowed(qualities, in.Allowed, true)
	if err != nil {
		return QualityProfileDetail{}, err
	}
	cutoff := in.Cutoff
	if cutoff == "" {
		// The service orders its qualities worst first, so the last one the
		// caller allowed is the best one they allowed.
		cutoff = bestOf(order, best)
	}
	if err := setCutoff(profile, qualities, cutoff); err != nil {
		return QualityProfileDetail{}, err
	}

	body, err := c.Post(ctx, "/qualityprofile", profile)
	if err != nil {
		return QualityProfileDetail{}, err
	}
	var raw rawQualityProfile
	if err := unmarshal(body, &raw); err != nil {
		return QualityProfileDetail{}, err
	}
	return raw.toDetail(), nil
}

// QualityProfileUpdate changes one quality profile. Every field is optional so
// an omitted one leaves that setting exactly as it was.
type QualityProfileUpdate struct {
	// ID is the profile to change.
	ID int
	// Name renames the profile.
	Name *string
	// UpgradeAllowed decides whether a better release replaces one on disk.
	UpgradeAllowed *bool
	// Cutoff names the quality upgrading stops at.
	Cutoff *string
	// Allow names qualities to accept, in addition to those already accepted.
	Allow []string
	// Disallow names qualities to stop accepting.
	Disallow []string
	// MinFormatScore is the custom format score a release must reach.
	MinFormatScore *int
	// CutoffFormatScore is the score above which upgrading stops.
	CutoffFormatScore *int
	// FormatScores sets the score of named custom formats.
	FormatScores map[string]int
}

// UpdateQualityProfile changes a quality profile in place, reading the stored
// record first so the settings this package does not model survive the write.
func UpdateQualityProfile(ctx context.Context, c *Client, in QualityProfileUpdate) (QualityProfileDetail, error) {
	path := "/qualityprofile/" + itoa(in.ID)
	profile, err := GetJSON[map[string]any](ctx, c, path)
	if err != nil {
		return QualityProfileDetail{}, err
	}

	if in.Name != nil {
		profile["name"] = *in.Name
	}
	if in.UpgradeAllowed != nil {
		profile["upgradeAllowed"] = *in.UpgradeAllowed
	}
	if in.MinFormatScore != nil {
		profile["minFormatScore"] = *in.MinFormatScore
	}
	if in.CutoffFormatScore != nil {
		profile["cutoffFormatScore"] = *in.CutoffFormatScore
	}

	qualities, _ := profileQualities(profile)
	if _, err := setQualitiesAllowed(qualities, in.Allow, true); err != nil {
		return QualityProfileDetail{}, err
	}
	if _, err := setQualitiesAllowed(qualities, in.Disallow, false); err != nil {
		return QualityProfileDetail{}, err
	}
	if in.Cutoff != nil {
		if err := setCutoff(profile, qualities, *in.Cutoff); err != nil {
			return QualityProfileDetail{}, err
		}
	}
	if err := setFormatScores(profile, in.FormatScores); err != nil {
		return QualityProfileDetail{}, err
	}

	body, err := c.Put(ctx, path, profile)
	if err != nil {
		return QualityProfileDetail{}, err
	}
	var raw rawQualityProfile
	if err := unmarshal(body, &raw); err != nil {
		return QualityProfileDetail{}, err
	}
	return raw.toDetail(), nil
}

// DeleteQualityProfile removes a quality profile. A profile still in use is
// refused by the service, and that refusal is returned rather than hidden.
func DeleteQualityProfile(ctx context.Context, c *Client, id int) error {
	_, err := c.Delete(ctx, "/qualityprofile/"+itoa(id))
	return err
}

// profileQualities indexes a decoded profile's quality list by name and returns
// the names in the profile's own order, which runs worst quality first. The
// table holds the live entries from the decoded profile, so allowing one
// through the table changes the profile itself.
func profileQualities(profile map[string]any) (nameTable, []string) {
	table := nameTable{kind: "quality", byKey: map[string]map[string]any{}}
	var order []string

	var walk func(items []any)
	walk = func(items []any) {
		for _, raw := range items {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if name := itemName(item); name != "" {
				table.byKey[strings.ToLower(name)] = item
				table.names = append(table.names, name)
				order = append(order, name)
			}
			if nested, ok := item["items"].([]any); ok {
				walk(nested)
			}
		}
	}
	items, _ := profile["items"].([]any)
	walk(items)

	sort.Strings(table.names)
	return table, order
}

// itemName returns the group name of a decoded quality entry, or the quality's
// own name for a single quality.
func itemName(item map[string]any) string {
	if name, ok := item["name"].(string); ok && name != "" {
		return name
	}
	if quality, ok := item["quality"].(map[string]any); ok {
		if name, ok := quality["name"].(string); ok {
			return name
		}
	}
	return ""
}

// itemID returns the id a cutoff would use for a decoded quality entry: the
// group id for a group, and for a single quality the quality's own id rather
// than the id of the quality definition wrapping it.
func itemID(item map[string]any) (int, bool) {
	if quality, ok := item["quality"].(map[string]any); ok {
		id, ok := quality["id"].(float64)
		return int(id), ok
	}
	id, ok := item["id"].(float64)
	return int(id), ok
}

// setQualitiesAllowed turns the named entries on or off and returns the set of
// names it touched. Naming a group turns its members with it, which is what the
// web UI does when a group is ticked.
func setQualitiesAllowed(qualities nameTable, names []string, allowed bool) (map[string]bool, error) {
	touched := make(map[string]bool, len(names))
	for _, name := range names {
		item, err := qualities.resolve(name)
		if err != nil {
			return nil, err
		}
		setItemAllowed(item, allowed)
		touched[itemName(item)] = true
	}
	return touched, nil
}

// setItemAllowed turns one entry on or off, together with any group members.
func setItemAllowed(item map[string]any, allowed bool) {
	item["allowed"] = allowed
	members, ok := item["items"].([]any)
	if !ok {
		return
	}
	for _, raw := range members {
		if member, ok := raw.(map[string]any); ok {
			setItemAllowed(member, allowed)
		}
	}
}

// bestOf returns the last name in order that appears in touched. The service
// lists qualities worst first, so that is the best of them.
func bestOf(order []string, touched map[string]bool) string {
	best := ""
	for _, name := range order {
		if touched[name] {
			best = name
		}
	}
	return best
}

// setCutoff points a decoded profile's cutoff at the named quality or group.
func setCutoff(profile map[string]any, qualities nameTable, name string) error {
	if name == "" {
		return fmt.Errorf("no cutoff quality given")
	}
	item, err := qualities.resolve(name)
	if err != nil {
		return err
	}
	id, ok := itemID(item)
	if !ok {
		return fmt.Errorf("quality %q carries no id this profile can cut off at", name)
	}
	profile["cutoff"] = id
	return nil
}

// setFormatScores scores the named custom formats in a decoded profile. The
// names are applied in sorted order so a run with several unknown names always
// reports the same one.
func setFormatScores(profile map[string]any, scores map[string]int) error {
	if len(scores) == 0 {
		return nil
	}
	formats := nameTable{kind: "custom format", byKey: map[string]map[string]any{}}
	items, _ := profile["formatItems"].([]any)
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name, _ := item["name"].(string)
		if name == "" {
			continue
		}
		formats.byKey[strings.ToLower(name)] = item
		formats.names = append(formats.names, name)
	}
	sort.Strings(formats.names)

	for _, name := range sortedKeys(scores) {
		item, err := formats.resolve(name)
		if err != nil {
			return err
		}
		item["score"] = scores[name]
	}
	return nil
}

// sortedKeys returns a map's keys in a stable order, so requests built from a
// map are reproducible and errors always name the same key first.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// --- custom formats ---

// CustomFormatSpecification is one rule that decides whether a custom format
// matches a release. Fields holds the rule's settings by name -- value carries
// the regular expression for the title and release-group rules -- because the
// upstream shape wraps each setting in a label, help text and privacy marker
// that answer nothing a caller asks.
type CustomFormatSpecification struct {
	Name           string         `json:"name"`
	Implementation string         `json:"implementation" jsonschema:"the rule type, e.g. ReleaseTitleSpecification or ReleaseGroupSpecification"`
	Negate         bool           `json:"negate,omitempty" jsonschema:"invert the rule, so a match rejects instead of accepts"`
	Required       bool           `json:"required,omitempty" jsonschema:"the format only matches when this rule does"`
	Fields         map[string]any `json:"fields,omitempty" jsonschema:"the rule's settings by name, e.g. value for a regular expression"`
}

// CustomFormatDetail is one custom format with the rules that make it match.
type CustomFormatDetail struct {
	ID                  int                         `json:"id"`
	Name                string                      `json:"name"`
	IncludeWhenRenaming bool                        `json:"includeCustomFormatWhenRenaming"`
	Specifications      []CustomFormatSpecification `json:"specifications"`
}

// rawCustomFormatDetail mirrors the upstream custom format resource.
type rawCustomFormatDetail struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	IncludeWhenRenaming bool   `json:"includeCustomFormatWhenRenaming"`
	Specifications      []struct {
		Name           string `json:"name"`
		Implementation string `json:"implementation"`
		Negate         bool   `json:"negate"`
		Required       bool   `json:"required"`
		Fields         []struct {
			Name  string `json:"name"`
			Value any    `json:"value"`
		} `json:"fields"`
	} `json:"specifications"`
}

// toDetail projects an upstream custom format, collapsing each rule's field
// array into the settings it actually carries.
func (r rawCustomFormatDetail) toDetail() CustomFormatDetail {
	out := CustomFormatDetail{
		ID: r.ID, Name: r.Name, IncludeWhenRenaming: r.IncludeWhenRenaming,
		Specifications: make([]CustomFormatSpecification, 0, len(r.Specifications)),
	}
	for _, spec := range r.Specifications {
		entry := CustomFormatSpecification{
			Name: spec.Name, Implementation: spec.Implementation,
			Negate: spec.Negate, Required: spec.Required,
		}
		for _, field := range spec.Fields {
			if field.Name == "" || field.Value == nil {
				continue
			}
			if entry.Fields == nil {
				entry.Fields = map[string]any{}
			}
			entry.Fields[field.Name] = field.Value
		}
		out.Specifications = append(out.Specifications, entry)
	}
	return out
}

// GetCustomFormat returns one custom format with its matching rules.
func GetCustomFormat(ctx context.Context, c *Client, id int) (CustomFormatDetail, error) {
	raw, err := GetJSON[rawCustomFormatDetail](ctx, c, "/customformat/"+itoa(id))
	if err != nil {
		return CustomFormatDetail{}, err
	}
	return raw.toDetail(), nil
}

// CustomFormatCreate describes a new custom format.
type CustomFormatCreate struct {
	// Name identifies the format wherever releases are scored.
	Name string
	// IncludeWhenRenaming adds the format's name to renamed files.
	IncludeWhenRenaming bool
	// Specifications are the rules that make the format match.
	Specifications []CustomFormatSpecification
}

// CreateCustomFormat adds a custom format.
func CreateCustomFormat(ctx context.Context, c *Client, in CustomFormatCreate) (CustomFormatDetail, error) {
	if strings.TrimSpace(in.Name) == "" {
		return CustomFormatDetail{}, fmt.Errorf("no custom format name given")
	}
	specs, err := specificationBody(in.Specifications)
	if err != nil {
		return CustomFormatDetail{}, err
	}
	if len(specs) == 0 {
		return CustomFormatDetail{}, fmt.Errorf(
			"no specifications given; a custom format with no rules matches nothing")
	}

	body, err := c.Post(ctx, "/customformat", map[string]any{
		"name":                            in.Name,
		"includeCustomFormatWhenRenaming": in.IncludeWhenRenaming,
		"specifications":                  specs,
	})
	if err != nil {
		return CustomFormatDetail{}, err
	}
	var raw rawCustomFormatDetail
	if err := unmarshal(body, &raw); err != nil {
		return CustomFormatDetail{}, err
	}
	return raw.toDetail(), nil
}

// CustomFormatUpdate changes one custom format. Omitting a field leaves it as
// it was; giving Specifications replaces the rule set outright, because two
// rule lists cannot be merged without guessing which rules correspond.
type CustomFormatUpdate struct {
	// ID is the format to change.
	ID int
	// Name renames the format.
	Name *string
	// IncludeWhenRenaming adds the format's name to renamed files.
	IncludeWhenRenaming *bool
	// Specifications replaces the whole rule set.
	Specifications []CustomFormatSpecification
}

// UpdateCustomFormat changes a custom format in place, reading the stored
// record first so the parts that were not named survive the write.
func UpdateCustomFormat(ctx context.Context, c *Client, in CustomFormatUpdate) (CustomFormatDetail, error) {
	path := "/customformat/" + itoa(in.ID)
	format, err := GetJSON[map[string]any](ctx, c, path)
	if err != nil {
		return CustomFormatDetail{}, err
	}

	if in.Name != nil {
		format["name"] = *in.Name
	}
	if in.IncludeWhenRenaming != nil {
		format["includeCustomFormatWhenRenaming"] = *in.IncludeWhenRenaming
	}
	if len(in.Specifications) > 0 {
		specs, err := specificationBody(in.Specifications)
		if err != nil {
			return CustomFormatDetail{}, err
		}
		format["specifications"] = specs
	}

	body, err := c.Put(ctx, path, format)
	if err != nil {
		return CustomFormatDetail{}, err
	}
	var raw rawCustomFormatDetail
	if err := unmarshal(body, &raw); err != nil {
		return CustomFormatDetail{}, err
	}
	return raw.toDetail(), nil
}

// DeleteCustomFormat removes a custom format. Every quality profile scoring it
// loses that score.
func DeleteCustomFormat(ctx context.Context, c *Client, id int) error {
	_, err := c.Delete(ctx, "/customformat/"+itoa(id))
	return err
}

// specificationBody renders matching rules the way the service returns them:
// each rule's settings go out as an array of named fields, which is the shape
// both /customformat/{id} and /customformat/schema come back in.
func specificationBody(specs []CustomFormatSpecification) ([]map[string]any, error) {
	out := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		if strings.TrimSpace(spec.Name) == "" {
			return nil, fmt.Errorf("a specification was given with no name")
		}
		if strings.TrimSpace(spec.Implementation) == "" {
			return nil, fmt.Errorf(
				"specification %q has no implementation; name the rule type, e.g. ReleaseTitleSpecification", spec.Name)
		}
		fields := make([]map[string]any, 0, len(spec.Fields))
		for _, key := range sortedKeys(spec.Fields) {
			fields = append(fields, map[string]any{"name": key, "value": spec.Fields[key]})
		}
		out = append(out, map[string]any{
			"name":           spec.Name,
			"implementation": spec.Implementation,
			"negate":         spec.Negate,
			"required":       spec.Required,
			"fields":         fields,
		})
	}
	return out, nil
}

// --- root folders ---

// AddRootFolder registers a library path with the service. The path is the
// service's own view of the filesystem, which in a container is the path inside
// the container rather than on the host.
func AddRootFolder(ctx context.Context, c *Client, path string) (RootFolder, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return RootFolder{}, fmt.Errorf("no root folder path given")
	}
	body, err := c.Post(ctx, "/rootfolder", map[string]any{"path": path})
	if err != nil {
		return RootFolder{}, err
	}
	var out RootFolder
	if err := unmarshal(body, &out); err != nil {
		return RootFolder{}, err
	}
	return out, nil
}

// DeleteRootFolder unregisters a library path. The files on disk are untouched;
// what the service already imported from the folder stays in the library.
func DeleteRootFolder(ctx context.Context, c *Client, id int) error {
	_, err := c.Delete(ctx, "/rootfolder/"+itoa(id))
	return err
}

// --- naming configuration ---

// NamingConfigUpdate changes the file and folder naming policy. Every field is
// optional so an omitted one leaves that format string alone, and the
// service-specific ones are refused rather than invented on the instance that
// does not have them.
type NamingConfigUpdate struct {
	// RenameFiles turns renaming on or off. It lands on renameEpisodes or
	// renameMovies, whichever the instance has.
	RenameFiles *bool
	// ReplaceIllegalCharacters substitutes characters the filesystem rejects.
	ReplaceIllegalCharacters *bool

	StandardEpisodeFormat *string
	DailyEpisodeFormat    *string
	AnimeEpisodeFormat    *string
	SeriesFolderFormat    *string
	SeasonFolderFormat    *string
	SpecialsFolderFormat  *string
	StandardMovieFormat   *string
	MovieFolderFormat     *string
}

// UpdateNamingConfig changes the naming policy, reading it first so the
// settings this package does not model survive the write. colonReplacementFormat
// is the reason that matters: Sonarr sends it as an integer and Radarr as a
// string, so a typed round trip would have to drop it.
func UpdateNamingConfig(ctx context.Context, c *Client, in NamingConfigUpdate) (NamingConfig, error) {
	current, err := GetJSON[map[string]any](ctx, c, "/config/naming")
	if err != nil {
		return NamingConfig{}, err
	}

	edit := configEdit{config: current, resource: "naming"}
	if in.RenameFiles != nil {
		edit.setEither(*in.RenameFiles, "renameEpisodes", "renameMovies")
	}
	edit.setBool("replaceIllegalCharacters", in.ReplaceIllegalCharacters)
	edit.setString("standardEpisodeFormat", in.StandardEpisodeFormat)
	edit.setString("dailyEpisodeFormat", in.DailyEpisodeFormat)
	edit.setString("animeEpisodeFormat", in.AnimeEpisodeFormat)
	edit.setString("seriesFolderFormat", in.SeriesFolderFormat)
	edit.setString("seasonFolderFormat", in.SeasonFolderFormat)
	edit.setString("specialsFolderFormat", in.SpecialsFolderFormat)
	edit.setString("standardMovieFormat", in.StandardMovieFormat)
	edit.setString("movieFolderFormat", in.MovieFolderFormat)
	if err := edit.err(c); err != nil {
		return NamingConfig{}, err
	}

	body, err := c.Put(ctx, configPath("/config/naming", current), current)
	if err != nil {
		return NamingConfig{}, err
	}
	var out NamingConfig
	if err := unmarshal(body, &out); err != nil {
		return NamingConfig{}, err
	}
	return out, nil
}

// --- media management configuration ---

// MediaManagementConfig is the file handling policy: what happens to imported
// files, deleted files and empty folders. The Sonarr-only and Radarr-only
// fields are both present and omitted when empty, because the two services
// describe the same settings with different names.
type MediaManagementConfig struct {
	ID int `json:"id"`

	AutoUnmonitorPreviouslyDownloadedEpisodes bool   `json:"autoUnmonitorPreviouslyDownloadedEpisodes,omitempty" jsonschema:"Sonarr only"`
	AutoUnmonitorPreviouslyDownloadedMovies   bool   `json:"autoUnmonitorPreviouslyDownloadedMovies,omitempty" jsonschema:"Radarr only"`
	CreateEmptySeriesFolders                  bool   `json:"createEmptySeriesFolders,omitempty" jsonschema:"Sonarr only"`
	CreateEmptyMovieFolders                   bool   `json:"createEmptyMovieFolders,omitempty" jsonschema:"Radarr only"`
	EpisodeTitleRequired                      string `json:"episodeTitleRequired,omitempty" jsonschema:"Sonarr only; always, bulkSeasonReleases or never"`
	AutoRenameFolders                         bool   `json:"autoRenameFolders,omitempty" jsonschema:"Radarr only"`

	RecycleBin                      string `json:"recycleBin,omitempty" jsonschema:"deleted files are moved here; empty means they are removed outright"`
	RecycleBinCleanupDays           int    `json:"recycleBinCleanupDays,omitempty"`
	DownloadPropersAndRepacks       string `json:"downloadPropersAndRepacks,omitempty" jsonschema:"preferAndUpgrade, doNotUpgrade or doNotPrefer"`
	DeleteEmptyFolders              bool   `json:"deleteEmptyFolders"`
	FileDate                        string `json:"fileDate,omitempty"`
	RescanAfterRefresh              string `json:"rescanAfterRefresh,omitempty" jsonschema:"always, afterManual or never"`
	SkipFreeSpaceCheckWhenImporting bool   `json:"skipFreeSpaceCheckWhenImporting"`
	MinimumFreeSpaceWhenImporting   int    `json:"minimumFreeSpaceWhenImporting,omitempty" jsonschema:"megabytes that must stay free"`
	CopyUsingHardlinks              bool   `json:"copyUsingHardlinks" jsonschema:"hardlink instead of copying, so seeding torrents cost no extra space"`
	ImportExtraFiles                bool   `json:"importExtraFiles"`
	ExtraFileExtensions             string `json:"extraFileExtensions,omitempty" jsonschema:"comma-separated, e.g. srt,nfo"`
	EnableMediaInfo                 bool   `json:"enableMediaInfo"`
	SetPermissionsLinux             bool   `json:"setPermissionsLinux"`
	ChmodFolder                     string `json:"chmodFolder,omitempty"`
	ChownGroup                      string `json:"chownGroup,omitempty"`
}

// GetMediaManagementConfig returns the file handling policy.
func GetMediaManagementConfig(ctx context.Context, c *Client) (MediaManagementConfig, error) {
	return GetJSON[MediaManagementConfig](ctx, c, "/config/mediamanagement")
}

// MediaManagementConfigUpdate changes the file handling policy. Every field is
// optional so an omitted one leaves that setting alone.
type MediaManagementConfigUpdate struct {
	// AutoUnmonitorPreviouslyDownloaded stops monitoring media whose files are
	// deleted. It lands on whichever of the two names the instance uses.
	AutoUnmonitorPreviouslyDownloaded *bool
	// CreateEmptyMediaFolders creates a folder for media with no files yet. It
	// lands on whichever of the two names the instance uses.
	CreateEmptyMediaFolders *bool

	RecycleBin                      *string
	RecycleBinCleanupDays           *int
	DownloadPropersAndRepacks       *string
	DeleteEmptyFolders              *bool
	FileDate                        *string
	RescanAfterRefresh              *string
	SkipFreeSpaceCheckWhenImporting *bool
	MinimumFreeSpaceWhenImporting   *int
	CopyUsingHardlinks              *bool
	ImportExtraFiles                *bool
	ExtraFileExtensions             *string
	EnableMediaInfo                 *bool
}

// UpdateMediaManagementConfig changes the file handling policy, reading it
// first so the settings this package does not model survive the write.
func UpdateMediaManagementConfig(ctx context.Context, c *Client, in MediaManagementConfigUpdate) (MediaManagementConfig, error) {
	current, err := GetJSON[map[string]any](ctx, c, "/config/mediamanagement")
	if err != nil {
		return MediaManagementConfig{}, err
	}

	edit := configEdit{config: current, resource: "media management"}
	if in.AutoUnmonitorPreviouslyDownloaded != nil {
		edit.setEither(*in.AutoUnmonitorPreviouslyDownloaded,
			"autoUnmonitorPreviouslyDownloadedEpisodes", "autoUnmonitorPreviouslyDownloadedMovies")
	}
	if in.CreateEmptyMediaFolders != nil {
		edit.setEither(*in.CreateEmptyMediaFolders, "createEmptySeriesFolders", "createEmptyMovieFolders")
	}
	edit.setString("recycleBin", in.RecycleBin)
	edit.setInt("recycleBinCleanupDays", in.RecycleBinCleanupDays)
	edit.setString("downloadPropersAndRepacks", in.DownloadPropersAndRepacks)
	edit.setBool("deleteEmptyFolders", in.DeleteEmptyFolders)
	edit.setString("fileDate", in.FileDate)
	edit.setString("rescanAfterRefresh", in.RescanAfterRefresh)
	edit.setBool("skipFreeSpaceCheckWhenImporting", in.SkipFreeSpaceCheckWhenImporting)
	edit.setInt("minimumFreeSpaceWhenImporting", in.MinimumFreeSpaceWhenImporting)
	edit.setBool("copyUsingHardlinks", in.CopyUsingHardlinks)
	edit.setBool("importExtraFiles", in.ImportExtraFiles)
	edit.setString("extraFileExtensions", in.ExtraFileExtensions)
	edit.setBool("enableMediaInfo", in.EnableMediaInfo)
	if err := edit.err(c); err != nil {
		return MediaManagementConfig{}, err
	}

	body, err := c.Put(ctx, configPath("/config/mediamanagement", current), current)
	if err != nil {
		return MediaManagementConfig{}, err
	}
	var out MediaManagementConfig
	if err := unmarshal(body, &out); err != nil {
		return MediaManagementConfig{}, err
	}
	return out, nil
}

// configEdit applies optional changes to a decoded configuration resource,
// refusing any setting the instance does not already have. Inventing a key
// would look like it worked and change nothing, which is worse than an error
// naming the setting.
type configEdit struct {
	config   map[string]any
	resource string
	missing  []string
}

// set writes one key when the config has it, and records it when it does not.
func (e *configEdit) set(key string, value any) {
	if _, ok := e.config[key]; !ok {
		e.missing = append(e.missing, key)
		return
	}
	e.config[key] = value
}

// setEither writes whichever of two alternative names the config uses, so one
// argument works on both services.
func (e *configEdit) setEither(value any, keys ...string) {
	for _, key := range keys {
		if _, ok := e.config[key]; ok {
			e.config[key] = value
			return
		}
	}
	e.missing = append(e.missing, strings.Join(keys, " or "))
}

// setString writes a string setting when one was given.
func (e *configEdit) setString(key string, value *string) {
	if value != nil {
		e.set(key, *value)
	}
}

// setInt writes an integer setting when one was given.
func (e *configEdit) setInt(key string, value *int) {
	if value != nil {
		e.set(key, *value)
	}
}

// setBool writes a boolean setting when one was given.
func (e *configEdit) setBool(key string, value *bool) {
	if value != nil {
		e.set(key, *value)
	}
}

// err reports the settings the instance does not have, so the caller learns
// which argument to drop rather than that the call failed.
func (e *configEdit) err(c *Client) error {
	if len(e.missing) == 0 {
		return nil
	}
	return fmt.Errorf("this %s instance has no %s setting called %s",
		c.spec.Name, e.resource, strings.Join(e.missing, ", "))
}

// configPath appends the record's own id when it has one. The configuration
// controllers expose the update under /{id}, alongside the collection route the
// read uses.
func configPath(base string, config map[string]any) string {
	if id, ok := config["id"].(float64); ok {
		return base + "/" + itoa(int(id))
	}
	return base
}

// --- delay profiles ---

// delayProtocols are the values preferredProtocol accepts.
var delayProtocols = []string{"usenet", "torrent"}

// DelayProfileUpdate changes one delay profile. Every field is optional so an
// omitted one leaves that setting alone.
type DelayProfileUpdate struct {
	// ID is the profile to change.
	ID int
	// PreferredProtocol is usenet or torrent.
	PreferredProtocol *string
	// UsenetDelay is how many minutes to hold a usenet release.
	UsenetDelay *int
	// TorrentDelay is how many minutes to hold a torrent.
	TorrentDelay *int
	// EnableUsenet allows usenet releases at all.
	EnableUsenet *bool
	// EnableTorrent allows torrents at all.
	EnableTorrent *bool
	// BypassIfHighestQuality grabs immediately when nothing better can arrive.
	BypassIfHighestQuality *bool
}

// UpdateDelayProfile changes a delay profile in place, reading the stored
// record first so its order and tags survive the write.
func UpdateDelayProfile(ctx context.Context, c *Client, in DelayProfileUpdate) (DelayProfile, error) {
	if in.PreferredProtocol != nil {
		protocol := strings.ToLower(strings.TrimSpace(*in.PreferredProtocol))
		if !oneOf(delayProtocols, protocol) {
			return DelayProfile{}, fmt.Errorf("unknown protocol %q; valid protocols: %s",
				*in.PreferredProtocol, strings.Join(delayProtocols, ", "))
		}
		in.PreferredProtocol = &protocol
	}

	path := "/delayprofile/" + itoa(in.ID)
	profile, err := GetJSON[map[string]any](ctx, c, path)
	if err != nil {
		return DelayProfile{}, err
	}

	if in.PreferredProtocol != nil {
		profile["preferredProtocol"] = *in.PreferredProtocol
	}
	if in.UsenetDelay != nil {
		profile["usenetDelay"] = *in.UsenetDelay
	}
	if in.TorrentDelay != nil {
		profile["torrentDelay"] = *in.TorrentDelay
	}
	if in.EnableUsenet != nil {
		profile["enableUsenet"] = *in.EnableUsenet
	}
	if in.EnableTorrent != nil {
		profile["enableTorrent"] = *in.EnableTorrent
	}
	if in.BypassIfHighestQuality != nil {
		profile["bypassIfHighestQuality"] = *in.BypassIfHighestQuality
	}

	body, err := c.Put(ctx, path, profile)
	if err != nil {
		return DelayProfile{}, err
	}
	var out DelayProfile
	if err := unmarshal(body, &out); err != nil {
		return DelayProfile{}, err
	}
	return out, nil
}

// --- release profiles ---

// ReleaseProfileCreate describes a new release profile.
type ReleaseProfileCreate struct {
	// Name identifies the profile.
	Name string
	// Enabled turns the profile on; a new profile is on unless this says not.
	Enabled *bool
	// Required are terms a release title must contain.
	Required []string
	// Ignored are terms that reject a release.
	Ignored []string
	// IndexerID limits the profile to one indexer; 0 means all of them.
	IndexerID *int
	// Tags limit the profile to the series carrying them.
	Tags []int
}

// CreateReleaseProfile adds a release profile.
//
// The body mirrors what the shipped web UI seeds a new profile with -- enabled,
// with empty term and tag arrays and indexerId 0 -- because the service rejects
// a null where it expects a list.
func CreateReleaseProfile(ctx context.Context, c *Client, in ReleaseProfileCreate) (ReleaseProfile, error) {
	if len(in.Required) == 0 && len(in.Ignored) == 0 {
		return ReleaseProfile{}, fmt.Errorf(
			"no terms given; a release profile needs required or ignored terms to do anything")
	}

	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	indexer := 0
	if in.IndexerID != nil {
		indexer = *in.IndexerID
	}

	body, err := c.Post(ctx, "/releaseprofile", map[string]any{
		"name":      in.Name,
		"enabled":   enabled,
		"required":  terms(in.Required),
		"ignored":   terms(in.Ignored),
		"indexerId": indexer,
		"tags":      ids(in.Tags),
	})
	if err != nil {
		return ReleaseProfile{}, err
	}
	return decodeReleaseProfile(body)
}

// ReleaseProfileUpdate changes one release profile. Every field is optional so
// an omitted one leaves that setting alone.
type ReleaseProfileUpdate struct {
	// ID is the profile to change.
	ID int
	// Name renames the profile.
	Name *string
	// Enabled turns the profile on or off.
	Enabled *bool
	// Required replaces the terms a release title must contain.
	Required []string
	// Ignored replaces the terms that reject a release.
	Ignored []string
	// IndexerID limits the profile to one indexer; 0 means all of them.
	IndexerID *int
	// Tags replace the tags the profile is limited to.
	Tags []int
}

// UpdateReleaseProfile changes a release profile in place, reading the stored
// record first so the terms that were not replaced survive the write.
func UpdateReleaseProfile(ctx context.Context, c *Client, in ReleaseProfileUpdate) (ReleaseProfile, error) {
	path := "/releaseprofile/" + itoa(in.ID)
	profile, err := GetJSON[map[string]any](ctx, c, path)
	if err != nil {
		return ReleaseProfile{}, err
	}

	if in.Name != nil {
		profile["name"] = *in.Name
	}
	if in.Enabled != nil {
		profile["enabled"] = *in.Enabled
	}
	if in.Required != nil {
		profile["required"] = terms(in.Required)
	}
	if in.Ignored != nil {
		profile["ignored"] = terms(in.Ignored)
	}
	if in.IndexerID != nil {
		profile["indexerId"] = *in.IndexerID
	}
	if in.Tags != nil {
		profile["tags"] = ids(in.Tags)
	}

	body, err := c.Put(ctx, path, profile)
	if err != nil {
		return ReleaseProfile{}, err
	}
	return decodeReleaseProfile(body)
}

// DeleteReleaseProfile removes a release profile, so its terms stop filtering
// releases.
func DeleteReleaseProfile(ctx context.Context, c *Client, id int) error {
	_, err := c.Delete(ctx, "/releaseprofile/"+itoa(id))
	return err
}

// decodeReleaseProfile projects the resource a write returns.
func decodeReleaseProfile(body []byte) (ReleaseProfile, error) {
	var raw rawReleaseProfile
	if err := unmarshal(body, &raw); err != nil {
		return ReleaseProfile{}, err
	}
	return ReleaseProfile{
		ID: raw.ID, Name: raw.Name, Enabled: raw.Enabled,
		Required: raw.Required, Ignored: raw.Ignored,
		IndexerID: raw.IndexerID, Tags: raw.Tags,
	}, nil
}

// terms renders a term list as an array, never null: the service rejects a null
// where it expects a list.
func terms(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

// ids renders an id list as an array, never null.
func ids(list []int) []int {
	if list == nil {
		return []int{}
	}
	return list
}
