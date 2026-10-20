package hub_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/shivamshivanshu/games.nvim/server/game"
	"github.com/shivamshivanshu/games.nvim/server/internal/hub"
	"github.com/shivamshivanshu/games.nvim/server/internal/limits"
	"github.com/shivamshivanshu/games.nvim/server/internal/protocol"
	"github.com/shivamshivanshu/games.nvim/server/internal/xslices"
)

func TestRoutesAreTheCatalogedRequests(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	h, err := hub.New(t.Context(), hub.Deps{Store: e.store, Games: []game.Game{}, Limits: limits.Default(), Time: e.clock})
	if err != nil {
		t.Fatal(err)
	}
	sessionHandled := []protocol.MessageType{protocol.TypeAuthLogin, protocol.TypePing}
	routed := append(slices.Collect(maps.Keys(h.Routes())), sessionHandled...)
	cataloged := xslices.Map(protocol.Requests, func(r protocol.RequestSpec) protocol.MessageType { return r.Type })
	slices.Sort(routed)
	slices.Sort(cataloged)
	if !slices.Equal(routed, cataloged) {
		t.Errorf("routed %v, cataloged %v", routed, cataloged)
	}
}
