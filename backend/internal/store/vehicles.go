package store

import (
	"context"
	"errors"
	"fmt"
)

// ErrVehiclePlateTaken reports that a vehicle with the same licence plate
// already exists (SPEC AC-02). The API answers 409 for it.
var ErrVehiclePlateTaken = errors.New("vehicle licence plate already registered")

// Vehicle is a customer vehicle. It is the shared shape returned by the public
// vehicle endpoint and nested in order details
// (Vehicle{id,plate,make,model,mileage}).
type Vehicle struct {
	ID      int64  `json:"id"`
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

// CreateVehicle inserts a vehicle and returns it with its assigned id. Every
// value travels as a bound parameter ($1..$4); no SQL is built from user input
// (SPEC AC-27). A duplicate plate is reported as ErrVehiclePlateTaken and no
// second row is created (SPEC AC-02).
func (s *Store) CreateVehicle(ctx context.Context, plate, make, model string, mileage int) (Vehicle, error) {
	const query = `
		INSERT INTO vehicles (plate, make, model, mileage)
		VALUES ($1, $2, $3, $4)
		RETURNING id, plate, make, model, mileage`

	var vehicle Vehicle
	err := s.Pool.QueryRow(ctx, query, plate, make, model, mileage).
		Scan(&vehicle.ID, &vehicle.Plate, &vehicle.Make, &vehicle.Model, &vehicle.Mileage)
	if err != nil {
		if isPgUniqueViolation(err) {
			return Vehicle{}, ErrVehiclePlateTaken
		}
		return Vehicle{}, fmt.Errorf("create vehicle: %w", err)
	}
	return vehicle, nil
}
