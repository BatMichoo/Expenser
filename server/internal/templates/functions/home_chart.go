package functions

import (
	"expenser/internal/utilities"
	"time"
)

func DateNow() string {
	return time.Now().Format(utilities.DateFormats.HTML)
}

func DateOneMonthPrior() string {
	return time.Now().AddDate(0, -1, 0).Format(utilities.DateFormats.HTML)
}
