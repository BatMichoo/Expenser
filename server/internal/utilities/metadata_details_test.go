package utilities

import (
	"testing"
	"time"

	"expenser/internal/models"
)

func TestMetadataDetails(t *testing.T) {
	cases := map[string]struct {
		in   any
		want string
	}{
		"none":      {nil, ""},
		"fuel":      {models.FuelMetadata{Liters: 40, PricePerLiter: 1.25}, "40.00 L (1.25/L)"},
		"water":     {models.WaterMetadata{Quantity: 3, PricePerUnit: 2}, "Qty: 3.00 m³ (2.00/m³)"},
		"internet":  {models.InternetTVMetadata{Provider: "A1", Plan: "Max"}, "A1 - Max"},
		"insurance": {models.InsuranceMetadata{Provider: "X", CoverageType: "Full", ValidUntil: time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC)}, "X (Full) until " + time.Date(2027, 1, 2, 0, 0, 0, 0, time.UTC).Format(DateFormats.Input)},
		"other":     {models.OtherMetadata{CustomName: "Bike rack"}, "Bike rack"},
		"unknown":   {struct{}{}, ""},
	}
	for name, tc := range cases {
		if got := MetadataDetails(tc.in, "en"); got != tc.want {
			t.Errorf("%s: got %q, want %q", name, got, tc.want)
		}
	}
}
