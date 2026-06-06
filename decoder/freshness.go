package decoder

import (
	"time"

	"github.com/guregu/null/v6"
)

type freshnessSource uint8

const (
	freshnessFallback freshnessSource = iota
	freshnessServer
)

type Freshness struct {
	timestampMs int64
	source      freshnessSource
}

func FallbackFreshness(timestampMs int64) Freshness {
	if timestampMs == 0 {
		timestampMs = time.Now().UnixMilli()
	}
	return Freshness{timestampMs: timestampMs, source: freshnessFallback}
}

func ServerFreshness(timestampMs, fallbackTimestampMs int64) Freshness {
	if timestampMs != 0 {
		return Freshness{timestampMs: timestampMs, source: freshnessServer}
	}
	return FallbackFreshness(fallbackTimestampMs)
}

func (f Freshness) TimestampMs() int64 {
	return f.timestampMs
}

func (f Freshness) Unix() int64 {
	return f.timestampMs / 1000
}

func (f Freshness) IsServer() bool {
	return f.source == freshnessServer
}

func (f Freshness) IsStaleFor(currentMs int64, newRecord bool) bool {
	return f.IsServer() && !newRecord && f.timestampMs < currentMs
}

func updatedMsToSeconds(updatedMs int64) int64 {
	return updatedMs / 1000
}

func updatedMsToNullSeconds(updatedMs null.Int) null.Int {
	if !updatedMs.Valid {
		return updatedMs
	}
	return null.IntFrom(updatedMs.Int64 / 1000)
}
