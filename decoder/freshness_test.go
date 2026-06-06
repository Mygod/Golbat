package decoder

import (
	"context"
	"testing"

	"github.com/guregu/null/v6"

	"golbat/db"
)

func TestFreshnessStaleSemantics(t *testing.T) {
	server := ServerFreshness(1000, 9000)
	if !server.IsServer() {
		t.Fatal("expected non-zero server timestamp to be server freshness")
	}
	if server.TimestampMs() != 1000 {
		t.Fatalf("expected server timestamp to win, got %d", server.TimestampMs())
	}
	if !server.IsStaleFor(2000, false) {
		t.Fatal("expected older server freshness to be stale for an existing row")
	}
	if server.IsStaleFor(2000, true) {
		t.Fatal("expected new rows to accept server freshness")
	}

	fallback := ServerFreshness(0, 9000)
	if fallback.IsServer() {
		t.Fatal("expected zero server timestamp to use fallback freshness")
	}
	if fallback.TimestampMs() != 9000 {
		t.Fatalf("expected fallback timestamp, got %d", fallback.TimestampMs())
	}
	if fallback.IsStaleFor(10000, false) {
		t.Fatal("expected fallback freshness to skip server stale checks")
	}
}

func TestSavePokemonRecordWithFreshnessSkipsStaleServerTimestamp(t *testing.T) {
	pokemon := &Pokemon{
		PokemonData: PokemonData{
			Id:        Uint64Str(1),
			PokemonId: 1,
			UpdatedMs: null.IntFrom(2000),
		},
		dirty: true,
	}

	if savePokemonRecordWithFreshness(context.Background(), db.DbDetails{}, pokemon, false, false, false, ServerFreshness(1000, 0)) {
		t.Fatal("expected stale server freshness to skip save")
	}
	if pokemon.UpdatedMs.ValueOrZero() != 2000 {
		t.Fatalf("expected existing UpdatedMs to remain unchanged, got %d", pokemon.UpdatedMs.ValueOrZero())
	}
}
