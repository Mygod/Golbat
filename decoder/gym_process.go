package decoder

import (
	"context"
	"fmt"

	"github.com/guregu/null/v6"

	"golbat/db"
	"golbat/pogo"
)

func UpdateGymRecordWithFortDetailsOutProto(ctx context.Context, db db.DbDetails, fort *pogo.FortDetailsOutProto, freshness Freshness) string {
	gym, unlock, err := getOrCreateGymRecord(ctx, db, fort.Id, "UpdateGymFromFortDetails")
	if err != nil {
		return err.Error()
	}
	defer unlock()

	gym.updateGymFromFortProto(fort)

	updateGymGetMapFortCache(gym, true)
	saveGymRecordWithFreshness(ctx, db, gym, freshness)

	return fmt.Sprintf("%s %s", gym.Id, gym.Name.ValueOrZero())
}

func UpdateGymRecordWithGymInfoProto(ctx context.Context, db db.DbDetails, gymInfo *pogo.GymGetInfoOutProto, freshness Freshness) string {
	gym, unlock, err := getOrCreateGymRecord(ctx, db, gymInfo.GymStatusAndDefenders.PokemonFortProto.FortId, "UpdateGymFromGymInfo")
	if err != nil {
		return err.Error()
	}
	defer unlock()

	if freshness.IsStaleFor(gym.UpdatedMs, gym.IsNewRecord()) {
		return fmt.Sprintf("%s stale GymInfo", gym.Id)
	}
	gym.updateGymFromGymInfoOutProto(gymInfo, freshness.TimestampMs())

	updateGymGetMapFortCache(gym, true)
	saveGymRecordWithFreshness(ctx, db, gym, freshness)
	return fmt.Sprintf("%s %s", gym.Id, gym.Name.ValueOrZero())
}

func UpdateGymRecordWithGetMapFortsOutProto(ctx context.Context, db db.DbDetails, mapFort *pogo.GetMapFortsOutProto_FortProto, freshness Freshness) (bool, string) {
	gym, unlock, err := getGymRecordForUpdate(ctx, db, mapFort.Id, "UpdateGymFromGetMapForts")
	if err != nil {
		return false, err.Error()
	}

	// we missed it in Pokestop & Gym. Lets save it to cache
	if gym == nil {
		return false, ""
	}
	defer unlock()

	gym.updateGymFromGetMapFortsOutProto(mapFort, false)
	saveGymRecordWithFreshness(ctx, db, gym, freshness)
	return true, fmt.Sprintf("%s %s", gym.Id, gym.Name.ValueOrZero())
}

func UpdateGymRecordWithRsvpProto(ctx context.Context, db db.DbDetails, req *pogo.RaidDetails, resp *pogo.GetEventRsvpsOutProto, freshness Freshness) string {
	gym, unlock, err := getGymRecordForUpdate(ctx, db, req.FortId, "UpdateGymWithRsvp")
	if err != nil {
		return err.Error()
	}

	if gym == nil {
		// Do not add RSVP details to unknown gyms
		return fmt.Sprintf("%s Gym not present", req.FortId)
	}
	defer unlock()

	gym.updateGymFromRsvpProto(resp)

	saveGymRecordWithFreshness(ctx, db, gym, freshness)

	return fmt.Sprintf("%s %s", gym.Id, gym.Name.ValueOrZero())
}

func ClearGymRsvp(ctx context.Context, db db.DbDetails, fortId string, freshness Freshness) string {
	gym, unlock, err := getGymRecordForUpdate(ctx, db, fortId, "ClearGymRsvp")
	if err != nil {
		return err.Error()
	}

	if gym == nil {
		// Do not add RSVP details to unknown gyms
		return fmt.Sprintf("%s Gym not present", fortId)
	}
	defer unlock()

	if gym.Rsvps.Valid {
		gym.SetRsvps(null.NewString("", false))

		saveGymRecordWithFreshness(ctx, db, gym, freshness)
	}

	return fmt.Sprintf("%s %s", gym.Id, gym.Name.ValueOrZero())
}
