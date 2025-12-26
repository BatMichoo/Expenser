package models

import "time"

type SearchResultsInput struct {
	Date time.Time `form:"date" binding:"required" time_format:"01/02/2006"`
}
