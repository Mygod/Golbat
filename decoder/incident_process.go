package decoder

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"

	"golbat/db"
	"golbat/pogo"
)

func UpdateIncidentLineup(ctx context.Context, db db.DbDetails, protoReq *pogo.OpenInvasionCombatSessionProto, protoRes *pogo.OpenInvasionCombatSessionOutProto, freshness Freshness) string {
	incident, unlock, err := getOrCreateIncidentRecord(ctx, db, protoReq.IncidentLookup.IncidentId, protoReq.IncidentLookup.FortId, "UpdateIncidentWithConfirmation")
	if err != nil {
		return fmt.Sprintf("getOrCreateIncidentRecord: %s", err)
	}
	defer unlock()

	if incident.newRecord {
		log.Debugf("Updating lineup before it was saved: %s", protoReq.IncidentLookup.IncidentId)
	}
	if freshness.IsStaleFor(incident.UpdatedMs, incident.IsNewRecord()) {
		return ""
	}
	incident.updateFromOpenInvasionCombatSessionOut(protoRes)

	saveIncidentRecordWithFreshness(ctx, db, incident, freshness)
	return ""
}

func UpdateIncidentLineupFromBattleState(ctx context.Context, db db.DbDetails, fortId, incidentId string, out *pogo.BattleStateOutProto) string {
	incident, unlock, err := getOrCreateIncidentRecord(ctx, db, incidentId, fortId, "UpdateIncidentLineupFromBattleState")
	if err != nil {
		return fmt.Sprintf("getOrCreateIncidentRecord: %s", err)
	}
	defer unlock()

	incident.updateFromBattleState(out)
	saveIncidentRecordWithFreshness(ctx, db, incident, FallbackFreshness(0))
	return ""
}

func ConfirmIncident(ctx context.Context, db db.DbDetails, proto *pogo.StartIncidentOutProto, freshness Freshness) string {
	incident, unlock, err := getOrCreateIncidentRecord(ctx, db, proto.Incident.IncidentId, proto.Incident.FortId, "UpdateIncidentFromInvasion")
	if err != nil {
		return fmt.Sprintf("getOrCreateIncidentRecord: %s", err)
	}
	defer unlock()

	if incident.newRecord {
		log.Debugf("Confirming incident before it was saved: %s", proto.Incident.IncidentId)
	}
	incident.updateFromStartIncidentOut(proto)

	saveIncidentRecordWithFreshness(ctx, db, incident, freshness)
	return ""
}
