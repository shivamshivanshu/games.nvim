package hub_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/games"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/prototest"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

var updateCatalog = flag.Bool("update", false, "rewrite testdata/catalog.json from the hub's catalog")

var catalogGolden = filepath.Join("testdata", "catalog.json")

// TestCatalogAdvertisesEveryGameAsPinned compares what the hub advertises for
// every registered game with testdata/catalog.json; run it with -update to
// accept a deliberate change.
func TestCatalogAdvertisesEveryGameAsPinned(t *testing.T) {
	registered := xslices.Map(games.All(), func(g game.Game) game.GameType { return g.Info().Type })
	c := newEnv(t).start(t).login(t, prototest.Alice)
	advertised := call[protocol.CatalogResult](t, c, protocol.TypeCatalogList, protocol.Empty{}).Games
	advertised = slices.DeleteFunc(advertised, func(info protocol.GameInfoWire) bool { return !slices.Contains(registered, info.ID) })
	got, err := json.MarshalIndent(protocol.CatalogResult{Games: advertised}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	if *updateCatalog {
		if err := os.WriteFile(catalogGolden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(catalogGolden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(want, got) {
		t.Errorf("the advertised catalog differs from %s; review the change with go test -run %s -update and git diff:\n%s", catalogGolden, t.Name(), got)
	}
}
