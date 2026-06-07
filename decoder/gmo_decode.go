package decoder

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"golbat/db"
	"golbat/pogo"
)

func UpdateFortBatch(ctx context.Context, db db.DbDetails, scanParameters ScanParameters, p []RawFortData) {
	// Logic is:
	// 1. Filter out pokestops that are unchanged (last modified time)
	// 2. Fetch current stops from database
	// 3. Generate batch of inserts as needed (with on duplicate saveGymRecord)

	//var stopsToModify []string

	for _, fort := range p {
		fortId := fort.Data.FortId
		freshness := ServerFreshness(fort.Timestamp, 0)
		if fort.Data.FortType == pogo.FortType_CHECKPOINT && scanParameters.ProcessPokestops {
			pokestop, unlock, err := getOrCreatePokestopRecord(ctx, db, fortId, "UpdateFortBatch")
			if err != nil {
				log.Errorf("getOrCreatePokestopRecord: %s", err)
				continue
			}

			pokestopStale := freshness.IsStaleFor(pokestop.UpdatedMs, pokestop.IsNewRecord())
			isNewRecord := false
			if pokestopStale {
				unlock()
			} else {
				pokestop.updatePokestopFromFort(fort.Data, fort.Cell, freshness.Unix())
				isNewRecord = pokestop.IsNewRecord()

				savePokestopRecordWithFreshness(ctx, db, pokestop, freshness)
				unlock()
			}

			// If this was a new pokestop, check if it was converted from a gym and copy shared fields.
			// To avoid deadlock, we do this after releasing the pokestop lock.
			if isNewRecord && DoesGymExist(ctx, db, fortId) {
				// Get shared fields from gym (with gym lock only)
				gym, gymUnlock, _ := GetGymRecordReadOnly(ctx, db, fortId, "UpdateFortBatch.pokestopSharedFields")
				if gym != nil {
					sharedFields := gym.GetSharedFields()
					gymUnlock()

					// Re-acquire pokestop lock to apply shared fields
					pokestop, unlock, err = getPokestopRecordForUpdate(ctx, db, fortId, "UpdateFortBatch.sharedFields")
					if err != nil {
						log.Errorf("getPokestopRecordForUpdate (shared fields): %s", err)
					} else if pokestop != nil {
						if freshness.IsStaleFor(pokestop.UpdatedMs, pokestop.IsNewRecord()) {
							unlock()
						} else {
							pokestop.ApplySharedFields(sharedFields)
							savePokestopRecordWithFreshness(ctx, db, pokestop, freshness)
							unlock()
						}
					}
				}
			}

			incidents := fort.Data.PokestopDisplays
			if incidents == nil && fort.Data.PokestopDisplay != nil {
				incidents = []*pogo.PokestopIncidentDisplayProto{fort.Data.PokestopDisplay}
			}

			for _, incidentProto := range incidents {
				if incidentProto.IncidentId == "" {
					continue
				}
				incident, unlock, err := getOrCreateIncidentRecord(ctx, db, incidentProto.IncidentId, fortId, "UpdateFortBatch")
				if err != nil {
					log.Errorf("getOrCreateIncidentRecord: %s", err)
					continue
				}
				if freshness.IsStaleFor(incident.UpdatedMs, incident.IsNewRecord()) {
					unlock()
					continue
				}
				incident.updateFromPokestopIncidentDisplay(incidentProto)
				saveIncidentRecordWithFreshness(ctx, db, incident, freshness)
				unlock()
			}
		}

		if fort.Data.FortType == pogo.FortType_GYM && scanParameters.ProcessGyms {
			gym, gymUnlock, err := getOrCreateGymRecord(ctx, db, fortId, "UpdateFortBatch")
			if err != nil {
				log.Errorf("getOrCreateGymRecord: %s", err)
				continue
			}

			isNewRecord := false
			if freshness.IsStaleFor(gym.UpdatedMs, gym.IsNewRecord()) {
				gymUnlock()
			} else {
				gym.updateGymFromFort(fort.Data, fort.Cell, fort.Timestamp)
				isNewRecord = gym.IsNewRecord()

				saveGymRecordWithFreshness(ctx, db, gym, freshness)
				gymUnlock()
			}

			// If this was a new gym, check if it was converted from a pokestop and copy shared fields.
			// To avoid deadlock, we do this after releasing the gym lock.
			if isNewRecord && DoesPokestopExist(ctx, db, fortId) {
				// Get shared fields from pokestop (with pokestop lock only)
				pokestop, unlock, _ := getPokestopRecordReadOnly(ctx, db, fortId, "UpdateFortBatch.gymSharedFields")
				if pokestop != nil {
					sharedFields := pokestop.GetSharedFields()
					unlock()

					// Re-acquire gym lock to apply shared fields
					gym, gymUnlock, err = getGymRecordForUpdate(ctx, db, fortId, "UpdateFortBatch.sharedFields")
					if err != nil {
						log.Errorf("getGymRecordForUpdate (shared fields): %s", err)
					} else if gym != nil {
						if freshness.IsStaleFor(gym.UpdatedMs, gym.IsNewRecord()) {
							gymUnlock()
						} else {
							gym.ApplySharedFields(sharedFields)
							saveGymRecordWithFreshness(ctx, db, gym, freshness)
							gymUnlock()
						}
					}
				}
			}
		}
	}
}

func UpdateStationBatch(ctx context.Context, db db.DbDetails, scanParameters ScanParameters, p []RawStationData) {
	for _, stationProto := range p {
		stationId := stationProto.Data.Id
		station, unlock, err := getOrCreateStationRecord(ctx, db, stationId, "UpdateStationBatch")
		if err != nil {
			log.Errorf("getOrCreateStationRecord: %s", err)
			continue
		}
		freshness := ServerFreshness(stationProto.Timestamp, 0)
		if !freshness.IsStaleFor(station.UpdatedMs, station.IsNewRecord()) {
			station.updateFromStationProto(stationProto.Data, stationProto.Cell)
		}
		syncStationBattlesFromProto(station, stationProto.Data.BattleDetails, freshness)
		saveStationRecordWithFreshness(ctx, db, station, freshness)
		unlock()
	}
}

func UpdatePokemonBatch(ctx context.Context, db db.DbDetails, scanParameters ScanParameters, wildPokemonList []RawWildPokemonData, nearbyPokemonList []RawNearbyPokemonData, mapPokemonList []RawMapPokemonData, weather []*pogo.ClientWeatherProto, username string) {
	weatherLookup := make(map[int64]pogo.GameplayWeatherProto_WeatherCondition)
	for _, weatherProto := range weather {
		weatherLookup[weatherProto.S2CellId] = weatherProto.GameplayWeather.GameplayCondition
	}

	for _, wild := range wildPokemonList {
		encounterId := wild.Data.EncounterId

		// spawnpointUpdateFromWild doesn't need Pokemon lock
		spawnpointUpdateFromWild(ctx, db, wild.Data, wild.Timestamp)

		if scanParameters.ProcessWild {
			pokemon, unlock, err := getOrCreatePokemonRecord(ctx, db, encounterId, "UpdatePokemonBatch.wild")
			if err != nil {
				log.Errorf("getOrCreatePokemonRecord: %s", err)
				continue
			}

			freshness := ServerFreshness(wild.Timestamp, 0)
			if freshness.IsStaleFor(pokemon.UpdatedMs.ValueOrZero(), pokemon.isNewRecord()) {
				unlock()
				continue
			}
			updateTime := freshness.Unix()
			if pokemon.isNewRecord() || pokemon.wildSignificantUpdate(wild.Data, updateTime) {
				pokemon.updateFromWild(ctx, db, wild.Data, int64(wild.Cell), weatherLookup, wild.Timestamp, username)
			}
			savePokemonRecordWithFreshness(ctx, db, pokemon, false, true, true, freshness)
			unlock()
		}
	}

	if scanParameters.ProcessNearby {
		for _, nearby := range nearbyPokemonList {
			encounterId := nearby.Data.EncounterId

			if nearby.Data.FortId != "" || scanParameters.ProcessNearbyCell {
				pokemon, unlock, err := getOrCreatePokemonRecord(ctx, db, encounterId, "UpdatePokemonBatch.nearby")
				if err != nil {
					log.Printf("getOrCreatePokemonRecord: %s", err)
					continue
				}

				freshness := ServerFreshness(nearby.Timestamp, 0)
				if freshness.IsStaleFor(pokemon.UpdatedMs.ValueOrZero(), pokemon.isNewRecord()) {
					unlock()
					continue
				}
				updateTime := freshness.Unix()
				if pokemon.isNewRecord() || pokemon.nearbySignificantUpdate(nearby.Data, updateTime) {
					pokemon.updateFromNearby(ctx, db, nearby.Data, int64(nearby.Cell), weatherLookup, nearby.Timestamp, username)
				}
				savePokemonRecordWithFreshness(ctx, db, pokemon, false, true, true, freshness)

				unlock()
			}
		}
	}

	for _, mapPokemon := range mapPokemonList {
		encounterId := mapPokemon.Data.EncounterId

		pokemon, unlock, err := getOrCreatePokemonRecord(ctx, db, encounterId, "UpdatePokemonBatch.map")
		if err != nil {
			log.Printf("getOrCreatePokemonRecord: %s", err)
			continue
		}

		freshness := ServerFreshness(mapPokemon.Timestamp, 0)
		if freshness.IsStaleFor(pokemon.UpdatedMs.ValueOrZero(), pokemon.isNewRecord()) {
			unlock()
			continue
		}
		pokemon.updateFromMap(ctx, db, mapPokemon.Data, int64(mapPokemon.Cell), weatherLookup, mapPokemon.Timestamp, username)
		storedDiskEncounter := diskEncounterCache.Get(encounterId)
		if storedDiskEncounter != nil {
			diskEncounter := storedDiskEncounter.Value()
			diskEncounterCache.Delete(encounterId)
			pokemon.updatePokemonFromDiskEncounterProto(ctx, db, diskEncounter, username)
			//log.Infof("Processed stored disk encounter")
		}
		savePokemonRecordWithFreshness(ctx, db, pokemon, false, true, true, freshness)

		unlock()
	}
}

func UpdateClientWeatherBatch(ctx context.Context, db db.DbDetails, p []*pogo.ClientWeatherProto, timestampMs int64, account string) (updates []WeatherUpdate) {
	hourKey := timestampMs / time.Hour.Milliseconds()
	for _, weatherProto := range p {
		weather, unlock, err := getOrCreateWeatherRecord(ctx, db, weatherProto.S2CellId, "UpdateClientWeatherBatch")
		if err != nil {
			log.Printf("getOrCreateWeatherRecord: %s", err)
			continue
		}

		if weather.newRecord || timestampMs >= weather.UpdatedMs {
			state := getWeatherConsensusState(weatherProto.S2CellId, hourKey)
			if state != nil {
				publish, publishProto := state.applyObservation(hourKey, account, weatherProto)
				if publish {
					if publishProto == nil {
						publishProto = weatherProto
					}
					weather.UpdatedMs = timestampMs
					weather.updateWeatherFromClientWeatherProto(publishProto)
					saveWeatherRecord(ctx, db, weather)
					if weather.oldValues.GameplayCondition != weather.GameplayCondition {
						updates = append(updates, WeatherUpdate{
							S2CellId:   publishProto.S2CellId,
							NewWeather: int32(publishProto.GetGameplayWeather().GetGameplayCondition()),
						})
					}
				}
			}
		}

		unlock()
	}
	return updates
}

func UpdateClientMapS2CellBatch(ctx context.Context, db db.DbDetails, cells []RawS2CellData) {
	saveS2CellRecords(ctx, db, cells)
}
