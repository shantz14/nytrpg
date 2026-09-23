package game

import (
	"sync/atomic"
	"time"
)

// Counters for monitoring, safe to read from any goroutine
type Stats struct {
	Ticks          atomic.Int64
	TotalTickNanos atomic.Int64
	LastTickNanos  atomic.Int64
	MaxTickNanos   atomic.Int64
	BytesOut       atomic.Int64
	RejectedMoves  atomic.Int64
	Panics         atomic.Int64
}

func (s *Stats) recordTick(d time.Duration) {
	s.Ticks.Add(1)
	s.TotalTickNanos.Add(int64(d))
	s.LastTickNanos.Store(int64(d))
	if int64(d) > s.MaxTickNanos.Load() {
		s.MaxTickNanos.Store(int64(d))
	}
}
