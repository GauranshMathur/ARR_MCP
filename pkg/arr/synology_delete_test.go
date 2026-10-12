package arr

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"testing"
)

const (
	delRunning  = `{"success":true,"data":{"finished":false,"processed_num":0,"total":0,"path":"/NAS/a","processing_path":"/NAS/a/f1"}}`
	delFinished = `{"success":true,"data":{"finished":true,"processed_num":4,"total":4,"path":"/NAS/a","processing_path":"/NAS/a/f4"}}`
)

func deleteStartsOf(f *dsmFake) int {
	n := 0
	for _, k := range f.seen() {
		if k == "SYNO.FileStation.Delete/start" {
			n++
		}
	}
	return n
}

// deleteFake is a NAS with no links whose delete task answers statuses in turn.
func deleteFake(t *testing.T, statuses ...string) *dsmFake {
	t.Helper()
	f := newWriteDSM(t, "/NAS/gone")
	f.on("SYNO.FileStation.Delete/start", `{"success":true,"data":{"taskid":"FileStation_DEL"}}`)
	f.handlers["SYNO.FileStation.Delete/status"] = func(n int, _ url.Values) string {
		return statuses[min(n-1, len(statuses)-1)]
	}
	afterDelete(f)
	return f
}

// afterDelete scripts getinfo as the NAS answers it: every path is there until
// the delete start arrives, and then missing (408, observed live) except the
// paths in stay, which DSM left in place.
func afterDelete(f *dsmFake, stay ...string) {
	scriptGetInfo(f, func(p string) string {
		if f.calls["SYNO.FileStation.Delete/start"] > 0 && !slices.Contains(stay, p) {
			return missingEntry(p)
		}
		return getinfoEntry(p, "/volume1"+p, "", "")
	})
}

func del(f *dsmFake, allowed []string, recursive bool, paths ...string) (SynologyDeleteResult, error) {
	return SynologyDelete(context.Background(), f.client(), allowed, SynologyDeleteRequest{Paths: paths, Recursive: recursive})
}

// noStart asserts that no delete start request reached the NAS.
func noStart(t *testing.T, f *dsmFake, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want a refusal, got success")
	}
	if n := deleteStartsOf(f); n != 0 {
		t.Errorf("%d delete start requests were sent, want none", n)
	}
}

func TestSynologyDeleteOpsAreExactlyTheThreeDeleteCalls(t *testing.T) {
	for _, op := range []synoOp{synoDeleteStart, synoDeleteState, synoDeleteStop} {
		if op.api != "SYNO.FileStation.Delete" || op.version != 2 || !synoWriteOps[op] {
			t.Errorf("%+v is not an allowed Delete v2 write op", op)
		}
	}
	if synoDeleteStart.method != "start" || synoDeleteState.method != "status" || synoDeleteStop.method != "stop" {
		t.Errorf("methods = %s %s %s", synoDeleteStart.method, synoDeleteState.method, synoDeleteStop.method)
	}
	n := 0
	for op := range synoWriteOps {
		if op.api == "SYNO.FileStation.Delete" {
			n++
		}
	}
	if n != 3 {
		t.Errorf("synoWriteOps holds %d Delete ops, want exactly 3", n)
	}
}

// Guards 1 to 5: every refusal sends nothing at all, not even a login.
func TestSynologyDeleteRefusalsSendNothing(t *testing.T) {
	eleven := make([]string, 11)
	for i := range eleven {
		eleven[i] = fmt.Sprintf("/NAS/Scratch/f%d", i)
	}
	for name, tc := range map[string]struct {
		allowed []string
		paths   []string
	}{
		"no allowedPaths":               {nil, []string{"/NAS/a"}},
		"empty allowedPaths":            {[]string{}, []string{"/NAS/a"}},
		"allowed path itself":           {[]string{"/NAS"}, []string{"/NAS"}},
		"narrow allowed path itself":    {[]string{"/NAS/Scratch"}, []string{"/NAS/Scratch"}},
		"allowed path with slash":       {[]string{"/NAS"}, []string{"/NAS/"}},
		"outside":                       {[]string{"/NAS"}, []string{"/kube/a"}},
		"prefix sibling":                {[]string{"/NAS"}, []string{"/NAS2/a"}},
		"traversal":                     {[]string{"/NAS"}, []string{"/NAS/../kube"}},
		"case differs":                  {[]string{"/NAS"}, []string{"/nas/a"}},
		"parent of a narrow root":       {[]string{"/NAS/Scratch"}, []string{"/NAS/Other"}},
		"share root, a narrower fence":  {[]string{"/NAS/Scratch"}, []string{"/NAS"}},
		"recycle bin":                   {[]string{"/NAS"}, []string{"/NAS/#recycle"}},
		"inside the recycle bin":        {[]string{"/NAS"}, []string{"/NAS/#recycle/old/a.mkv"}},
		"recycle in the middle":         {[]string{"/NAS"}, []string{"/NAS/a/#recycle/b"}},
		"recycle other case":            {[]string{"/NAS"}, []string{"/NAS/#Recycle/b"}},
		"recycle with dot segments":     {[]string{"/NAS"}, []string{"/NAS/./#recycle//b"}},
		"recycle as the allowed root":   {[]string{"/NAS/#recycle"}, []string{"/NAS/#recycle/b"}},
		"one bad among good":            {[]string{"/NAS"}, []string{"/NAS/a", "/NAS/b", "/kube/c"}},
		"recycle last among good":       {[]string{"/NAS"}, []string{"/NAS/a", "/NAS/#recycle/x"}},
		"no paths":                      {[]string{"/NAS"}, nil},
		"empty path":                    {[]string{"/NAS"}, []string{""}},
		"eleven paths":                  {[]string{"/NAS"}, eleven},
		"duplicate":                     {[]string{"/NAS"}, []string{"/NAS/a", "/NAS/a"}},
		"duplicate after cleaning":      {[]string{"/NAS"}, []string{"/NAS/a", "/NAS//a/"}},
		"relative":                      {[]string{"/NAS"}, []string{"NAS/a"}},
		"only slash":                    {[]string{"/NAS"}, []string{"/"}},
		"root allowed by a loose entry": {[]string{"/", "/NAS"}, []string{"/NAS"}},
	} {
		t.Run(name, func(t *testing.T) {
			f := deleteFake(t, delFinished)
			_, err := del(f, tc.allowed, true, tc.paths...)
			refused(t, f, err)
		})
	}
}

func TestSynologyDeleteAcceptsTenPaths(t *testing.T) {
	fastPolling(t, 5)
	ten := make([]string, 10)
	for i := range ten {
		ten[i] = fmt.Sprintf("/NAS/Scratch/f%d", i)
	}
	f := deleteFake(t, delFinished)
	if _, err := del(f, allowNAS, false, ten...); err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

// Guard 2 and 3 on their own, so they hold even if the fence let a path through.
func TestSynologyDeleteRefuseHelper(t *testing.T) {
	for path, ok := range map[string]bool{
		"/NAS":                false,
		"/kube":               false,
		"/NAS/#recycle":       false,
		"/NAS/#recycle/a":     false,
		"/NAS/x/#RECYCLE/a":   false,
		"/NAS/a":              true,
		"/NAS/#recycled":      true,
		"/NAS/recycle":        true,
		"/NAS/a/#recycle.txt": true,
	} {
		err := synoDeleteRefuse(path)
		if (err == nil) != ok {
			t.Errorf("synoDeleteRefuse(%q) = %v, want ok=%v", path, err, ok)
		}
	}
}

// Guard 4: the link check runs on every path in one batch, and one refusal fails
// the whole call with nothing started. Nothing is trusted to be missing: unlike a
// copy's landing entry, a path to delete that DSM says does not exist refuses.
func TestSynologyDeleteLinkCheckRefusesTheWholeBatch(t *testing.T) {
	for name, entry := range map[string]func(p string) string{
		"second path is a link (407)": func(p string) string {
			if p == "/NAS/b" {
				return fmt.Sprintf(`{"path":%q,"code":407}`, p)
			}
			return getinfoEntry(p, "/volume1"+p, "", "")
		},
		"second path is missing (408)": func(p string) string {
			if p == "/NAS/b" {
				return missingEntry(p)
			}
			return getinfoEntry(p, "/volume1"+p, "", "")
		},
		"second path resolves elsewhere": func(p string) string {
			if p == "/NAS/b" {
				return getinfoEntry(p, "/volume1/kube/b", "", "")
			}
			return getinfoEntry(p, "/volume1"+p, "", "")
		},
		"a folder above is a link": func(p string) string {
			if p == "/NAS/b/c" {
				return getinfoEntry(p, "/volume1/kube/c", "", "")
			}
			return getinfoEntry(p, "/volume1"+p, "", "")
		},
		"typed as a link": func(p string) string {
			if p == "/NAS/b" {
				return getinfoEntry(p, "/volume1"+p, "symlink", "")
			}
			return getinfoEntry(p, "/volume1"+p, "", "")
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := deleteFake(t, delFinished)
			scriptGetInfo(f, entry)
			paths := []string{"/NAS/a", "/NAS/b"}
			if strings.Contains(name, "above") {
				paths = []string{"/NAS/a", "/NAS/b/c"}
			}
			_, err := del(f, allowNAS, true, paths...)
			noStart(t, f, err)
			mustContain(t, err.Error(), "nothing was changed")
			if g := f.calls["SYNO.FileStation.List/getinfo"]; g != 1 {
				t.Errorf("getinfo calls = %d, want one batch", g)
			}
		})
	}
}

func TestSynologyDeleteAsksAboutEveryPathBeforeTheSingleStart(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, delFinished)
	if _, err := del(f, allowNAS, false, "/NAS/a", "/NAS/b/", "/NAS/c, 1.mkv"); err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	var asked []string
	for _, r := range f.requests() {
		if r.key() == "SYNO.FileStation.List/getinfo" {
			asked = append(asked, r.form.Get("path"))
		}
	}
	if len(asked) != 2 {
		t.Fatalf("getinfo requests = %d, want the link check and one confirmation", len(asked))
	}
	if want := `["/NAS/a","/NAS/b","/NAS/c, 1.mkv"]`; asked[1] != want {
		t.Errorf("confirmation asked %s, want %s", asked[1], want)
	}
	for _, p := range []string{`"/NAS/a"`, `"/NAS/b"`, `"/NAS/c, 1.mkv"`} {
		if !strings.Contains(asked[0], p) {
			t.Errorf("getinfo did not ask about %s: %s", p, asked[0])
		}
	}
	want := "SYNO.FileStation.List/list_share,SYNO.FileStation.List/getinfo,SYNO.FileStation.Delete/start,SYNO.FileStation.Delete/status,SYNO.FileStation.List/getinfo,SYNO.FileStation.Delete/stop"
	if g := strings.Join(f.seen(), ","); g != want {
		t.Errorf("calls = %s\n want %s", g, want)
	}
}

// Guard 7: recursive is on the wire every time, false unless the caller asked.
func TestSynologyDeleteStartAlwaysCarriesRecursive(t *testing.T) {
	for _, recursive := range []bool{false, true} {
		t.Run(fmt.Sprint(recursive), func(t *testing.T) {
			fastPolling(t, 5)
			f := deleteFake(t, delRunning, delFinished)
			got, err := del(f, allowNAS, recursive, "/NAS/a, 1.mkv", "/NAS/c/")
			if err != nil {
				t.Fatalf("SynologyDelete: %v", err)
			}
			start := formFor(f, "SYNO.FileStation.Delete/start")
			for k, v := range map[string]string{
				"api": "SYNO.FileStation.Delete", "version": "2", "method": "start",
				"path": `["/NAS/a, 1.mkv","/NAS/c"]`, "recursive": fmt.Sprint(recursive), "accurate_progress": "true",
			} {
				if start.Get(k) != v {
					t.Errorf("start %s = %q, want %q", k, start.Get(k), v)
				}
			}
			if _, ok := start["recursive"]; !ok {
				t.Error("start request has no recursive field")
			}
			mustContain(t, jsonOf(t, got), fmt.Sprintf(`"recursive":%v`, recursive), `"finished":true`, `"partial":false`,
				`"paths":["/NAS/a, 1.mkv","/NAS/c"]`, `"deleted":["/NAS/a, 1.mkv","/NAS/c"]`, `"remaining":[]`)
			mustNotContain(t, jsonOf(t, got), "processed", "total", "processingPath")
			for _, r := range f.requests() {
				if m := r.form.Get("method"); m == "status" || m == "stop" {
					if r.form.Get("taskid") != "FileStation_DEL" || r.form.Get("version") != "2" {
						t.Errorf("%s form = %v", r.key(), r.form)
					}
				}
			}
		})
	}
}

func TestSynologyDeleteNeverUsesTheBlockingMethod(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, delFinished)
	if _, err := del(f, allowNAS, true, "/NAS/a"); err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	for _, k := range f.seen() {
		if k == "SYNO.FileStation.Delete/delete" {
			t.Error("the blocking delete method was called")
		}
	}
}

func TestSynologyDeleteTimeoutIsPartialAndStopsTheTask(t *testing.T) {
	fastPolling(t, 3)
	f := deleteFake(t, delRunning)
	got, err := del(f, allowNAS, true, "/NAS/a")
	if err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"finished":false`, `"partial":true`, "synology_list_files", `"deleted":["/NAS/a"]`, `"remaining":[]`)
	seen := f.seen()
	if seen[len(seen)-1] != "SYNO.FileStation.Delete/stop" {
		t.Errorf("last call = %s, want the task stopped", seen[len(seen)-1])
	}
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

// Guard 6: an ambiguous start is an error that says to look, never a second start.
func TestSynologyDeleteTransportErrorOnStartIsNotRetried(t *testing.T) {
	f := deleteFake(t, delFinished)
	f.handlers["SYNO.FileStation.Delete/start"] = func(int, url.Values) string {
		f.srv.CloseClientConnections() // the answer never arrives
		return ""
	}
	_, err := del(f, allowNAS, true, "/NAS/a")
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "synology_list_files", "not started again")
	mustNotContain(t, err.Error(), dsmTestPass, "SID-")
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want exactly 1", n)
	}
}

func TestSynologyDeleteStartAnswerWithoutATaskIsNotRetried(t *testing.T) {
	f := deleteFake(t, delFinished)
	f.on("SYNO.FileStation.Delete/start", `{"success":true,"data":{}}`)
	_, err := del(f, allowNAS, true, "/NAS/a")
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "synology_list_files", "not started again")
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

func TestSynologyDeleteStartRefusedByDSMIsNotRetried(t *testing.T) {
	f := deleteFake(t, delFinished)
	f.on("SYNO.FileStation.Delete/start", `{"success":false,"error":{"code":408}}`)
	_, err := del(f, allowNAS, true, "/NAS/a")
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "no such file")
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

func TestSynologyDeleteUnknownTaskNeverStartsAgain(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, `{"success":false,"error":{"code":599}}`)
	_, err := del(f, allowNAS, true, "/NAS/a")
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "synology_list_files", "not started again")
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want exactly 1", n)
	}
}

func TestSynologyDeleteStatusErrorSaysSomeFilesMayBeGone(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, delRunning, `{"success":false,"error":{"code":1001}}`)
	_, err := del(f, allowNAS, true, "/NAS/a")
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "code 1001", "synology_list_files", "may already be")
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want exactly 1", n)
	}
	seen := f.seen()
	if seen[len(seen)-1] != "SYNO.FileStation.Delete/stop" {
		t.Errorf("last call = %s, want the task stopped", seen[len(seen)-1])
	}
}

func TestSynologyDeleteResultClaimsNothingBeyondWhatDSMSaid(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, delFinished)
	got, err := del(f, allowNAS, false, "/NAS/a")
	if err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	out := jsonOf(t, got)
	mustNotContain(t, out, "recycle", "permanent", "removed")
	if got.Note != "" {
		t.Errorf("note = %q on a finished delete, want none", got.Note)
	}
}

func TestSynologyDeleteErrorsNeverCarryTheCredentials(t *testing.T) {
	f := deleteFake(t, delFinished)
	f.on("SYNO.FileStation.Delete/start", `{"success":false,"error":{"code":1003}}`)
	_, err := del(f, allowNAS, true, "/NAS/a")
	if err == nil {
		t.Fatal("want an error")
	}
	mustNotContain(t, err.Error(), dsmTestPass, "SID-")
}

// The live case: DSM accepts a non-recursive delete of a non-empty folder, says
// finished with nothing processed, and leaves the folder. That is an error.
func TestSynologyDeleteFinishedButFolderStillThereIsAnError(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, `{"success":true,"data":{"finished":true,"processed_num":0,"total":0,"path":"/NAS/dir","processing_path":""}}`)
	afterDelete(f, "/NAS/dir")
	got, err := del(f, allowNAS, false, "/NAS/dir")
	if err == nil {
		t.Fatalf("want an error, got %+v", got)
	}
	mustContain(t, err.Error(), "/NAS/dir", "still there", "recursive")
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want exactly 1", n)
	}
}

func TestSynologyDeleteMixedBatchErrorNamesBothLists(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, delFinished)
	afterDelete(f, "/NAS/dir")
	_, err := del(f, allowNAS, false, "/NAS/file", "/NAS/dir")
	if err == nil {
		t.Fatal("want an error")
	}
	mustContain(t, err.Error(), "deleted", "/NAS/file", "still there", "/NAS/dir", "recursive")
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want exactly 1", n)
	}
}

func TestSynologyDeleteAllMissingAfterwardsIsSuccessWithDeletedFilled(t *testing.T) {
	fastPolling(t, 5)
	f := deleteFake(t, delFinished)
	got, err := del(f, allowNAS, true, "/NAS/a", "/NAS/b")
	if err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"deleted":["/NAS/a","/NAS/b"]`, `"remaining":[]`, `"finished":true`)
	if n := deleteStartsOf(f); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

// Only a 408 proves a path is gone: another code, a path that is not echoed,
// or a wrong entry count is not confirmation.
func TestSynologyDeleteOnlyA408ConfirmsDeletion(t *testing.T) {
	fastPolling(t, 5)
	for name, entry := range map[string]func(p string) string{
		"other code":  func(p string) string { return fmt.Sprintf(`{"path":%q,"code":407}`, p) },
		"wrong echo":  func(p string) string { return missingEntry(p + "x") },
		"empty entry": func(p string) string { return `{}` },
	} {
		t.Run(name, func(t *testing.T) {
			f := deleteFake(t, delFinished)
			scriptGetInfo(f, func(p string) string {
				if f.calls["SYNO.FileStation.Delete/start"] > 0 {
					return entry(p)
				}
				return getinfoEntry(p, "/volume1"+p, "", "")
			})
			if _, err := del(f, allowNAS, true, "/NAS/a"); err == nil {
				t.Fatal("want an error")
			}
			if n := deleteStartsOf(f); n != 1 {
				t.Errorf("starts = %d, want exactly 1", n)
			}
		})
	}
}

func TestSynologyDeleteConfirmationFailureIsAnErrorThatSaysSo(t *testing.T) {
	fastPolling(t, 5)
	for name, body := range map[string]string{
		"DSM error":   `{"success":false,"error":{"code":1001}}`,
		"wrong count": `{"success":true,"data":{"files":[]}}`,
	} {
		t.Run(name, func(t *testing.T) {
			f := deleteFake(t, delFinished)
			pre := f.handlers["SYNO.FileStation.List/getinfo"]
			f.handlers["SYNO.FileStation.List/getinfo"] = func(n int, form url.Values) string {
				if f.calls["SYNO.FileStation.Delete/start"] > 0 {
					return body
				}
				return pre(n, form)
			}
			_, err := del(f, allowNAS, true, "/NAS/a")
			if err == nil {
				t.Fatal("want an error")
			}
			mustContain(t, err.Error(), "finished", "could not be confirmed", "synology_list_files")
			if n := deleteStartsOf(f); n != 1 {
				t.Errorf("starts = %d, want exactly 1", n)
			}
		})
	}
}

func TestSynologyDeletePartialStillFillsRemainingWhenItCan(t *testing.T) {
	fastPolling(t, 3)
	f := deleteFake(t, delRunning)
	afterDelete(f, "/NAS/b")
	got, err := del(f, allowNAS, true, "/NAS/a", "/NAS/b")
	if err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"partial":true`, `"deleted":["/NAS/a"]`, `"remaining":["/NAS/b"]`, "synology_list_files")
}

func TestSynologyDeletePartialWithoutConfirmationHasEmptyListsNotNull(t *testing.T) {
	fastPolling(t, 3)
	f := deleteFake(t, delRunning)
	pre := f.handlers["SYNO.FileStation.List/getinfo"]
	f.handlers["SYNO.FileStation.List/getinfo"] = func(n int, form url.Values) string {
		if f.calls["SYNO.FileStation.Delete/start"] > 0 {
			return `{"success":false,"error":{"code":1001}}`
		}
		return pre(n, form)
	}
	got, err := del(f, allowNAS, true, "/NAS/a")
	if err != nil {
		t.Fatalf("SynologyDelete: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"partial":true`, `"deleted":[]`, `"remaining":[]`)
}
