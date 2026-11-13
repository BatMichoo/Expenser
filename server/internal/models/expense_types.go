package models

import (
	"encoding/json"
	"log"
	"reflect"
)

// House utility types: utility_types.id as seeded by the migrations. The
// metadata, forms and translations are keyed by these IDs.
const (
	UtilityElectricity = 1
	UtilityWater       = 2
	UtilityGas         = 3
	UtilityInternet    = 4
	UtilityTV          = 5
	UtilityWaste       = 6
	UtilityOther       = 7
)

// Car expense types: car_expense_types.id as seeded by the migrations.
const (
	CarFuel         = 1
	CarMaintenance  = 2
	CarInsurance    = 3
	CarWash         = 4
	CarParkingTolls = 5
	CarOther        = 6
	CarOil          = 7
	CarTires        = 8
)

// GroceryCategoryOther is the "Other" grocery category, used when a
// receipt item has none.
const GroceryCategoryOther = 7

// newHouseMetadata returns a pointer to an empty metadata struct for the
// utility type, or nil when the type records no metadata.
func newHouseMetadata(typeID int) any {
	switch typeID {
	case UtilityElectricity:
		return &ElectricityMetadata{}
	case UtilityWater:
		return &WaterMetadata{}
	case UtilityGas:
		return &GasMetadata{}
	case UtilityInternet, UtilityTV:
		return &InternetTVMetadata{}
	case UtilityOther:
		return &OtherMetadata{}
	}
	return nil
}

// newCarMetadata is newHouseMetadata for car expense types.
func newCarMetadata(typeID int) any {
	switch typeID {
	case CarFuel:
		return &FuelMetadata{}
	case CarMaintenance:
		return &MaintenanceMetadata{}
	case CarInsurance:
		return &InsuranceMetadata{}
	case CarParkingTolls:
		return &ParkingTollsMetadata{}
	case CarOther:
		return &OtherMetadata{}
	}
	return nil
}

// decodeMetadata unmarshals raw into target (a pointer from newHouseMetadata
// or newCarMetadata) and returns the struct value, or nil when there is no
// metadata or it cannot be read. Unreadable metadata is logged: the expense
// still shows, without its details.
func decodeMetadata(raw Metadata, target any) any {
	if len(raw) == 0 || target == nil {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		log.Printf("expense metadata %s: %v", raw, err)
		return nil
	}
	return reflect.ValueOf(target).Elem().Interface()
}
