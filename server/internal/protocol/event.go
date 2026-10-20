package protocol

import (
	"encoding/json"
	"fmt"
	"maps"

	"games.nvim/server/game"
)

const eventKindField = "kind"

func EncodeEvent(e game.Event) (json.RawMessage, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("encode %s event: %w", e.EventKind(), err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, fmt.Errorf("event %s is not a JSON object: %w", e.EventKind(), err)
	}
	kind, err := json.Marshal(e.EventKind())
	if err != nil {
		return nil, err
	}
	return json.Marshal(withField(fields, eventKindField, kind))
}

func withField(fields map[string]json.RawMessage, name string, value json.RawMessage) map[string]json.RawMessage {
	out := maps.Clone(fields)
	if out == nil {
		out = make(map[string]json.RawMessage, 1)
	}
	out[name] = value
	return out
}

func InfoToWire(info game.Info) GameInfoWire {
	options := make(map[string]OptionWire, len(info.Options))
	for _, spec := range info.Options {
		options[spec.Name] = OptionWire{Type: OptionEnum, Values: spec.Values, Default: spec.Default}
	}
	return GameInfoWire{
		ID:           info.Type,
		Name:         info.Name,
		Players:      PlayersWire{Min: info.Players.Min, Max: info.Players.Max},
		Capabilities: info.Capabilities,
		Options:      options,
	}
}
