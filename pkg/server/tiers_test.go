package server

import (
	"context"
	"testing"

	"github.com/GauranshMathur/ARR_MCP/pkg/config"
)

// A tool that loses state with no way back must be gated as destructive, or the
// default confirmScope=destructive would let it run unprompted.
//
//   - qbittorrent_remove_tags strips tags from torrents, and with no tags given strips
//     every tag they carry; the old tags are not recorded anywhere.
//   - bazarr_modify_subtitle rewrites the subtitle file in place, so a bad result means
//     downloading it again.
func TestStateLosingToolsAreGatedAsDestructive(t *testing.T) {
	qbit, _ := fakeQBitServer(t, `[]`)
	baz, _ := fakeArr(t, `{"data":[]}`)

	cases := []struct {
		tool string
		cfg  *config.Config
	}{
		{"qbittorrent_remove_tags", qbitCfg(qbit.URL, permsFull)},
		{"bazarr_modify_subtitle", bazarrCfg(baz.URL, permsFull)},
	}
	for _, tc := range cases {
		t.Run(tc.tool, func(t *testing.T) {
			cs := connect(t, tc.cfg)
			res, err := cs.ListTools(context.Background(), nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			for _, tool := range res.Tools {
				if tool.Name != tc.tool {
					continue
				}
				if tool.Annotations == nil || tool.Annotations.DestructiveHint == nil || !*tool.Annotations.DestructiveHint {
					t.Errorf("%s annotations = %+v, want a destructive hint", tc.tool, tool.Annotations)
				}
				return
			}
			t.Fatalf("%s not advertised", tc.tool)
		})
	}
}
