// Package cashflowcalendar defines original-anchor monthly reporting boundaries.
package cashflowcalendar

import "time"

// MonthBoundary preserves index zero exactly. Subsequent anchors clamp the
// original day to the target month, choose the later instant in a repeated hour,
// and move nonexistent wall times forward by the clock gap. Every index is
// calculated from the original anchor, not from a previously adjusted boundary.
func MonthBoundary(start time.Time, index int) time.Time {
	if index == 0 {
		return start
	}
	// A fixed-offset calendar is used only for civil arithmetic, not to normalize
	// supplied instants. Resolve the resulting wall time in the anchor's location.
	civil := time.FixedZone("", 0)
	first := time.Date(start.Year(), start.Month()+time.Month(index), 1, 0, 0, 0, 0, civil)
	day := min(start.Day(), first.AddDate(0, 1, -1).Day())
	wall := time.Date(
		first.Year(), first.Month(), day, start.Hour(), start.Minute(), start.Second(), start.Nanosecond(), civil,
	)
	return resolveMonthWallTime(wall, start.Location())
}

func resolveMonthWallTime(wall time.Time, location *time.Location) time.Time {
	guess := time.Date(
		wall.Year(), wall.Month(), wall.Day(), wall.Hour(), wall.Minute(), wall.Second(), wall.Nanosecond(), location,
	)
	_, offset := guess.Zone()
	offsets := []int{offset}
	zoneStart, zoneEnd := guess.ZoneBounds()
	if !zoneStart.IsZero() {
		_, previous := zoneStart.Add(-time.Nanosecond).Zone()
		offsets = append(offsets, previous)
	}
	if !zoneEnd.IsZero() {
		_, next := zoneEnd.Zone()
		offsets = append(offsets, next)
	}
	var exact, forward, forwardWall time.Time
	for _, candidateOffset := range offsets {
		candidate := wall.Add(-time.Duration(candidateOffset) * time.Second).In(location)
		localWall := time.Date(
			candidate.Year(), candidate.Month(), candidate.Day(), candidate.Hour(), candidate.Minute(),
			candidate.Second(), candidate.Nanosecond(), wall.Location(),
		)
		if localWall.Equal(wall) && (exact.IsZero() || candidate.After(exact)) {
			exact = candidate
		}
		if localWall.After(wall) && (forward.IsZero() || localWall.Before(forwardWall)) {
			forward, forwardWall = candidate, localWall
		}
	}
	if !exact.IsZero() {
		return exact
	}
	return forward
}
