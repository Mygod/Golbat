package decoder

import (
	"context"
	"testing"
	"time"

	"github.com/guregu/null/v6"

	"golbat/config"
	"golbat/db"
	"golbat/pogo"
	"golbat/stats_collector"
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

func TestAcceptedNoOpWildPokemonObservationAdvancesWatermark(t *testing.T) {
	const (
		encounterId = uint64(900000001)
		spawnId     = int64(0xabc)
	)

	previousPokemonMemoryOnly := config.Config.PokemonMemoryOnly
	previousSender := webhooksSender
	previousStats := statsCollector
	config.Config.PokemonMemoryOnly = true
	webhooksSender = &recordingWebhooksSender{}
	statsCollector = stats_collector.NewNoopStatsCollector()
	defer func() {
		config.Config.PokemonMemoryOnly = previousPokemonMemoryOnly
		webhooksSender = previousSender
		statsCollector = previousStats
		pokemonCache.Delete(encounterId)
		spawnpointCache.Delete(spawnId)
	}()

	pokemon := &Pokemon{
		PokemonData: PokemonData{
			Id:                      Uint64Str(encounterId),
			PokemonId:               int16(pogo.HoloPokemonId_BULBASAUR),
			Lat:                     1.23,
			Lon:                     4.56,
			SpawnId:                 null.IntFrom(spawnId),
			UpdatedMs:               null.IntFrom(1000),
			Form:                    null.IntFrom(0),
			Weather:                 null.IntFrom(0),
			Costume:                 null.IntFrom(0),
			Gender:                  null.IntFrom(0),
			SeenType:                null.StringFrom(SeenType_Wild),
			ExpireTimestamp:         null.IntFrom(3600),
			ExpireTimestampVerified: true,
		},
	}
	pokemonCache.Set(encounterId, pokemon, time.Hour)
	spawnpointCache.Set(spawnId, &Spawnpoint{
		SpawnpointData: SpawnpointData{
			Id:        spawnId,
			Lat:       1.23,
			Lon:       4.56,
			UpdatedMs: 3000,
			LastSeen:  3,
		},
	}, time.Hour)

	UpdatePokemonBatch(context.Background(), db.DbDetails{}, ScanParameters{ProcessWild: true}, []RawWildPokemonData{{
		Cell:      1,
		Timestamp: 3000,
		Data: &pogo.WildPokemonProto{
			EncounterId:  encounterId,
			Latitude:     1.23,
			Longitude:    4.56,
			SpawnPointId: "abc",
			Pokemon: &pogo.PokemonProto{
				PokemonId: pogo.HoloPokemonId_BULBASAUR,
				PokemonDisplay: &pogo.PokemonDisplayProto{
					DisplayId: int64(pogo.HoloPokemonId_BULBASAUR),
				},
			},
		},
	}}, nil, nil, nil, "scanner")

	if pokemon.UpdatedMs.ValueOrZero() != 3000 {
		t.Fatalf("expected no-op wild observation to advance watermark, got %d", pokemon.UpdatedMs.ValueOrZero())
	}
	if pokemon.IsDirty() {
		t.Fatal("expected cache-only watermark advancement not to mark pokemon dirty")
	}

	UpdatePokemonBatch(context.Background(), db.DbDetails{}, ScanParameters{ProcessWild: true}, []RawWildPokemonData{{
		Cell:      1,
		Timestamp: 2000,
		Data: &pogo.WildPokemonProto{
			EncounterId:  encounterId,
			Latitude:     1.23,
			Longitude:    4.56,
			SpawnPointId: "abc",
			Pokemon: &pogo.PokemonProto{
				PokemonId: pogo.HoloPokemonId_IVYSAUR,
				PokemonDisplay: &pogo.PokemonDisplayProto{
					DisplayId: int64(pogo.HoloPokemonId_IVYSAUR),
				},
			},
		},
	}}, nil, nil, nil, "scanner")

	if pokemon.PokemonId != int16(pogo.HoloPokemonId_BULBASAUR) {
		t.Fatalf("expected stale replay to be rejected, got pokemon id %d", pokemon.PokemonId)
	}
	if pokemon.UpdatedMs.ValueOrZero() != 3000 {
		t.Fatalf("expected stale replay not to regress watermark, got %d", pokemon.UpdatedMs.ValueOrZero())
	}
}
