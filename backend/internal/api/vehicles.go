package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"workshop/backend/internal/store"
)

// vehicleInput is the JSON body of POST /api/public/vehicles.
type vehicleInput struct {
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

// handleCreateVehicle serves POST /api/public/vehicles. It creates a vehicle
// and answers 201 with the new id; an already used plate answers 409 and no
// second vehicle is created; an invalid body answers 422 (SPEC AC-02).
func (s *Server) handleCreateVehicle(w http.ResponseWriter, r *http.Request) {
	if s.Store == nil {
		internalError(w, "Das Fahrzeug konnte nicht angelegt werden.")
		return
	}

	var in vehicleInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		unprocessable(w, "Die Anfrage ist kein gültiges JSON.")
		return
	}
	in.Plate = strings.TrimSpace(in.Plate)
	in.Make = strings.TrimSpace(in.Make)
	in.Model = strings.TrimSpace(in.Model)

	if message := validateVehicleInput(in); message != "" {
		unprocessable(w, message)
		return
	}

	vehicle, err := s.Store.CreateVehicle(r.Context(), in.Plate, in.Make, in.Model, in.Mileage)
	if err != nil {
		if errors.Is(err, store.ErrVehiclePlateTaken) {
			conflict(w, "Dieses Kennzeichen ist bereits registriert.")
			return
		}
		logStoreFailure("create vehicle", err)
		internalError(w, "Das Fahrzeug konnte nicht angelegt werden.")
		return
	}
	writeJSON(w, http.StatusCreated, vehicle)
}

// validateVehicleInput returns a German message for the first missing or
// invalid field, or an empty string when the input is valid.
func validateVehicleInput(in vehicleInput) string {
	if in.Plate == "" {
		return "Bitte ein Kennzeichen angeben."
	}
	if in.Make == "" {
		return "Bitte eine Marke angeben."
	}
	if in.Model == "" {
		return "Bitte ein Modell angeben."
	}
	if in.Mileage < 0 {
		return "Der Kilometerstand darf nicht negativ sein."
	}
	return ""
}
