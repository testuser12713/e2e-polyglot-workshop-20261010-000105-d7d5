package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"workshop/backend/internal/store"
)

// TestCreateVehicleReturns201WithID checks that a valid vehicle is created and
// returned with a unique id (SPEC AC-02).
func TestCreateVehicleReturns201WithID(t *testing.T) {
	srv, st := newCustomerVehicleIntegrationServer(t)

	plate := "B-CV-" + cvUniqueSuffix()
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), "DELETE FROM vehicles WHERE plate = $1", plate)
	})

	body := fmt.Sprintf(`{"plate":%q,"make":"VW","model":"Golf","mileage":123456}`, plate)
	resp, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/vehicles", body)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", resp.StatusCode, raw)
	}

	var created store.Vehicle
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("decode vehicle: %v (body %s)", err, raw)
	}
	if created.ID <= 0 {
		t.Errorf("vehicle id = %d, want a positive id", created.ID)
	}
	if created.Plate != plate || created.Make != "VW" || created.Model != "Golf" || created.Mileage != 123456 {
		t.Errorf("vehicle = %+v, want the submitted fields", created)
	}
}

// TestCreateVehicleDuplicatePlateReturns409AndCreatesNoSecond checks that a
// second vehicle with the same plate is rejected with 409 and only one row
// exists (SPEC AC-02).
func TestCreateVehicleDuplicatePlateReturns409AndCreatesNoSecond(t *testing.T) {
	srv, st := newCustomerVehicleIntegrationServer(t)

	plate := "B-DUP-" + cvUniqueSuffix()
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), "DELETE FROM vehicles WHERE plate = $1", plate)
	})

	body := fmt.Sprintf(`{"plate":%q,"make":"VW","model":"Golf","mileage":1000}`, plate)

	first, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/vehicles", body)
	if first.StatusCode != http.StatusCreated {
		t.Fatalf("first create status = %d, want 201 (body %s)", first.StatusCode, raw)
	}

	second, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/vehicles", body)
	if second.StatusCode != http.StatusConflict {
		t.Fatalf("second create status = %d, want 409 (body %s)", second.StatusCode, raw)
	}

	var errBody struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &errBody); err != nil {
		t.Fatalf("decode error body: %v (body %s)", err, raw)
	}
	if errBody.Error.Code != "conflict" || errBody.Error.Message == "" {
		t.Errorf("error body = %+v, want a conflict code and a message", errBody.Error)
	}

	var count int
	if err := st.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM vehicles WHERE plate = $1", plate).Scan(&count); err != nil {
		t.Fatalf("count vehicles: %v", err)
	}
	if count != 1 {
		t.Errorf("vehicle count for plate %q = %d, want exactly 1", plate, count)
	}
}

// TestCreateVehicleInvalidReturns422 checks that a missing field or a negative
// mileage is rejected with 422.
func TestCreateVehicleInvalidReturns422(t *testing.T) {
	srv, _ := newCustomerVehicleIntegrationServer(t)

	cases := map[string]string{
		"missing plate":    `{"plate":"","make":"VW","model":"Golf","mileage":0}`,
		"missing make":     `{"plate":"B-XX-1","make":"","model":"Golf","mileage":0}`,
		"missing model":    `{"plate":"B-XX-1","make":"VW","model":"","mileage":0}`,
		"negative mileage": `{"plate":"B-XX-1","make":"VW","model":"Golf","mileage":-5}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			resp, raw := cvRequest(t, http.MethodPost, srv.URL+"/api/public/vehicles", body)
			if resp.StatusCode != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want 422 (body %s)", resp.StatusCode, raw)
			}
		})
	}
}
