package arr

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

// getinfoEntry renders one File Station entry as getinfo answers it.
func getinfoEntry(path, real, typ, mount string) string {
	return fmt.Sprintf(`{"path":%q,"name":"n","isdir":true,"additional":{"real_path":%q,"type":%q,"mount_point_type":%q}}`, path, real, typ, mount)
}

// scriptGetInfo makes List getinfo answer from entry, which gets each asked path.
func scriptGetInfo(f *dsmFake, entry func(path string) string) {
	f.handlers["SYNO.FileStation.List/getinfo"] = func(_ int, form url.Values) string {
		var paths []string
		_ = json.Unmarshal([]byte(form.Get("path")), &paths)
		parts := make([]string, 0, len(paths))
		for _, p := range paths {
			parts = append(parts, entry(p))
		}
		return `{"success":true,"data":{"files":[` + strings.Join(parts, ",") + `]}}`
	}
}

// missingEntry is how getinfo answers a path that does not exist (code 408,
// observed live), with the path echoed as asked.
func missingEntry(path string) string { return fmt.Sprintf(`{"path":%q,"code":408}`, path) }

// identityGetInfo resolves every path to itself below prefix, as DSM does when
// nothing is a link, except the paths marked absent with f.absent.
func identityGetInfo(f *dsmFake, prefix string) {
	scriptGetInfo(f, func(p string) string {
		if f.absent[p] {
			return missingEntry(p)
		}
		return getinfoEntry(p, prefix+p, "", "")
	})
}

// okEntry is a plain, link-free answer for p, with the three paths the
// writeCalls tools would create or land on reported as not existing.
func okEntry(p string) string {
	switch p {
	case "/NAS/a/x", "/NAS/b", "/NAS/d/a":
		return missingEntry(p)
	}
	return getinfoEntry(p, "/volume1"+p, "", "")
}

// newShareDSM is a bare DSM that can list shares; the test scripts getinfo.
func newShareDSM(t *testing.T) *dsmFake {
	t.Helper()
	f := newDSM(t)
	f.absent = map[string]bool{}
	shareList(f, "/volume1")
	return f
}

// shareList answers list_share with shares NAS, kube and docker below prefix.
func shareList(f *dsmFake, prefix string) {
	var parts []string
	for _, n := range []string{"docker", "kube", "NAS"} {
		parts = append(parts, fmt.Sprintf(`{"name":%q,"path":"/%s","additional":{"real_path":%q}}`, n, n, prefix+"/"+n))
	}
	f.on("SYNO.FileStation.List/list_share", `{"success":true,"data":{"total":3,"shares":[`+strings.Join(parts, ",")+`]}}`)
}

// newWriteDSM is a NAS with no links; absent names the paths a test's write
// will create or land on.
func newWriteDSM(t *testing.T, absent ...string) *dsmFake {
	t.Helper()
	f := newDSM(t)
	f.absent = map[string]bool{}
	for _, a := range absent {
		f.absent[a] = true
	}
	identityGetInfo(f, "/volume1")
	shareList(f, "/volume1")
	return f
}

// writeOps returns the state-changing calls the fake received.
func writeOps(f *dsmFake) []string {
	var out []string
	for _, k := range f.seen() {
		if strings.HasPrefix(k, "SYNO.FileStation.CreateFolder") || strings.HasPrefix(k, "SYNO.FileStation.Rename") || strings.HasPrefix(k, "SYNO.FileStation.CopyMove") {
			out = append(out, k)
		}
	}
	return out
}

// calls runs every write tool once against f, so each check is proven for all three.
func writeCalls(f *dsmFake) map[string]func() error {
	ctx := context.Background()
	return map[string]func() error{
		"create": func() error { _, err := SynologyCreateFolder(ctx, f.client(), allowNAS, "/NAS/a", "x"); return err },
		"rename": func() error { _, err := SynologyRename(ctx, f.client(), allowNAS, "/NAS/a", "b"); return err },
		"copy": func() error {
			_, err := SynologyCopyMove(ctx, f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/d"})
			return err
		},
	}
}

func refusedBeforeWrite(t *testing.T, f *dsmFake, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("want a refusal, got success")
	}
	if ops := writeOps(f); len(ops) != 0 {
		t.Errorf("write operations were sent: %v", ops)
	}
	mustContain(t, err.Error(), "nothing was changed")
}

func TestSynologyWriteResolvesRealPathsBeforeTheWrite(t *testing.T) {
	fastPolling(t, 5)
	f := newWriteDSM(t, "/NAS/a/x")
	f.on("SYNO.FileStation.CreateFolder/create", `{"success":true,"data":{"folders":[{"isdir":true,"name":"x","path":"/NAS/a/x"}]}}`)
	if err := writeCalls(f)["create"](); err != nil {
		t.Fatalf("create: %v", err)
	}
	seen := f.seen()
	want := []string{"SYNO.FileStation.List/list_share", "SYNO.FileStation.List/getinfo", "SYNO.FileStation.CreateFolder/create"}
	if strings.Join(seen, ",") != strings.Join(want, ",") {
		t.Errorf("calls = %v, want %v", seen, want)
	}
	info := formFor(f, "SYNO.FileStation.List/getinfo")
	if info.Get("path") != `["/NAS","/NAS/a","/NAS/a/x"]` {
		t.Errorf("getinfo path = %q, want the share, the parent and the new folder's path", info.Get("path"))
	}
	for _, want := range []string{"real_path", "type", "mount_point_type"} {
		if !strings.Contains(info.Get("additional"), want) {
			t.Errorf("getinfo additional = %q, lacks %s", info.Get("additional"), want)
		}
	}
}

func TestSynologyWriteRefusesWhenAnExistingPathResolvesElsewhere(t *testing.T) {
	cases := map[string]func(path string) string{
		"into another share": func(p string) string {
			if p == "/NAS/a" || p == "/NAS/d" {
				return getinfoEntry(p, "/volume1/kube"+strings.TrimPrefix(p, "/NAS"), "", "")
			}
			return okEntry(p)
		},
		"back inside the fence": func(p string) string {
			if p == "/NAS/a" || p == "/NAS/d" {
				return getinfoEntry(p, "/volume1/NAS/other", "", "")
			}
			return okEntry(p)
		},
		"a deeper link": func(p string) string {
			if p == "/NAS/a" || p == "/NAS/d" {
				return getinfoEntry(p, "/volume1/NAS/a/extra", "", "")
			}
			return okEntry(p)
		},
		"prefix sibling": func(p string) string {
			if p == "/NAS/a" || p == "/NAS/d" {
				return getinfoEntry(p, "/volume1/NAS2/"+p[5:], "", "")
			}
			return okEntry(p)
		},
	}
	for cname, entry := range cases {
		for tool := range writeCalls(nil) {
			t.Run(cname+"/"+tool, func(t *testing.T) {
				f := newShareDSM(t)
				scriptGetInfo(f, entry)
				err := writeCalls(f)[tool]()
				refusedBeforeWrite(t, f, err)
				mustContain(t, err.Error(), "resolves outside its expected location")
				mustNotContain(t, err.Error(), "/volume1/kube", "/volume1/NAS2")
			})
		}
	}
}

func TestSynologyCopyMoveChecksEverySourceAndTheDestinationSeparately(t *testing.T) {
	for _, bad := range []string{"/NAS/s1", "/NAS/s2", "/NAS/d"} {
		f := newShareDSM(t)
		scriptGetInfo(f, func(p string) string {
			if p == bad {
				return getinfoEntry(p, "/volume1/kube/x", "", "")
			}
			return okEntry(p)
		})
		_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
			SynologyCopyMoveRequest{Sources: []string{"/NAS/s1", "/NAS/s2"}, Destination: "/NAS/d", Move: true})
		refusedBeforeWrite(t, f, err)
		mustContain(t, err.Error(), bad)
	}
}

func TestSynologyWriteRefusesMountsAndLinksByTypeOrMountPoint(t *testing.T) {
	for name, tc := range map[string]struct{ typ, mount string }{
		"remote mount":    {"", "remote"},
		"any mount point": {"", "anything"},
		"link type":       {"link", ""},
		"symlink type":    {"SymLink", ""},
	} {
		for tool := range writeCalls(nil) {
			t.Run(name+"/"+tool, func(t *testing.T) {
				f := newShareDSM(t)
				scriptGetInfo(f, func(p string) string {
					if p == "/NAS/a" || p == "/NAS/d" {
						return getinfoEntry(p, "/volume1"+p, tc.typ, tc.mount)
					}
					return okEntry(p)
				})
				refusedBeforeWrite(t, f, writeCalls(f)[tool]())
			})
		}
	}
}

func TestSynologyWriteRefusesAMountedRoot(t *testing.T) {
	f := newShareDSM(t)
	scriptGetInfo(f, func(p string) string {
		if p == "/NAS" {
			return getinfoEntry(p, "/volume1/NAS", "", "remote")
		}
		return okEntry(p)
	})
	refusedBeforeWrite(t, f, writeCalls(f)["create"]())
}

func TestSynologyWriteFailsClosedWhenTheLookupIsNotUsable(t *testing.T) {
	for name, handler := range map[string]func(int, url.Values) string{
		"dsm error": func(int, url.Values) string { return `{"success":false,"error":{"code":408}}` },
		"no files":  func(int, url.Values) string { return `{"success":true,"data":{"files":[]}}` },
		"no data":   func(int, url.Values) string { return `{"success":true}` },
		"too few": func(int, url.Values) string {
			return `{"success":true,"data":{"files":[` + getinfoEntry("/NAS", "/volume1/NAS", "", "") + `]}}`
		},
		"too many": func(_ int, f url.Values) string {
			return `{"success":true,"data":{"files":[` + strings.Repeat(getinfoEntry("/NAS", "/volume1/NAS", "", "")+",", 3) + getinfoEntry("/NAS", "/volume1/NAS", "", "") + `]}}`
		},
		"entry error": func(int, url.Values) string {
			return `{"success":true,"data":{"files":[{"path":"/NAS","code":408},{"path":"/NAS/a","code":408}]}}`
		},
		"no real path": func(int, url.Values) string {
			return `{"success":true,"data":{"files":[{"path":"/NAS","isdir":true},{"path":"/NAS/a","isdir":true}]}}`
		},
		"undecodable": func(int, url.Values) string { return `not json` },
	} {
		for tool := range writeCalls(nil) {
			t.Run(name+"/"+tool, func(t *testing.T) {
				f := newShareDSM(t)
				f.handlers["SYNO.FileStation.List/getinfo"] = handler
				refusedBeforeWrite(t, f, writeCalls(f)[tool]())
			})
		}
	}
}

// The root's own real path is read, not assumed to be below /volume1.
func TestSynologyWriteUsesTheRootsOwnRealPath(t *testing.T) {
	fastPolling(t, 5)
	f := newShareDSM(t)
	f.absent["/NAS/b"] = true
	identityGetInfo(f, "/volume2")
	shareList(f, "/volume2")
	f.on("SYNO.FileStation.Rename/rename", `{"success":true,"data":{"files":[{"isdir":false,"name":"b","path":"/NAS/a"}]}}`)
	if err := writeCalls(f)["rename"](); err != nil {
		t.Fatalf("rename on /volume2: %v", err)
	}
	// A path that still claims /volume1 while its root is on /volume2 is refused.
	g := newShareDSM(t)
	scriptGetInfo(g, func(p string) string {
		if p == "/NAS" {
			return getinfoEntry(p, "/volume2/NAS", "", "")
		}
		return okEntry(p)
	})
	refusedBeforeWrite(t, g, writeCalls(g)["rename"]())
}

// A fence narrower than a share resolves its own root too.
func TestSynologyWriteResolvesANarrowRootAndTheRemainderBelowIt(t *testing.T) {
	f := newWriteDSM(t, "/NAS/Scratch/a/x")
	f.on("SYNO.FileStation.CreateFolder/create", `{"success":true,"data":{"folders":[{"isdir":true,"name":"x","path":"/NAS/Scratch/a/x"}]}}`)
	if _, err := SynologyCreateFolder(context.Background(), f.client(), []string{"/NAS/Scratch"}, "/NAS/Scratch/a", "x"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p := formFor(f, "SYNO.FileStation.List/getinfo").Get("path"); p != `["/NAS","/NAS/Scratch","/NAS/Scratch/a","/NAS/Scratch/a/x"]` {
		t.Errorf("getinfo path = %q", p)
	}
	// The root being a link makes everything under it look wrong.
	g := newShareDSM(t)
	scriptGetInfo(g, func(p string) string {
		if p == "/NAS/Scratch" {
			return getinfoEntry(p, "/volume1/kube", "", "")
		}
		return okEntry(p)
	})
	_, err := SynologyCreateFolder(context.Background(), g.client(), []string{"/NAS/Scratch"}, "/NAS/Scratch/a", "x")
	refusedBeforeWrite(t, g, err)
}

func TestSynologyWriteTextualRefusalStillSendsNothingAtAll(t *testing.T) {
	f := newWriteDSM(t)
	_, err := SynologyCreateFolder(context.Background(), f.client(), allowNAS, "/kube", "x")
	refused(t, f, err)
}

// formFor returns the form of the first request for api/method.
func formFor(f *dsmFake, key string) url.Values {
	for _, r := range f.requests() {
		if r.key() == key {
			return r.form
		}
	}
	return url.Values{}
}
