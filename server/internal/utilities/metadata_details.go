package utilities

import (
	"fmt"
	"strings"

	"expenser/internal/models"
)

// MetadataDetails is an expense's metadata (from UnmarshalMetadata) as one
// line of text for the expense list, empty when there is none. Formatting
// lives here, not on the models, because it translates and formats dates.
func MetadataDetails(metadata any, lang string) string {
	switch m := metadata.(type) {
	// House
	case models.ElectricityMetadata:
		return fmt.Sprintf("Day: %.2f (%.4f/kWh), Night: %.2f (%.4f/kWh)", m.KWhDay, m.PriceDay, m.KWhNight, m.PriceNight)
	case models.WaterMetadata:
		return fmt.Sprintf("Qty: %.2f m³ (%.2f/m³)", m.Quantity, m.PricePerUnit)
	case models.GasMetadata:
		return fmt.Sprintf("Qty: %.2f (%.2f/unit)", m.Quantity, m.PricePerUnit)
	case models.InternetTVMetadata:
		return fmt.Sprintf("%s - %s", m.Provider, m.Plan)
	// Car
	case models.FuelMetadata:
		return fmt.Sprintf("%.2f L (%.2f/L)", m.Liters, m.PricePerLiter)
	case models.MaintenanceMetadata:
		parts := make([]string, len(m.Parts))
		for i, part := range m.Parts {
			parts[i] = T(lang, "parts."+part)
		}
		return fmt.Sprintf("Part: %s, Labor: %.2f", strings.Join(parts, ", "), m.LaborCost)
	case models.InsuranceMetadata:
		return fmt.Sprintf("%s (%s) until %s", m.Provider, m.CoverageType, m.ValidUntil.Format(DateFormats.Input))
	case models.ParkingTollsMetadata:
		return fmt.Sprintf("%s (%s)", m.Location, m.Duration)
	// Both
	case models.OtherMetadata:
		return m.CustomName
	}
	return ""
}
