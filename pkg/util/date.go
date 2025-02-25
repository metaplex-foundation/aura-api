package util

import "time"

func GenerateDateRange(maxDay time.Time) []time.Time {
	var dates []time.Time
	today := time.Now().UTC().Truncate(24 * time.Hour)
	maxDay = maxDay.Truncate(24 * time.Hour)

	// start from the day after maxDay
	for d := maxDay.AddDate(0, 0, 1); d.Equal(today); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}

	return dates
}
