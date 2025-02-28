package util

import "time"

// generates range of dates beginning from `startDay` and until yesterday
func GenerateDateRange(startDay time.Time) []time.Time {
	var dates []time.Time
	today := time.Now().UTC().Truncate(24 * time.Hour)
	startDay = startDay.Truncate(24 * time.Hour)

	for d := startDay; d.Before(today); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}

	return dates
}
