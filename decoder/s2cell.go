package decoder

import (
	"context"

	"golbat/db"

	"github.com/golang/geo/s2"
	"github.com/guregu/null/v6"
	"github.com/jellydator/ttlcache/v3"
	log "github.com/sirupsen/logrus"
)

type S2Cell struct {
	Id        uint64   `db:"id"`
	Latitude  float64  `db:"center_lat"`
	Longitude float64  `db:"center_lon"`
	Level     null.Int `db:"level"`
	UpdatedMs int64    `db:"updated_ms"`
}

// CREATE TABLE `weather` (
//  `id` bigint NOT NULL,
//  `level` tinyint unsigned DEFAULT NULL,
//  `center_lat` double(18,14) NOT NULL DEFAULT '0.00000000000000',
//  `center_lon` double(18,14) NOT NULL DEFAULT '0.00000000000000',
//  `updated` int unsigned NOT NULL,
//  PRIMARY KEY (`id`)
//)

func saveS2CellRecords(ctx context.Context, db db.DbDetails, cells []RawS2CellData) {
	// prepare list of cells to update
	for _, cell := range cells {
		cellId := cell.Cell
		freshness := ServerFreshness(cell.Timestamp, 0)
		var s2Cell *S2Cell

		if c := s2CellCache.Get(cellId); c != nil {
			cachedCell := c.Value()
			if freshness.IsStaleFor(cachedCell.UpdatedMs, false) {
				continue
			}
			if cachedCell.UpdatedMs > freshness.TimestampMs()-GetUpdateThreshold(900)*1000 {
				continue
			}
			s2Cell = cachedCell
		} else {
			mapS2Cell := s2.CellFromCellID(s2.CellID(cellId))
			s2Cell = &S2Cell{}
			s2Cell.Id = cellId
			s2Cell.Latitude = mapS2Cell.CapBound().RectBound().Center().Lat.Degrees()
			s2Cell.Longitude = mapS2Cell.CapBound().RectBound().Center().Lng.Degrees()
			s2Cell.Level = null.IntFrom(int64(mapS2Cell.Level()))

			s2CellCache.Set(s2Cell.Id, s2Cell, ttlcache.DefaultTTL)
		}
		s2Cell.UpdatedMs = freshness.TimestampMs()

		if dbDebugEnabled {
			log.Debugf("[DB_UPDATE] S2Cell Updated cell: %d", s2Cell.Id)
		}

		// Queue through the typed queue
		if s2cellQueue != nil {
			s2cellQueue.Enqueue(S2CellData{
				Id:        s2Cell.Id,
				Latitude:  s2Cell.Latitude,
				Longitude: s2Cell.Longitude,
				Level:     s2Cell.Level.ValueOrZero(),
				UpdatedMs: s2Cell.UpdatedMs,
			}, false, 0)
		}
	}
}
