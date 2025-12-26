package utilities

type DateFormat struct {
	Input     string
	MonthOnly string
	Output    string
	HTML      string
}

var DateFormats = DateFormat{
	Input:     "2006-01-02",
	MonthOnly: "2006-01",
	Output:    "01.02.2006",
	HTML:      "2006-01-02",
}
