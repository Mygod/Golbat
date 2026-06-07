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

func TestDebouncedServerFreshnessAdvancesInMemoryWatermark(t *testing.T) {
	freshness := ServerFreshness(2000, 0)

	pokestop := &Pokestop{
		PokestopData: PokestopData{
			Id:        "pokestop-1",
			UpdatedMs: 1000,
		},
	}
	if savePokestopRecordWithFreshness(context.Background(), db.DbDetails{}, pokestop, freshness) {
		t.Fatal("expected debounced pokestop observation not to write")
	}
	if pokestop.UpdatedMs != 2000 {
		t.Fatalf("expected debounced pokestop observation to advance watermark, got %d", pokestop.UpdatedMs)
	}
	if pokestop.IsDirty() {
		t.Fatal("expected debounced pokestop watermark not to mark row dirty")
	}

	gym := &Gym{
		GymData: GymData{
			Id:        "gym-1",
			UpdatedMs: 1000,
		},
	}
	if saveGymRecordWithFreshness(context.Background(), db.DbDetails{}, gym, freshness) {
		t.Fatal("expected debounced gym observation not to write")
	}
	if gym.UpdatedMs != 2000 {
		t.Fatalf("expected debounced gym observation to advance watermark, got %d", gym.UpdatedMs)
	}
	if gym.IsDirty() {
		t.Fatal("expected debounced gym watermark not to mark row dirty")
	}

	initStationBattleCache()
	station := &Station{
		StationData: StationData{
			Id:        "station-1",
			UpdatedMs: 1000,
		},
	}
	if saveStationRecordWithFreshness(context.Background(), db.DbDetails{}, station, freshness) {
		t.Fatal("expected debounced station observation not to write")
	}
	if station.UpdatedMs != 2000 {
		t.Fatalf("expected debounced station observation to advance watermark, got %d", station.UpdatedMs)
	}
	if station.IsDirty() {
		t.Fatal("expected debounced station watermark not to mark row dirty")
	}
}

func TestFallbackFreshnessAppliesAndAdvancesExistingPokemonTimestamp(t *testing.T) {
	pokemon := &Pokemon{
		PokemonData: PokemonData{
			Id:        Uint64Str(1),
			PokemonId: 2,
			UpdatedMs: null.IntFrom(2000),
		},
		oldValues: PokemonOldValues{PokemonId: 1},
		dirty:     true,
	}

	if !savePokemonRecordWithFreshness(context.Background(), db.DbDetails{}, pokemon, false, false, false, FallbackFreshness(9000)) {
		t.Fatal("expected fallback freshness to save dirty existing pokemon")
	}
	if pokemon.UpdatedMs.ValueOrZero() != 9000 {
		t.Fatalf("expected fallback save to advance UpdatedMs, got %d", pokemon.UpdatedMs.ValueOrZero())
	}
	if pokemon.Changed != 9 {
		t.Fatalf("expected fallback save to apply non-watermark timestamps for changed fields, got %d", pokemon.Changed)
	}
}
