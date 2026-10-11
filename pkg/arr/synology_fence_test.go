package arr

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

// Answers are matched to questions by the path each entry echoes, never by
// position alone. DSM answers in order and echoes the path exactly as asked
// (observed live, also for a path that does not exist); anything else refuses.
func TestSynologyWriteRefusesAnswersThatDoNotEchoTheAskedPaths(t *testing.T) {
	reorder := func(_ int, form url.Values) string {
		var paths []string
		_ = json.Unmarshal([]byte(form.Get("path")), &paths)
		parts := make([]string, len(paths))
		for i, p := range paths {
			parts[len(paths)-1-i] = okEntry(p)
		}
		return `{"success":true,"data":{"files":[` + strings.Join(parts, ",") + `]}}`
	}
	normalised := func(_ int, form url.Values) string {
		var paths []string
		_ = json.Unmarshal([]byte(form.Get("path")), &paths)
		parts := make([]string, len(paths))
		for i, p := range paths {
			e := okEntry(p)
			parts[i] = strings.Replace(e, `"path":"`+p+`"`, `"path":"`+p+`/"`, 1)
		}
		return `{"success":true,"data":{"files":[` + strings.Join(parts, ",") + `]}}`
	}
	noPath := func(_ int, form url.Values) string {
		var paths []string
		_ = json.Unmarshal([]byte(form.Get("path")), &paths)
		parts := make([]string, len(paths))
		for i, p := range paths {
			parts[i] = strings.Replace(okEntry(p), `"path":"`+p+`",`, "", 1)
		}
		return `{"success":true,"data":{"files":[` + strings.Join(parts, ",") + `]}}`
	}
	for name, h := range map[string]func(int, url.Values) string{"reordered": reorder, "normalised": normalised, "no path": noPath} {
		for tool := range writeCalls(nil) {
			t.Run(name+"/"+tool, func(t *testing.T) {
				f := newShareDSM(t)
				f.handlers["SYNO.FileStation.List/getinfo"] = h
				refusedBeforeWrite(t, f, writeCalls(f)[tool]())
			})
		}
	}
}

// The root is anchored to its share: a root (or a folder above it) that is
// itself a link must not make everything below it look consistent.
func TestSynologyWriteRefusesALinkedRootThatItsChildrenAgreeWith(t *testing.T) {
	f := newShareDSM(t)
	scriptGetInfo(f, func(p string) string {
		switch {
		case p == "/NAS/Scratch/a/x":
			return missingEntry(p)
		case p == "/NAS/Scratch" || strings.HasPrefix(p, "/NAS/Scratch/"):
			return getinfoEntry(p, "/volume1/kube/x"+strings.TrimPrefix(p, "/NAS/Scratch"), "", "")
		}
		return getinfoEntry(p, "/volume1"+p, "", "")
	})
	_, err := SynologyCreateFolder(context.Background(), f.client(), []string{"/NAS/Scratch"}, "/NAS/Scratch/a", "x")
	refusedBeforeWrite(t, f, err)
}

func TestSynologyWriteRefusesWhenAFolderAboveTheRootIsALink(t *testing.T) {
	f := newShareDSM(t)
	scriptGetInfo(f, func(p string) string {
		if p == "/NAS/Deep/Scratch/a/x" {
			return missingEntry(p)
		}
		if strings.HasPrefix(p, "/NAS/Deep") {
			return getinfoEntry(p, "/volume1/kube"+strings.TrimPrefix(p, "/NAS/Deep"), "", "")
		}
		return getinfoEntry(p, "/volume1"+p, "", "")
	})
	_, err := SynologyCreateFolder(context.Background(), f.client(), []string{"/NAS/Deep/Scratch"}, "/NAS/Deep/Scratch/a", "x")
	refusedBeforeWrite(t, f, err)
}

func TestSynologyWriteRefusesWhenTheShareCannotBeAnchored(t *testing.T) {
	for name, body := range map[string]string{
		"share missing":      `{"success":true,"data":{"total":1,"shares":[{"name":"kube","path":"/kube","additional":{"real_path":"/volume1/kube"}}]}}`,
		"share case differs": `{"success":true,"data":{"total":1,"shares":[{"name":"nas","path":"/nas","additional":{"real_path":"/volume1/NAS"}}]}}`,
		"no real path":       `{"success":true,"data":{"total":1,"shares":[{"name":"NAS","path":"/NAS"}]}}`,
		"relative real path": `{"success":true,"data":{"total":1,"shares":[{"name":"NAS","path":"/NAS","additional":{"real_path":"volume1/NAS"}}]}}`,
		"no shares":          `{"success":true,"data":{"total":0,"shares":[]}}`,
		"no data":            `{"success":true}`,
		"dsm error":          `{"success":false,"error":{"code":105}}`,
		"share elsewhere":    `{"success":true,"data":{"total":1,"shares":[{"name":"NAS","path":"/NAS","additional":{"real_path":"/volume1/kube"}}]}}`,
	} {
		for tool := range writeCalls(nil) {
			t.Run(name+"/"+tool, func(t *testing.T) {
				f := newShareDSM(t)
				scriptGetInfo(f, okEntry)
				f.on("SYNO.FileStation.List/list_share", body)
				refusedBeforeWrite(t, f, writeCalls(f)[tool]())
			})
		}
	}
}

// copy_move also resolves destination + "/" + each source's name, because a
// write lands on that entry (and with overwrite replaces it).
func TestSynologyCopyMoveRefusesAnExistingDestinationEntryThatResolvesElsewhere(t *testing.T) {
	for _, move := range []bool{false, true} {
		for _, overwrite := range []bool{false, true} {
			t.Run(map[bool]string{false: "copy", true: "move"}[move]+map[bool]string{false: "", true: "/overwrite"}[overwrite], func(t *testing.T) {
				f := newShareDSM(t)
				scriptGetInfo(f, func(p string) string {
					if p == "/NAS/d/a" {
						return getinfoEntry(p, "/volume1/kube/a", "", "")
					}
					return okEntry(p)
				})
				_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
					SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/d", Move: move, Overwrite: overwrite})
				refusedBeforeWrite(t, f, err)
				mustContain(t, err.Error(), "/NAS/d/a")
			})
		}
	}
}

func TestSynologyCopyMoveRefusesAnExistingDestinationEntryThatIsALink(t *testing.T) {
	f := newShareDSM(t)
	scriptGetInfo(f, func(p string) string {
		if p == "/NAS/d/a" {
			return getinfoEntry(p, "/volume1/NAS/d/a", "", "remote")
		}
		return okEntry(p)
	})
	_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
		SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/d", Overwrite: true})
	refusedBeforeWrite(t, f, err)
}

func TestSynologyCopyMoveResolvesEverySourcesDestinationEntry(t *testing.T) {
	fastPolling(t, 5)
	f := copyFake(t, cmFinished)
	f.absent["/NAS/d/s1"], f.absent["/NAS/d/s2"] = true, true
	if _, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
		SynologyCopyMoveRequest{Sources: []string{"/NAS/s1", "/NAS/x/s2"}, Destination: "/NAS/d"}); err != nil {
		t.Fatalf("SynologyCopyMove: %v", err)
	}
	want := `["/NAS","/NAS/s1","/NAS/x/s2","/NAS/d","/NAS/d/s1","/NAS/d/s2"]`
	if got := formFor(f, "SYNO.FileStation.List/getinfo").Get("path"); got != want {
		t.Errorf("getinfo path = %s\nwant %s", got, want)
	}
}

// An existing, plain destination entry is allowed (that is what overwrite is for).
func TestSynologyCopyMoveAllowsAnExistingPlainDestinationEntry(t *testing.T) {
	fastPolling(t, 5)
	f := copyFake(t, cmFinished)
	delete(f.absent, "/NAS/d/a")
	if _, err := SynologyCopyMove(context.Background(), f.client(), allowNAS,
		SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/d", Overwrite: true}); err != nil {
		t.Fatalf("SynologyCopyMove: %v", err)
	}
}

func TestSynologyDestinationEntryLookupFailsClosedOnOtherCodes(t *testing.T) {
	for _, code := range []string{"403", "418", "100", "0"} {
		f := newShareDSM(t)
		scriptGetInfo(f, func(p string) string {
			if p == "/NAS/d/a" {
				return `{"path":"/NAS/d/a","code":` + code + `}`
			}
			return okEntry(p)
		})
		_, err := SynologyCopyMove(context.Background(), f.client(), allowNAS, SynologyCopyMoveRequest{Sources: []string{"/NAS/a"}, Destination: "/NAS/d"})
		refusedBeforeWrite(t, f, err)
	}
}

func TestSynologyRenameOntoAnExistingNameIsRefused(t *testing.T) {
	f := newShareDSM(t)
	scriptGetInfo(f, func(p string) string { return getinfoEntry(p, "/volume1"+p, "", "") })
	_, err := SynologyRename(context.Background(), f.client(), allowNAS, "/NAS/a", "b")
	refusedBeforeWrite(t, f, err)
	mustContain(t, err.Error(), "already exists", "/NAS/b")
}

func TestSynologyCreateFolderOnAnExistingNameIsRefused(t *testing.T) {
	f := newShareDSM(t)
	scriptGetInfo(f, func(p string) string { return getinfoEntry(p, "/volume1"+p, "", "") })
	_, err := SynologyCreateFolder(context.Background(), f.client(), allowNAS, "/NAS/a", "x")
	refusedBeforeWrite(t, f, err)
	mustContain(t, err.Error(), "already exists", "/NAS/a/x")
}

func TestSynologyVerifyRefusesUnexpectedShapes(t *testing.T) {
	f := newWriteDSM(t)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"nothing to check": func() error { return synoVerifyRealPaths(ctx, f.client(), nil, nil, false) },
		"empty path":       func() error { return synoVerifyRealPaths(ctx, f.client(), []synoFenced{{"", "/NAS"}}, nil, false) },
		"empty root":       func() error { return synoVerifyRealPaths(ctx, f.client(), []synoFenced{{"/NAS/a", ""}}, nil, false) },
		"path outside its root": func() error {
			return synoVerifyRealPaths(ctx, f.client(), []synoFenced{{"/NAS2/a", "/NAS"}}, nil, false)
		},
		"root is the whole NAS": func() error { return synoVerifyRealPaths(ctx, f.client(), []synoFenced{{"/NAS/a", "/"}}, nil, false) },
		"target outside root": func() error {
			return synoVerifyRealPaths(ctx, f.client(), []synoFenced{{"/NAS/a", "/NAS"}}, []synoFenced{{"/kube/x", "/NAS"}}, false)
		},
		"relative path": func() error { return synoVerifyRealPaths(ctx, f.client(), []synoFenced{{"NAS/a", "/NAS"}}, nil, false) },
	} {
		if err := call(); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests were sent for malformed input, want none", n)
	}
}

// A root that is a share itself still verifies (remainder empty).
func TestSynologyVerifyAcceptsAShareRootItself(t *testing.T) {
	f := newWriteDSM(t)
	if err := synoVerifyRealPaths(context.Background(), f.client(), []synoFenced{{"/NAS", "/NAS"}}, nil, false); err != nil {
		t.Fatalf("share root: %v", err)
	}
}

// The strings sent to the write call are the strings that were checked,
// including a name with characters that could confuse a parser.
func TestSynologyWriteSendsExactlyTheCheckedStrings(t *testing.T) {
	name := `we"ird, [n]ame & <x>`
	f := newWriteDSM(t, "/NAS/a/"+name)
	f.on("SYNO.FileStation.CreateFolder/create", `{"success":true,"data":{"folders":[{"isdir":true,"name":"n","path":"p"}]}}`)
	if _, err := SynologyCreateFolder(context.Background(), f.client(), allowNAS, "/NAS//a/", name); err != nil {
		t.Fatalf("create: %v", err)
	}
	var asked, sentParent, sentName []string
	for _, r := range f.requests() {
		switch r.key() {
		case "SYNO.FileStation.List/getinfo":
			_ = json.Unmarshal([]byte(r.form.Get("path")), &asked)
		case "SYNO.FileStation.CreateFolder/create":
			_ = json.Unmarshal([]byte(r.form.Get("folder_path")), &sentParent)
			_ = json.Unmarshal([]byte(r.form.Get("name")), &sentName)
		}
	}
	if len(sentParent) != 1 || len(sentName) != 1 || sentParent[0] != "/NAS/a" || sentName[0] != name {
		t.Fatalf("sent parent %v name %v", sentParent, sentName)
	}
	if asked[len(asked)-1] != sentParent[0]+"/"+sentName[0] {
		t.Errorf("the checked target %q is not the parent + name that was sent", asked[len(asked)-1])
	}
}
