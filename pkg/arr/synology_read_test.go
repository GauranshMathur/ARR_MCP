package arr

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"
)

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func mustContain(t *testing.T, got string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(got, w) {
			t.Errorf("result %s\n  missing %s", got, w)
		}
	}
}

func mustNotContain(t *testing.T, got string, bans ...string) {
	t.Helper()
	for _, b := range bans {
		if strings.Contains(got, b) {
			t.Errorf("result leaks %q: %s", b, got)
		}
	}
}

func TestSynologySystemInfoCombinesDSMInfoAndCoreSystem(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.DSM.Info/getinfo", `{"success":true,"data":{"model":"DS423+","ram":10240,"serial":"SERIAL123456","temperature":56,"temperature_warn":false,"time":"Sun Oct 11 19:42:12 2026","uptime":2753220,"version":"90080","version_string":"DSM 7.4.1-90080"}}`)
	f.on("SYNO.Core.System/info", `{"success":true,"data":{"cpu_cores":"4","cpu_family":"Celeron","cpu_series":"J4125","cpu_vendor":"INTEL","firmware_date":"2026/07/22","serial":"SERIAL123456","sys_temp":52,"time_zone":"Singapore","up_time":"764:47:5"}}`)

	got, err := SynologySystemInfo(context.Background(), f.client())
	if err != nil {
		t.Fatalf("SynologySystemInfo: %v", err)
	}
	out := jsonOf(t, got)
	mustContain(t, out, `"model":"DS423+"`, `"dsmVersion":"DSM 7.4.1-90080"`, `"uptimeSeconds":2753220`, `"temperatureC":56`,
		`"memoryMB":10240`, `"cpu":"INTEL Celeron J4125"`, `"cpuCores":4`, `"firmwareDate":"2026/07/22"`, `"timeZone":"Singapore"`)
	mustNotContain(t, out, "SERIAL123456")
	if got := strings.Join(f.seen(), ","); got != "SYNO.DSM.Info/getinfo,SYNO.Core.System/info" {
		t.Errorf("calls = %s", got)
	}
}

func TestSynologyUtilizationKeepsLoadMemoryNetworkAndDiskAndDropsLUNs(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.Core.System.Utilization/get", `{"success":true,"data":{
	 "cpu":{"15min_load":118,"1min_load":173,"5min_load":142,"other_load":5,"system_load":2,"user_load":6},
	 "disk":{"disk":[{"device":"sata1","display_name":"Drive 3","read_byte":473770,"write_byte":3510414,"utilization":12,"type":"internal"}],
	         "total":{"read_byte":3195561,"write_byte":14202053,"utilization":11}},
	 "lun":[{"lun_name":"k8s-csi-pvc-66aa90ab","uuid":"031cb481","total_iops":0}],
	 "memory":{"avail_real":487680,"cached":8356844,"real_usage":11,"swap_usage":6,"total_real":10031236,"total_swap":8118188},
	 "network":[{"device":"total","rx":1278221,"tx":2391608},{"device":"eth1","rx":1278221,"tx":2391608}],
	 "nfs":[{"device":"x"}],"smb":[{"device":"y"}],"space":{"total":{"utilization":3},"volume":[]},"time":1791719000}}`)

	got, err := SynologyUtilization(context.Background(), f.client())
	if err != nil {
		t.Fatalf("SynologyUtilization: %v", err)
	}
	out := jsonOf(t, got)
	mustContain(t, out, `"loadAverage1m":1.73`, `"loadAverage5m":1.42`, `"loadAverage15m":1.18`, `"userPercent":6`, `"systemPercent":2`,
		`"usedPercent":11`, `"swapUsedPercent":6`, `"totalMB":9796`, `"availableMB":476`, `"cachedMB":8160`,
		`"device":"eth1"`, `"rxBytesPerSec":1278221`, `"displayName":"Drive 3"`, `"utilizationPercent":12`, `"readBytesPerSec":473770`)
	mustNotContain(t, out, "k8s-csi", "031cb481", "lun")
}

func TestSynologyUpdateStatus(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.Core.Upgrade.Server/check", `{"success":true,"data":{"available":false}}`)
	got, err := SynologyUpdateStatus(context.Background(), f.client())
	if err != nil {
		t.Fatalf("SynologyUpdateStatus: %v", err)
	}
	if jsonOf(t, got) != `{"updateAvailable":false}` {
		t.Errorf("result = %s", jsonOf(t, got))
	}
}

const dsmLoadInfo = `{"success":true,"data":{
 "env":{"big":"junk"},"ports":[{"x":1}],"overview_data":{"y":2},
 "storagePools":[{"id":"reuse_1","num_id":1,"status":"normal","summary_status":"normal","device_type":"shr_with_1_disk_protect","raidType":"multiple",
   "size":{"total":"35967448645632","used":"35967142461440"},"disk_failure_number":0,"scrubbingStatus":"schedule_done","cache_disks":["nvme0n1"],
   "raids":[{"devices":[{"id":"sata4","status":"normal"},{"id":"sata3","status":"normal"}]}],
   "vspace_can_do":{"huge":"blob"},"can_do":{"delete":true}}],
 "volumes":[{"id":"volume_1","status":"normal","summary_status":"normal","fs_type":"btrfs","vol_path":"/volume1","pool_path":"reuse_1",
   "size":{"total":"34528444686336","used":"24849032151040","total_device":"35967129878528"},"vspace_can_do":{"huge":"blob"},"uuid":"94nN0i"}],
 "disks":[
  {"id":"sata1","name":"Drive 3","model":"ST12000NE0008-2PK103","vendor":"Seagate","diskType":"SATA","isSsd":false,"size_total":"12000138625024",
   "status":"normal","smart_status":"normal","overview_status":"normal","temp":46,"unc":0,"used_by":"reuse_1","remain_life":{"trustable":true,"value":-1},
   "serial":"ZTN08BLQ","ui_serial":"ZTN08BLQ","firm":"RAL3","action":{"junk":true}},
  {"id":"nvme0n1","name":"Cache device 1","model":"Samsung SSD 970","vendor":"Samsung","diskType":"SSD","isSsd":true,"size_total":"500107862016",
   "status":"normal","smart_status":"normal","temp":38,"unc":0,"used_by":"alloc_cache_1_1","remain_life":{"trustable":true,"value":97},"serial":"S4EVNX0"}
 ]}}`

func TestSynologyStorageTrimsLoadInfoToPoolsAndVolumes(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.Storage.CGI.Storage/load_info", dsmLoadInfo)
	got, err := SynologyStorage(context.Background(), f.client())
	if err != nil {
		t.Fatalf("SynologyStorage: %v", err)
	}
	out := jsonOf(t, got)
	mustContain(t, out,
		`"id":"reuse_1"`, `"status":"normal"`, `"deviceType":"shr_with_1_disk_protect"`, `"totalBytes":35967448645632`, `"scrubbing":"schedule_done"`,
		`"diskIds":["sata4","sata3"]`, `"cacheDisks":["nvme0n1"]`,
		`"id":"volume_1"`, `"fsType":"btrfs"`, `"path":"/volume1"`, `"poolId":"reuse_1"`,
		`"totalBytes":34528444686336`, `"usedBytes":24849032151040`, `"freeBytes":9679412535296`, `"usedPercent":72`)
	mustNotContain(t, out, "junk", "blob", "94nN0i", "ZTN08BLQ")
	if len(out) > 1500 {
		t.Errorf("overview is %d bytes; load_info must be trimmed hard", len(out))
	}
}

func TestSynologyDisksHealthWithoutSerials(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.Storage.CGI.Storage/load_info", dsmLoadInfo)
	got, err := SynologyDisks(context.Background(), f.client())
	if err != nil {
		t.Fatalf("SynologyDisks: %v", err)
	}
	out := jsonOf(t, got)
	mustContain(t, out, `"id":"sata1"`, `"name":"Drive 3"`, `"model":"ST12000NE0008-2PK103"`, `"vendor":"Seagate"`, `"type":"SATA"`,
		`"sizeBytes":12000138625024`, `"status":"normal"`, `"smartStatus":"normal"`, `"temperatureC":46`, `"uncorrectableSectors":0`,
		`"usedBy":"reuse_1"`, `"id":"nvme0n1"`, `"isSsd":true`, `"remainingLifePercent":97`)
	if strings.Count(out, "remainingLifePercent") != 1 {
		t.Errorf("an HDD reports remain_life -1: it must be omitted, not shown as a percentage: %s", out)
	}
	mustNotContain(t, out, "ZTN08BLQ", "S4EVNX0", "junk", "RAL3")
}

func TestSynologyListSharesPagesAndCapsTheLimit(t *testing.T) {
	f := newDSM(t)
	var form url.Values
	f.handlers["SYNO.FileStation.List/list_share"] = func(_ int, v url.Values) string {
		form = v
		return `{"success":true,"data":{"offset":0,"total":5,"shares":[
		 {"isdir":true,"name":"docker","path":"/docker","additional":{"real_path":"/volume1/docker","perm":{"share_right":"RW","is_share_readonly":false}}},
		 {"isdir":true,"name":"NAS","path":"/NAS","additional":{"real_path":"/volume1/NAS","perm":{"share_right":"RO","is_share_readonly":true}}}]}}`
	}
	got, err := SynologyListShares(context.Background(), f.client(), 0, 100000)
	if err != nil {
		t.Fatalf("SynologyListShares: %v", err)
	}
	if form.Get("limit") != "100" || form.Get("offset") != "0" {
		t.Errorf("limit/offset = %q/%q, want 100/0 (capped)", form.Get("limit"), form.Get("offset"))
	}
	if form.Get("additional") != `["real_path","perm"]` {
		t.Errorf("additional = %q", form.Get("additional"))
	}
	out := jsonOf(t, got)
	mustContain(t, out, `"total":5`, `"count":2`, `"name":"docker"`, `"path":"/docker"`, `"realPath":"/volume1/docker"`, `"access":"RW"`, `"readOnly":true`)
}

func TestSynologyListSharesDefaultsToTwentyFive(t *testing.T) {
	f := newDSM(t)
	var form url.Values
	f.handlers["SYNO.FileStation.List/list_share"] = func(_ int, v url.Values) string {
		form = v
		return `{"success":true,"data":{"offset":0,"total":0,"shares":[]}}`
	}
	if _, err := SynologyListShares(context.Background(), f.client(), -3, 0); err != nil {
		t.Fatal(err)
	}
	if form.Get("limit") != "25" || form.Get("offset") != "0" {
		t.Errorf("limit/offset = %q/%q, want 25/0", form.Get("limit"), form.Get("offset"))
	}
}

func TestSynologyListFilesSendsPagingSortAndFilter(t *testing.T) {
	f := newDSM(t)
	var form url.Values
	f.handlers["SYNO.FileStation.List/list"] = func(_ int, v url.Values) string {
		form = v
		return `{"success":true,"data":{"offset":0,"total":9,"files":[
		 {"isdir":true,"name":"Movies","path":"/NAS/Movies","additional":{"size":4096,"owner":{"user":"Arkhaya","group":"users"},"time":{"mtime":1791599880,"crtime":1739794392}}},
		 {"isdir":false,"name":"a.mkv","path":"/NAS/a.mkv","additional":{"size":123456,"owner":{"user":"Arkhaya"},"time":{"mtime":1791599880},"type":"MKV"}}]}}`
	}
	desc := true
	got, err := SynologyListFiles(context.Background(), f.client(), SynologyListQuery{
		Path: "/NAS//Movies/", Limit: 500, Offset: 10, SortBy: "mtime", Desc: &desc, Type: "file",
	})
	if err != nil {
		t.Fatalf("SynologyListFiles: %v", err)
	}
	for k, v := range map[string]string{"folder_path": "/NAS/Movies", "limit": "100", "offset": "10", "sort_by": "mtime", "sort_direction": "desc", "filetype": "file", "additional": `["size","time","owner"]`} {
		if form.Get(k) != v {
			t.Errorf("form %s = %q, want %q", k, form.Get(k), v)
		}
	}
	out := jsonOf(t, got)
	mustNotContain(t, out, `"created"`, `"group"`)
	mustContain(t, out, `"total":9`, `"count":2`, `"name":"a.mkv"`, `"sizeBytes":123456`, `"modified":"`+time.Unix(1791599880, 0).UTC().Format(time.RFC3339)+`"`, `"owner":"Arkhaya"`, `"extension":"MKV"`)
	// A directory's size field is the directory entry, not its contents.
	if strings.Contains(out, `"name":"Movies","path":"/NAS/Movies","isDir":true,"sizeBytes"`) || strings.Contains(out, "4096") {
		t.Errorf("a directory must not report its entry size as sizeBytes: %s", out)
	}
}

func TestSynologyListFilesDefaults(t *testing.T) {
	f := newDSM(t)
	var form url.Values
	f.handlers["SYNO.FileStation.List/list"] = func(_ int, v url.Values) string {
		form = v
		return `{"success":true,"data":{"offset":0,"total":0,"files":[]}}`
	}
	if _, err := SynologyListFiles(context.Background(), f.client(), SynologyListQuery{Path: "/NAS"}); err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]string{"limit": "25", "sort_by": "name", "sort_direction": "asc", "filetype": "all"} {
		if form.Get(k) != v {
			t.Errorf("default %s = %q, want %q", k, form.Get(k), v)
		}
	}
}

func TestSynologyFileInfo(t *testing.T) {
	f := newDSM(t)
	var form url.Values
	f.handlers["SYNO.FileStation.List/getinfo"] = func(_ int, v url.Values) string {
		form = v
		return `{"success":true,"data":{"files":[{"isdir":false,"name":"a, b.mkv","path":"/NAS/a, b.mkv","additional":{"real_path":"/volume1/NAS/a, b.mkv","size":99,"owner":{"user":"u","group":"users"},"time":{"mtime":1791599880,"crtime":1739794392,"atime":1791718801},"type":"MKV"}}]}}`
	}
	got, err := SynologyFileInfo(context.Background(), f.client(), "/NAS/a, b.mkv")
	if err != nil {
		t.Fatalf("SynologyFileInfo: %v", err)
	}
	// A JSON array, so a comma in the name is not read as a separator.
	if form.Get("path") != `["/NAS/a, b.mkv"]` {
		t.Errorf("path = %q", form.Get("path"))
	}
	mustContain(t, jsonOf(t, got), `"name":"a, b.mkv"`, `"realPath":"/volume1/NAS/a, b.mkv"`, `"sizeBytes":99`, `"group":"users"`, `"created":"`)
}

// getinfo answers success even for a missing path, with the failure inside the
// file entry.
func TestSynologyFileInfoSurfacesAPerFileError(t *testing.T) {
	f := newDSM(t)
	f.on("SYNO.FileStation.List/getinfo", `{"success":true,"data":{"files":[{"code":408,"path":"/NAS/nope"}]}}`)
	_, err := SynologyFileInfo(context.Background(), f.client(), "/NAS/nope")
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("error = %v, want no such file or directory", err)
	}
}

// --- background tasks ---

// fastPolling shrinks the poll interval so bound tests run in milliseconds.
func fastPolling(t *testing.T, polls int) {
	t.Helper()
	oi, om := synoPollInterval, synoMaxPolls
	synoPollInterval, synoMaxPolls = time.Millisecond, polls
	t.Cleanup(func() { synoPollInterval, synoMaxPolls = oi, om })
}

func searchFake(t *testing.T, listBodies ...string) *dsmFake {
	t.Helper()
	f := newDSM(t)
	f.on("SYNO.FileStation.Search/start", `{"success":true,"data":{"taskid":"task-1","has_not_index_share":true}}`)
	f.handlers["SYNO.FileStation.Search/list"] = func(n int, _ url.Values) string {
		return listBodies[min(n-1, len(listBodies)-1)]
	}
	return f
}

func TestSynologySearchPollsUntilFinishedThenStopsAndCleansTheTask(t *testing.T) {
	fastPolling(t, 10)
	f := searchFake(t,
		`{"success":true,"data":{"finished":false,"offset":0,"total":1,"files":[{"isdir":false,"name":"a.mkv","path":"/NAS/a.mkv"}]}}`,
		`{"success":true,"data":{"finished":true,"offset":0,"total":2,"files":[{"isdir":false,"name":"a.mkv","path":"/NAS/a.mkv"},{"isdir":true,"name":"b","path":"/NAS/b"}]}}`)
	got, err := SynologySearchFiles(context.Background(), f.client(), SynologySearchQuery{Path: "/NAS", Pattern: "mkv", Limit: 500})
	if err != nil {
		t.Fatalf("SynologySearchFiles: %v", err)
	}
	if got.Partial || got.Total != 2 || len(got.Files) != 2 {
		t.Errorf("result = %+v, want complete with 2 files", got)
	}
	want := "SYNO.FileStation.Search/start,SYNO.FileStation.Search/list,SYNO.FileStation.Search/list,SYNO.FileStation.Search/stop,SYNO.FileStation.Search/clean"
	if g := strings.Join(f.seen(), ","); g != want {
		t.Errorf("calls = %s\n want %s", g, want)
	}
	start := f.requests()[1].form
	for k, v := range map[string]string{"folder_path": "/NAS", "pattern": "mkv", "recursive": "true"} {
		if start.Get(k) != v {
			t.Errorf("start %s = %q, want %q", k, start.Get(k), v)
		}
	}
	var list url.Values
	for _, r := range f.requests() {
		if r.key() == "SYNO.FileStation.Search/list" {
			list = r.form
		}
	}
	if list.Get("taskid") != "task-1" || list.Get("limit") != "100" {
		t.Errorf("list form = %v, want taskid task-1 and a capped limit of 100", list)
	}
}

func TestSynologySearchHitsTheBoundAndReturnsPartialResultsAfterStopping(t *testing.T) {
	fastPolling(t, 3)
	f := searchFake(t, `{"success":true,"data":{"finished":false,"offset":0,"total":7,"files":[{"isdir":false,"name":"a.mkv","path":"/NAS/a.mkv"}]}}`)
	got, err := SynologySearchFiles(context.Background(), f.client(), SynologySearchQuery{Path: "/NAS", Pattern: "mkv"})
	if err != nil {
		t.Fatalf("SynologySearchFiles: %v", err)
	}
	if !got.Partial || len(got.Files) != 1 {
		t.Errorf("result = %+v, want partial with the files found so far", got)
	}
	seen := strings.Join(f.seen(), ",")
	if strings.Count(seen, "Search/list") != 3 {
		t.Errorf("polled %s, want exactly the 3 capped attempts", seen)
	}
	if !strings.HasSuffix(seen, "SYNO.FileStation.Search/stop,SYNO.FileStation.Search/clean") {
		t.Errorf("calls = %s, want the task stopped and cleaned last", seen)
	}
}

func TestSynologySearchStopsTheTaskWhenTheContextExpires(t *testing.T) {
	fastPolling(t, 1000)
	f := searchFake(t, `{"success":true,"data":{"finished":false,"offset":0,"total":0,"files":[]}}`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	got, err := SynologySearchFiles(ctx, f.client(), SynologySearchQuery{Path: "/NAS", Pattern: "x"})
	if err != nil {
		t.Fatalf("SynologySearchFiles: %v", err)
	}
	if !got.Partial {
		t.Errorf("result = %+v, want partial", got)
	}
	if seen := strings.Join(f.seen(), ","); !strings.HasSuffix(seen, "Search/stop,SYNO.FileStation.Search/clean") {
		t.Errorf("calls = %s, want stop and clean even though the context expired", seen)
	}
}

// DSM runs one search at a time: a start made while another runs is answered
// finished with no total at all. That is not "no files found".
func TestSynologySearchThatNeverRanIsAnErrorNotAnEmptyResult(t *testing.T) {
	fastPolling(t, 5)
	f := searchFake(t, `{"success":true,"data":{"finished":true}}`)
	_, err := SynologySearchFiles(context.Background(), f.client(), SynologySearchQuery{Path: "/NAS", Pattern: "x"})
	if err == nil || !strings.Contains(err.Error(), "did not run") {
		t.Fatalf("error = %v, want a did-not-run error", err)
	}
	if seen := strings.Join(f.seen(), ","); !strings.HasSuffix(seen, "Search/stop,SYNO.FileStation.Search/clean") {
		t.Errorf("calls = %s, want the task cleaned up", seen)
	}
}

// The did-not-run answer is intermittent on the live NAS (about half of starts),
// so the search is started again, a bounded number of times.
func TestSynologySearchStartsAgainWhenTheFirstStartDidNotRun(t *testing.T) {
	fastPolling(t, 5)
	f := searchFake(t,
		`{"success":true,"data":{"finished":true}}`,
		`{"success":true,"data":{"finished":true,"offset":0,"total":1,"files":[{"isdir":false,"name":"a.mkv","path":"/NAS/a.mkv"}]}}`)
	got, err := SynologySearchFiles(context.Background(), f.client(), SynologySearchQuery{Path: "/NAS", Pattern: "mkv"})
	if err != nil || got.Total != 1 || got.Partial {
		t.Fatalf("got %+v, err %v, want the second start's result", got, err)
	}
	seen := strings.Join(f.seen(), ",")
	if strings.Count(seen, "Search/start") != 2 || strings.Count(seen, "Search/stop") != 2 || strings.Count(seen, "Search/clean") != 2 {
		t.Errorf("calls = %s, want 2 starts, each stopped and cleaned", seen)
	}
}

func TestSynologySearchGivesUpAfterFourStarts(t *testing.T) {
	fastPolling(t, 5)
	f := searchFake(t, `{"success":true,"data":{"finished":true}}`)
	if _, err := SynologySearchFiles(context.Background(), f.client(), SynologySearchQuery{Path: "/NAS", Pattern: "x"}); err == nil {
		t.Fatal("expected an error")
	}
	if n := strings.Count(strings.Join(f.seen(), ","), "Search/start"); n != 4 {
		t.Errorf("starts = %d, want 4", n)
	}
}

func TestSynologySearchGenuinelyEmptyIsNotAnError(t *testing.T) {
	fastPolling(t, 5)
	f := searchFake(t, `{"success":true,"data":{"finished":true,"offset":0,"total":0,"files":[]}}`)
	got, err := SynologySearchFiles(context.Background(), f.client(), SynologySearchQuery{Path: "/NAS", Pattern: "x"})
	if err != nil || got.Partial || got.Total != 0 {
		t.Errorf("got %+v, err %v, want a complete empty result", got, err)
	}
}

func TestSynologySearchRejectsBadInputWithoutARequest(t *testing.T) {
	f := searchFake(t, `{"success":true,"data":{"finished":true,"total":0}}`)
	for name, q := range map[string]SynologySearchQuery{
		"relative path": {Path: "NAS", Pattern: "x"},
		"traversal":     {Path: "/NAS/../kube", Pattern: "x"},
		"empty pattern": {Path: "/NAS", Pattern: " "},
		"bad type":      {Path: "/NAS", Pattern: "x", Type: "everything"},
	} {
		if _, err := SynologySearchFiles(context.Background(), f.client(), q); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests reached the NAS", n)
	}
}

func dirSizeFake(t *testing.T, statuses ...string) *dsmFake {
	t.Helper()
	f := newDSM(t)
	f.on("SYNO.FileStation.List/getinfo", `{"success":true,"data":{"files":[{"name":"x","path":"/NAS/x","isdir":true}]}}`)
	f.on("SYNO.FileStation.DirSize/start", `{"success":true,"data":{"taskid":"dir-1"}}`)
	f.handlers["SYNO.FileStation.DirSize/status"] = func(n int, _ url.Values) string {
		return statuses[min(n-1, len(statuses)-1)]
	}
	return f
}

func TestSynologyDirSizePollsThenStopsTheTask(t *testing.T) {
	fastPolling(t, 10)
	f := dirSizeFake(t,
		`{"success":true,"data":{"finished":false,"num_dir":3,"num_file":10,"total_size":100}}`,
		`{"success":true,"data":{"finished":true,"num_dir":822,"num_file":4853,"total_size":7968754103816}}`)
	got, err := SynologyDirSize(context.Background(), f.client(), "/NAS/Movies/")
	if err != nil {
		t.Fatalf("SynologyDirSize: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"path":"/NAS/Movies"`, `"totalBytes":7968754103816`, `"directories":822`, `"files":4853`, `"partial":false`)
	want := "SYNO.FileStation.List/getinfo,SYNO.FileStation.DirSize/start,SYNO.FileStation.DirSize/status,SYNO.FileStation.DirSize/status,SYNO.FileStation.DirSize/stop"
	if g := strings.Join(f.seen(), ","); g != want {
		t.Errorf("calls = %s\n want %s", g, want)
	}
	if p := f.requests()[2].form.Get("path"); p != `["/NAS/Movies"]` {
		t.Errorf("start path = %q, want a JSON array of the cleaned path", p)
	}
}

// For a path that does not exist DSM can answer a finished task of all zeros,
// which would read as an empty folder. The path is looked up first.
func TestSynologyDirSizeChecksThePathExistsFirst(t *testing.T) {
	fastPolling(t, 5)
	f := dirSizeFake(t, `{"success":true,"data":{"finished":true,"num_dir":0,"num_file":0,"total_size":0}}`)
	f.on("SYNO.FileStation.List/getinfo", `{"success":true,"data":{"files":[{"code":408,"path":"/NAS/nope"}]}}`)
	_, err := SynologyDirSize(context.Background(), f.client(), "/NAS/nope")
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Fatalf("error = %v, want no such file or directory", err)
	}
	if got := strings.Join(f.seen(), ","); got != "SYNO.FileStation.List/getinfo" {
		t.Errorf("calls = %s, want no task started for a missing path", got)
	}
}

func TestSynologyDirSizeBoundIsPartialAndStillStops(t *testing.T) {
	fastPolling(t, 2)
	f := dirSizeFake(t, `{"success":true,"data":{"finished":false,"num_dir":3,"num_file":10,"total_size":100}}`)
	got, err := SynologyDirSize(context.Background(), f.client(), "/NAS")
	if err != nil {
		t.Fatalf("SynologyDirSize: %v", err)
	}
	mustContain(t, jsonOf(t, got), `"partial":true`, `"totalBytes":100`)
	if seen := strings.Join(f.seen(), ","); strings.Count(seen, "DirSize/status") != 2 || !strings.HasSuffix(seen, "DirSize/stop") {
		t.Errorf("calls = %s, want 2 capped polls then stop", seen)
	}
}

// DSM sometimes starts a directory-size task it then does not know (status 599
// from the first poll on), and a missing path looks the same. The start is
// repeated a bounded number of times before the error is believed.
func TestSynologyDirSizeGivesUpAfterFourUnknownTasksAndStopsEach(t *testing.T) {
	fastPolling(t, 5)
	f := dirSizeFake(t, `{"success":false,"error":{"code":599}}`)
	_, err := SynologyDirSize(context.Background(), f.client(), "/NAS/nope")
	if err == nil || !strings.Contains(err.Error(), "no such task") {
		t.Fatalf("error = %v", err)
	}
	seen := strings.Join(f.seen(), ",")
	if strings.Count(seen, "DirSize/start") != 4 || strings.Count(seen, "DirSize/stop") != 4 {
		t.Errorf("calls = %s, want 4 starts, each stopped", seen)
	}
}

func TestSynologyDirSizeStartsAgainWhenTheFirstTaskIsUnknown(t *testing.T) {
	fastPolling(t, 5)
	f := dirSizeFake(t,
		`{"success":false,"error":{"code":599}}`,
		`{"success":true,"data":{"finished":true,"num_dir":1,"num_file":2,"total_size":300}}`)
	got, err := SynologyDirSize(context.Background(), f.client(), "/NAS")
	if err != nil || got.TotalBytes != 300 || got.Partial {
		t.Fatalf("got %+v, err %v, want the second task's figures", got, err)
	}
}

// A 599 after polling has begun is a real failure, not the flaky start.
func TestSynologyDirSizeDoesNotRepeatAfterAGoodPoll(t *testing.T) {
	fastPolling(t, 5)
	f := dirSizeFake(t,
		`{"success":true,"data":{"finished":false,"num_dir":1,"num_file":1,"total_size":1}}`,
		`{"success":false,"error":{"code":599}}`)
	if _, err := SynologyDirSize(context.Background(), f.client(), "/NAS"); err == nil {
		t.Fatal("expected an error")
	}
	if n := strings.Count(strings.Join(f.seen(), ","), "DirSize/start"); n != 1 {
		t.Errorf("starts = %d, want 1", n)
	}
}

// --- path validation ---

func TestSynologyCleanPath(t *testing.T) {
	good := map[string]string{
		"/NAS":                "/NAS",
		"/NAS/":               "/NAS",
		"/NAS//Movies/./x":    "/NAS/Movies/x",
		" /NAS/Movies ":       "/NAS/Movies",
		"/NAS/a, b (2009)":    "/NAS/a, b (2009)",
		"/NAS/..hidden":       "/NAS/..hidden",
		"/NAS/file..name.mkv": "/NAS/file..name.mkv",
	}
	for in, want := range good {
		got, err := SynologyCleanPath(in)
		if err != nil || got != want {
			t.Errorf("SynologyCleanPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"", " ", "/", "//", "NAS", "NAS/Movies", "./NAS", "../NAS", "/NAS/..", "/NAS/../kube", "/NAS/Movies/../../kube",
		"/..", "/NAS/./../x", "/NAS\x00/x", "/NAS/a\nb", `/NAS\..\kube`, "http://x/NAS", "~/NAS",
	}
	for _, in := range bad {
		if got, err := SynologyCleanPath(in); err == nil {
			t.Errorf("SynologyCleanPath(%q) = %q, want an error", in, got)
		}
	}
}

func TestSynologyPathToolsRejectBadPathsWithoutARequest(t *testing.T) {
	f := newDSM(t)
	ctx := context.Background()
	for _, p := range []string{"NAS", "/NAS/../kube", "", "/"} {
		if _, err := SynologyListFiles(ctx, f.client(), SynologyListQuery{Path: p}); err == nil {
			t.Errorf("list accepted %q", p)
		}
		if _, err := SynologyFileInfo(ctx, f.client(), p); err == nil {
			t.Errorf("getinfo accepted %q", p)
		}
		if _, err := SynologyDirSize(ctx, f.client(), p); err == nil {
			t.Errorf("dirsize accepted %q", p)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests reached the NAS, want none", n)
	}
}

func TestSynologyListFilesRejectsBadSortAndFilterWithoutARequest(t *testing.T) {
	f := newDSM(t)
	for _, q := range []SynologyListQuery{
		{Path: "/NAS", SortBy: "password"},
		{Path: "/NAS", Type: "symlink"},
	} {
		if _, err := SynologyListFiles(context.Background(), f.client(), q); err == nil {
			t.Errorf("accepted %+v", q)
		}
	}
	if n := len(f.requests()); n != 0 {
		t.Errorf("%d requests reached the NAS", n)
	}
}
