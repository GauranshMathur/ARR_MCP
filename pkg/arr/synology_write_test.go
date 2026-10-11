package arr

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

var allowNAS = []string{"/NAS"}

func TestSynologyWriteOpsAreExactlyTheFileStationWrites(t *testing.T) {
	want := map[synoOp]bool{
		{"SYNO.FileStation.CreateFolder", "create", 2}: true,
		{"SYNO.FileStation.Rename", "rename", 2}:       true,
		{"SYNO.FileStation.CopyMove", "start", 3}:      true,
		{"SYNO.FileStation.CopyMove", "status", 3}:     true,
		{"SYNO.FileStation.CopyMove", "stop", 3}:       true,
	}
	if len(synoWriteOps) != len(want) {
		t.Errorf("synoWriteOps has %d entries, want %d: %v", len(synoWriteOps), len(want), synoWriteOps)
	}
	for op := range want {
		if !synoWriteOps[op] {
			t.Errorf("synoWriteOps lacks %+v", op)
		}
	}
	for op := range synoWriteOps {
		for _, banned := range []string{"Delete", "ISCSI", "Core.User", "Core.Group", "Core.Share", "Storage.CGI", "Backup"} {
			if strings.Contains(op.api, banned) {
				t.Errorf("write op %+v touches %s", op, banned)
			}
		}
		if synoReadOps[op] {
			t.Errorf("%+v is in both lists", op)
		}
	}
}

func TestSynologyCallStillRefusesUnlistedOpsNextToTheWriteOps(t *testing.T) {
	f := newWriteDSM(t)
	for _, op := range []synoOp{
		{"SYNO.FileStation.Delete", "start", 2},
		{"SYNO.FileStation.Delete", "delete", 2},
		{"SYNO.FileStation.CopyMove", "start", 2}, // wrong version
		{"SYNO.FileStation.CopyMove", "delete", 3},
		{"SYNO.FileStation.CreateFolder", "create", 1},
		{"SYNO.Core.ISCSI.LUN", "delete", 1},
		{"SYNO.Core.Share", "set", 1},
		{"SYNO.Backup.Task", "backup", 2},
	} {
		if _, err := synoCall[map[string]any](context.Background(), f.client(), op, nil); err == nil {
			t.Errorf("synoCall accepted %+v", op)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests reached the NAS, want none", n)
	}
}

func TestSynologyPathAllowed(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		path    string
		strict  bool
		ok      bool
	}{
		{"empty list refuses", nil, "/NAS/x", false, false},
		{"empty slice refuses", []string{}, "/NAS", false, false},
		{"exact match", []string{"/NAS"}, "/NAS", false, true},
		{"inside", []string{"/NAS"}, "/NAS/Movies/a.mkv", false, true},
		{"exact refused when strict", []string{"/NAS"}, "/NAS", true, false},
		{"inside passes strict", []string{"/NAS"}, "/NAS/x", true, true},
		{"sibling prefix NAS2", []string{"/NAS"}, "/NAS2", false, false},
		{"sibling prefix NAS2 child", []string{"/NAS"}, "/NAS2/x", false, false},
		{"sibling prefix NASx", []string{"/NAS"}, "/NASx/y", false, false},
		{"traversal out", []string{"/NAS"}, "/NAS/../kube", false, false},
		{"traversal inside", []string{"/NAS"}, "/NAS/a/../b", false, false},
		{"trailing slash path", []string{"/NAS"}, "/NAS/", false, true},
		{"trailing slashes inside", []string{"/NAS"}, "/NAS/x//", false, true},
		{"double slash after root", []string{"/NAS"}, "/NAS//x", false, true},
		{"trailing slash in config", []string{"/NAS/"}, "/NAS/x", false, true},
		{"narrow root, parent refused", []string{"/NAS/Scratch"}, "/NAS", false, false},
		{"narrow root, sibling refused", []string{"/NAS/Scratch"}, "/NAS/Other", false, false},
		{"narrow root, prefix sibling", []string{"/NAS/Scratch"}, "/NAS/Scratch2/x", false, false},
		{"narrow root, inside", []string{"/NAS/Scratch"}, "/NAS/Scratch/x", false, true},
		{"second entry matches", []string{"/docker", "/NAS"}, "/NAS/x", false, true},
		{"case differs lower", []string{"/NAS"}, "/nas/x", false, false},
		{"case differs sub", []string{"/NAS/Scratch"}, "/NAS/scratch/x", false, false},
		{"relative", []string{"/NAS"}, "NAS/x", false, false},
		{"root", []string{"/NAS"}, "/", false, false},
		{"backslash", []string{"/NAS"}, `/NAS\x`, false, false},
		{"control char", []string{"/NAS"}, "/NAS/a\nb", false, false},
		{"symlink looking name is just a name", []string{"/NAS"}, "/NAS/link-to-kube", false, true},
		{"invalid config entry never matches", []string{"/", "../NAS", ""}, "/NAS/x", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := synoCheckAllowed(tc.allowed, tc.path, tc.strict)
			if (err == nil) != tc.ok {
				t.Fatalf("synoCheckAllowed(%v, %q, strict=%v) = %q, %v; want ok=%v", tc.allowed, tc.path, tc.strict, got, err, tc.ok)
			}
		})
	}
}

func TestSynologyEmptyAllowedPathsErrorSaysHowToConfigure(t *testing.T) {
	_, err := synoCheckAllowed(nil, "/NAS/x", false)
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "allowedPaths", "config", "/NAS")
}

func TestSynologyOutsideErrorNamesTheFence(t *testing.T) {
	_, err := synoCheckAllowed([]string{"/NAS"}, "/kube/x", false)
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "/kube/x", "allowedPaths", "/NAS")
}

// refused asserts that nothing reached the NAS, not even a login.
func refused(t *testing.T, f *dsmFake, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want a refusal, got success")
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests reached the NAS (logins: %d), want none", n, f.logins)
	}
}

func TestSynologyWritesWithoutAllowedPathsSendNothing(t *testing.T) {
	for name, call := range map[string]func(*dsmFake) error{
		"create": func(f *dsmFake) error {
			_, err := SynologyCreateFolder(context.Background(), f.client(), nil, "/NAS", "x")
			return err
		},
		"rename": func(f *dsmFake) error {
			_, err := SynologyRename(context.Background(), f.client(), nil, "/NAS/a", "b")
			return err
		},
		"copy": func(f *dsmFake) error {
			_, err := SynologyCopyMove(context.Background(), f.client(), nil, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b"})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newWriteDSM(t)
			err := call(f)
			refused(t, f, err)
			mustContain(t, err.Error(), "allowedPaths")
		})
	}
}

func TestSynologyCreateFolderRefusalsSendNothing(t *testing.T) {
	for name, tc := range map[string]struct{ parent, name string }{
		"parent outside":    {"/kube", "x"},
		"sibling prefix":    {"/NAS2", "x"},
		"traversal":         {"/NAS/../kube", "x"},
		"relative":          {"NAS", "x"},
		"empty name":        {"/NAS", ""},
		"blank name":        {"/NAS", "   "},
		"slash in name":     {"/NAS", "a/b"},
		"backslash in name": {"/NAS", `a\b`},
		"dotdot name":       {"/NAS", ".."},
		"dot name":          {"/NAS", "."},
		"escape via name":   {"/NAS", "../kube"},
		"control char name": {"/NAS", "a\x00b"},
	} {
		t.Run(name, func(t *testing.T) {
			f := newWriteDSM(t)
			_, err := SynologyCreateFolder(context.Background(), f.client(), allowNAS, tc.parent, tc.name)
			refused(t, f, err)
		})
	}
}

func TestSynologyCreateFolderSendsTheGuidesParameters(t *testing.T) {
	f := newWriteDSM(t, "/NAS/Scratch")
	f.on("SYNO.FileStation.CreateFolder/create", `{"success":true,"data":{"folders":[{"isdir":true,"name":"Scratch","path":"/NAS/Scratch"}]}}`)
	got, err := SynologyCreateFolder(context.Background(), f.client(), allowNAS, "/NAS/", "Scratch")
	if err != nil {
		t.Fatalf("SynologyCreateFolder: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"path":"/NAS/Scratch"`, `"isDir":true`)
	var form url.Values
	for _, r := range f.requests() {
		if r.key() == "SYNO.FileStation.CreateFolder/create" {
			form = r.form
		}
	}
	for k, v := range map[string]string{
		"version": "2", "folder_path": `["/NAS"]`, "name": `["Scratch"]`, "force_parent": "false",
	} {
		if form.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, form.Get(k), v)
		}
	}
}

func TestSynologyCreateFolderDSMFailureIsAnError(t *testing.T) {
	f := newWriteDSM(t, "/NAS/x")
	f.on("SYNO.FileStation.CreateFolder/create", `{"success":false,"error":{"code":1100}}`)
	_, err := SynologyCreateFolder(context.Background(), f.client(), allowNAS, "/NAS", "x")
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "code 1100")
	mustNotContain(t, err.Error(), "failed to create a folder: unknown")
}

func TestSynologyRenameRefusalsSendNothing(t *testing.T) {
	for name, tc := range map[string]struct{ path, name string }{
		"source outside":   {"/kube/a", "b"},
		"source prefix":    {"/NAS2/a", "b"},
		"source traversal": {"/NAS/../kube/a", "b"},
		"the fence itself": {"/NAS", "b"},
		"empty name":       {"/NAS/a", ""},
		"slash name":       {"/NAS/a", "b/c"},
		"leading slash":    {"/NAS/a", "/kube/b"},
		"dotdot name":      {"/NAS/a", ".."},
		"escape name":      {"/NAS/a", "../b"},
		"backslash name":   {"/NAS/a", `..\b`},
	} {
		t.Run(name, func(t *testing.T) {
			f := newWriteDSM(t)
			_, err := SynologyRename(context.Background(), f.client(), allowNAS, tc.path, tc.name)
			refused(t, f, err)
		})
	}
}

func TestSynologyRenameSendsTheGuidesParameters(t *testing.T) {
	f := newWriteDSM(t, "/NAS/Movies/b, c.mkv")
	f.on("SYNO.FileStation.Rename/rename", `{"success":true,"data":{"files":[{"isdir":false,"name":"b, c.mkv","path":"/NAS/Movies/b, c.mkv"}]}}`)
	got, err := SynologyRename(context.Background(), f.client(), allowNAS, "/NAS/Movies/a.mkv", "b, c.mkv")
	if err != nil {
		t.Fatalf("SynologyRename: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"name":"b, c.mkv"`, `"path":"/NAS/Movies/b, c.mkv"`)
	form := formFor(f, "SYNO.FileStation.Rename/rename")
	for k, v := range map[string]string{
		"api": "SYNO.FileStation.Rename", "version": "2", "method": "rename",
		"path": `["/NAS/Movies/a.mkv"]`, "name": `["b, c.mkv"]`,
	} {
		if form.Get(k) != v {
			t.Errorf("%s = %q, want %q", k, form.Get(k), v)
		}
	}
}

// --- copy / move ---

func copyFake(t *testing.T, statuses ...string) *dsmFake {
	t.Helper()
	f := newWriteDSM(t, "/NAS/b/a", "/NAS/b/a, 1.mkv", "/NAS/b/c", "/NAS/d/a")
	f.on("SYNO.FileStation.CopyMove/start", `{"success":true,"data":{"taskid":"FileStation_ABC"}}`)
	f.handlers["SYNO.FileStation.CopyMove/status"] = func(n int, _ url.Values) string {
		return statuses[min(n-1, len(statuses)-1)]
	}
	return f
}

const (
	cmRunning  = `{"success":true,"data":{"finished":false,"processed_size":10,"total":100,"progress":0.1,"path":"/NAS/a","dest_folder_path":"/NAS/b"}}`
	cmFinished = `{"success":true,"data":{"finished":true,"processed_size":100,"total":100,"progress":1,"path":"/NAS/a","dest_folder_path":"/NAS/b"}}`
)

func startsOf(f *dsmFake) int {
	n := 0
	for _, k := range f.seen() {
		if k == "SYNO.FileStation.CopyMove/start" {
			n++
		}
	}
	return n
}

func TestSynologyCopyMoveRefusalsSendNothing(t *testing.T) {
	for name, req := range map[string]SynologyCopyMoveRequest{
		"destination outside":         {Sources: []string{"/NAS/a"}, Destination: "/kube"},
		"destination prefix sibling":  {Sources: []string{"/NAS/a"}, Destination: "/NAS2"},
		"destination traversal":       {Sources: []string{"/NAS/a"}, Destination: "/NAS/../kube"},
		"source outside":              {Sources: []string{"/kube/a"}, Destination: "/NAS/b"},
		"one of several sources out":  {Sources: []string{"/NAS/a", "/NAS/b", "/docker/c"}, Destination: "/NAS/d"},
		"last source is a prefix sib": {Sources: []string{"/NAS/a", "/NASx/b"}, Destination: "/NAS/d"},
		"source is the fence itself":  {Sources: []string{"/NAS"}, Destination: "/NAS/d"},
		"no sources":                  {Destination: "/NAS/d"},
		"empty source":                {Sources: []string{""}, Destination: "/NAS/d"},
		"empty destination":           {Sources: []string{"/NAS/a"}},
		"move out of fence":           {Sources: []string{"/NAS/a"}, Destination: "/docker", Move: true},
		"traversal in a later source": {Sources: []string{"/NAS/a", "/NAS/x/../../kube"}, Destination: "/NAS/d"},
	} {
		t.Run(name, func(t *testing.T) {
			f := copyFake(t, cmFinished)
			_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, req)
			refused(t, f, err)
		})
	}
}

func TestSynologyCopyMoveDestinationMayBeTheFenceItself(t *testing.T) {
	fastPolling(t, 5)
	f := copyFake(t, cmFinished)
	if _, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS"}); err != nil {
		t.Fatalf("SynologyCopyMove: %v", err)
	}
}

func TestSynologyCopyMoveSendsTheGuidesParametersAndOverwriteIsFalseByDefault(t *testing.T) {
	fastPolling(t, 5)
	f := copyFake(t, cmRunning, cmFinished)
	got, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
		SynologyCopyMoveRequest{Sources: []string{"/NAS/a, 1.mkv", "/NAS/c/"}, Destination: "/NAS/b/"})
	if err != nil {
		t.Fatalf("SynologyCopyMove: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"operation":"copy"`, `"finished":true`, `"partial":false`, `"overwrite":false`, `"destination":"/NAS/b"`)
	want := "SYNO.FileStation.List/list_share,SYNO.FileStation.List/getinfo,SYNO.FileStation.CopyMove/start,SYNO.FileStation.CopyMove/status,SYNO.FileStation.CopyMove/status,SYNO.FileStation.CopyMove/stop"
	if g := strings.Join(f.seen(), ","); g != want {
		t.Errorf("calls = %s\n want %s", g, want)
	}
	start := formFor(f, "SYNO.FileStation.CopyMove/start")
	for k, v := range map[string]string{
		"api": "SYNO.FileStation.CopyMove", "version": "3", "method": "start",
		"path": `["/NAS/a, 1.mkv","/NAS/c"]`, "dest_folder_path": `"/NAS/b"`,
		"overwrite": "false", "remove_src": "false",
	} {
		if start.Get(k) != v {
			t.Errorf("start %s = %q, want %q", k, start.Get(k), v)
		}
	}
	for _, r := range f.requests() {
		if r.form.Get("method") == "status" || r.form.Get("method") == "stop" {
			if r.form.Get("taskid") != "FileStation_ABC" || r.form.Get("version") != "3" {
				t.Errorf("%s form = %v", r.key(), r.form)
			}
		}
	}
}

func TestSynologyCopyMoveMoveAndOverwriteAreExplicit(t *testing.T) {
	fastPolling(t, 5)
	f := copyFake(t, cmFinished)
	got, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
		SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b", Move: true, Overwrite: true})
	if err != nil {
		t.Fatalf("SynologyCopyMove: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"operation":"move"`, `"overwrite":true`)
	start := formFor(f, "SYNO.FileStation.CopyMove/start")
	if start.Get("overwrite") != "true" || start.Get("remove_src") != "true" {
		t.Errorf("overwrite=%q remove_src=%q, want true true", start.Get("overwrite"), start.Get("remove_src"))
	}
}

func TestSynologyCopyMoveTimeoutIsPartialAndStopsTheTask(t *testing.T) {
	fastPolling(t, 3)
	f := copyFake(t, cmRunning)
	got, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
		SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b", Move: true})
	if err != nil {
		t.Fatalf("SynologyCopyMove: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"finished":false`, `"partial":true`, "synology_list_files")
	mustNotContain(t, jsonOf(t, got), "processedBytes", "totalBytes")
	seen := f.seen()
	if seen[len(seen)-1] != "SYNO.FileStation.CopyMove/stop" {
		t.Errorf("last call = %s, want the task stopped", seen[len(seen)-1])
	}
	if startsOf(f) != 1 {
		t.Errorf("starts = %d, want 1", startsOf(f))
	}
}

func TestSynologyCopyMoveCancelledContextStopsTheTask(t *testing.T) {
	fastPolling(t, 50)
	f := copyFake(t, cmRunning)
	ctx, cancel := context.WithCancel(context.Background())
	f.handlers["SYNO.FileStation.CopyMove/status"] = func(n int, _ url.Values) string {
		if n == 2 {
			cancel()
		}
		return cmRunning
	}
	got, err := SynologyCopyMove(ctx, f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b"})
	if err != nil {
		t.Fatalf("SynologyCopyMove: %v", err)
	}
	if !got.Partial {
		t.Errorf("result = %+v, want partial", got)
	}
	seen := f.seen()
	if seen[len(seen)-1] != "SYNO.FileStation.CopyMove/stop" {
		t.Errorf("last call = %s, want stop", seen[len(seen)-1])
	}
}

// A task DSM does not know is ambiguous: it may never have run, or it may have
// finished and been forgotten. Starting it again could copy or move twice, so it
// is an error that says to look, never a second start.
func TestSynologyCopyMoveNeverStartsTwiceWhenTheTaskIsUnknown(t *testing.T) {
	fastPolling(t, 5)
	f := copyFake(t, `{"success":false,"error":{"code":599}}`)
	_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b", Move: true})
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "synology_list_files", "not started again")
	if n := startsOf(f); n != 1 {
		t.Errorf("starts = %d, want exactly 1", n)
	}
}

func TestSynologyCopyMoveNeverStartsTwiceWhenTheStatusFails(t *testing.T) {
	fastPolling(t, 5)
	f := copyFake(t, cmRunning, `{"success":false,"error":{"code":1001}}`)
	_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b", Move: true})
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "code 1001", "synology_list_files")
	if n := startsOf(f); n != 1 {
		t.Errorf("starts = %d, want exactly 1", n)
	}
	seen := f.seen()
	if seen[len(seen)-1] != "SYNO.FileStation.CopyMove/stop" {
		t.Errorf("last call = %s, want the task stopped", seen[len(seen)-1])
	}
}

func TestSynologyCopyMoveStartAnswerWithoutATaskIsNotRetried(t *testing.T) {
	fastPolling(t, 5)
	f := newWriteDSM(t, "/NAS/b/a")
	f.on("SYNO.FileStation.CopyMove/start", `{"success":true,"data":{}}`)
	_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b"})
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "synology_list_files")
	if n := startsOf(f); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

func TestSynologyCopyMoveStartRefusedByDSMIsNotRetried(t *testing.T) {
	f := newWriteDSM(t, "/NAS/b/a")
	f.on("SYNO.FileStation.CopyMove/start", `{"success":false,"error":{"code":408}}`)
	_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b"})
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "no such file")
	if n := startsOf(f); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

func TestSynologyCopyMoveErrorsNeverCarryTheCredentials(t *testing.T) {
	f := newWriteDSM(t, "/NAS/b/a")
	f.on("SYNO.FileStation.CopyMove/start", `{"success":false,"error":{"code":1003}}`)
	_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/b"})
	if err == nil {
		t.Fatal("want an error")
	}
	mustNotContain(t, err.Error(), dsmTestPass, "SID-")
	mustContain(t, err.Error(), "code 1003")
}
