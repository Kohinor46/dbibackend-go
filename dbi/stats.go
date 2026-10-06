package dbi

import (
	"fmt"
	"log/slog"
	"time"
)

// stats splits the active part of a session (first FILE_RANGE to the end of
// the last one) into "PC is sending", the handshake, and "waiting for the
// Switch", which tells whether a slow install is limited by USB/PC or by the
// console (SD card write speed, NSZ decompression). Idle time before the first
// and after the last range (menus, finished install left connected) is
// reported separately so it doesn't skew the speed.
type stats struct {
	start     time.Time
	first     time.Time // start of the first FILE_RANGE
	last      time.Time // end of the last FILE_RANGE
	ranges    int
	bytes     int64
	minRange  int64
	maxRange  int64
	sending   time.Duration // FILE_RANGE data phases
	handshake time.Duration // FILE_RANGE ACK/header/RESPONSE/ACK exchange before data
}

// addRange records one FILE_RANGE: begin is when the command arrived,
// dataStart when its data phase began, end when the data was sent.
func (s *stats) addRange(size int64, begin, dataStart, end time.Time) {
	if s.ranges == 0 {
		s.first = begin
		s.minRange = size
	}
	s.last = end
	s.minRange = min(s.minRange, size)
	s.maxRange = max(s.maxRange, size)
	s.ranges++
	s.bytes += size
	s.handshake += dataStart.Sub(begin)
	s.sending += end.Sub(dataStart)
}

func (s *stats) log(l *slog.Logger) {
	if s.ranges == 0 {
		return
	}
	active := s.last.Sub(s.first)
	l.Info("Session stats",
		"sent_MB", s.bytes>>20,
		"ranges", s.ranges,
		"range_KB_min/avg/max", []int64{s.minRange >> 10, s.bytes / int64(s.ranges) >> 10, s.maxRange >> 10},
		"avg_MB/s", mbps(s.bytes, active),
		"usb_MB/s", mbps(s.bytes, s.sending),
		"time_data", pct(s.sending, active),
		"time_handshake", pct(s.handshake, active),
		"waiting_for_switch", pct(active-s.sending-s.handshake, active),
		"active", active.Round(time.Second),
		"idle", (time.Since(s.start) - active).Round(time.Second),
	)
}

func mbps(n int64, d time.Duration) string {
	if d <= 0 {
		return "0"
	}
	return fmt.Sprintf("%.1f", float64(n)/d.Seconds()/(1<<20))
}

func pct(part, total time.Duration) string {
	if total <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%d%%", part*100/total)
}
