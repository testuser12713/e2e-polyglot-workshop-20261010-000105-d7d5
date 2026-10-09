package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"workshop/backend/internal/store"
)

// ItemInput is one position of an order request. It is part of the shared API
// contract and is reused by the workshop position routes.
type ItemInput struct {
	Kind           string  `json:"kind"`
	Description    string  `json:"description"`
	Quantity       float64 `json:"quantity"`
	Hours          float64 `json:"hours"`
	UnitPriceCents int64   `json:"unit_price_cents"`
}

type orderCustomerInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type orderVehicleInput struct {
	Plate   string `json:"plate"`
	Make    string `json:"make"`
	Model   string `json:"model"`
	Mileage int    `json:"mileage"`
}

type createOrderRequest struct {
	Customer    orderCustomerInput `json:"customer"`
	Vehicle     orderVehicleInput  `json:"vehicle"`
	DesiredDate string             `json:"desired_date"`
	Problem     string             `json:"problem"`
	Items       []ItemInput        `json:"items"`
}

type createOrderResponse struct {
	OrderNumber string `json:"order_number"`
	Status      string `json:"status"`
}

// handleCreateOrder serves POST /api/public/orders. A complete request resolves
// or creates the customer and the vehicle, stores the order with its positions
// and the first status-log entry in one transaction, and answers 201 with the
// freshly generated order number and the status "requested" (SPEC AC-03).
func (s *Server) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	if s == nil || s.Store == nil || s.Store.Pool == nil {
		internalError(w, "Auftrag konnte nicht angelegt werden. Bitte später erneut versuchen.")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var req createOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		unprocessable(w, "Die Anfrage ist ungültig. Bitte alle Pflichtfelder ausfüllen.")
		return
	}

	desiredDate, validationMessage := validateOrderRequest(req)
	if validationMessage != "" {
		unprocessable(w, validationMessage)
		return
	}

	items := make([]store.OrderItemInput, 0, len(req.Items))
	for _, item := range req.Items {
		items = append(items, store.OrderItemInput{
			Kind:           item.Kind,
			Description:    item.Description,
			Quantity:       item.Quantity,
			Hours:          item.Hours,
			UnitPriceCents: item.UnitPriceCents,
		})
	}

	hourlyRateCents := 0
	if s.Config != nil {
		hourlyRateCents = s.Config.WorkshopHourlyRateCents
	}

	params := store.CreateOrderParams{
		Customer: store.CustomerInput{
			Name:  strings.TrimSpace(req.Customer.Name),
			Email: strings.TrimSpace(req.Customer.Email),
			Phone: strings.TrimSpace(req.Customer.Phone),
		},
		Vehicle: store.VehicleInput{
			Plate:   strings.TrimSpace(req.Vehicle.Plate),
			Make:    strings.TrimSpace(req.Vehicle.Make),
			Model:   strings.TrimSpace(req.Vehicle.Model),
			Mileage: req.Vehicle.Mileage,
		},
		DesiredDate:     desiredDate,
		Problem:         strings.TrimSpace(req.Problem),
		Items:           items,
		HourlyRateCents: int64(hourlyRateCents),
	}

	created, err := s.Store.CreateOrder(r.Context(), params)
	if err != nil {
		// Never log the error value: a PostgreSQL constraint error can echo the
		// conflicting e-mail or plate. Only the status line reports the failure
		// (SPEC AC-33).
		log.Printf("create public order: database error on %s", safePath(r.URL.Path))
		internalError(w, "Auftrag konnte nicht angelegt werden. Bitte später erneut versuchen.")
		return
	}

	writeJSON(w, http.StatusCreated, createOrderResponse{
		OrderNumber: created.OrderNumber,
		Status:      created.Status,
	})
}

// validateOrderRequest checks the submitted order request and returns the parsed
// desired date. The second result is a German validation message, empty when
// the request is valid.
func validateOrderRequest(req createOrderRequest) (time.Time, string) {
	if strings.TrimSpace(req.Customer.Name) == "" {
		return time.Time{}, "Bitte den Namen angeben."
	}
	email := strings.TrimSpace(req.Customer.Email)
	if email == "" || !strings.Contains(email, "@") {
		return time.Time{}, "Bitte eine gültige E-Mail-Adresse angeben."
	}
	if strings.TrimSpace(req.Customer.Phone) == "" {
		return time.Time{}, "Bitte eine Telefonnummer angeben."
	}
	if strings.TrimSpace(req.Vehicle.Plate) == "" {
		return time.Time{}, "Bitte das Kennzeichen angeben."
	}
	if strings.TrimSpace(req.Vehicle.Make) == "" {
		return time.Time{}, "Bitte die Marke angeben."
	}
	if strings.TrimSpace(req.Vehicle.Model) == "" {
		return time.Time{}, "Bitte das Modell angeben."
	}
	if req.Vehicle.Mileage < 0 {
		return time.Time{}, "Der Kilometerstand darf nicht negativ sein."
	}

	desiredDate, err := time.Parse("2006-01-02", strings.TrimSpace(req.DesiredDate))
	if err != nil {
		return time.Time{}, "Bitte einen gültigen Wunschtermin (JJJJ-MM-TT) angeben."
	}

	for _, item := range req.Items {
		if item.Kind != "labor" && item.Kind != "part" {
			return time.Time{}, "Eine Position hat eine ungültige Art."
		}
		if strings.TrimSpace(item.Description) == "" {
			return time.Time{}, "Bitte jede Position beschreiben."
		}
		if item.Quantity < 0 || item.Hours < 0 || item.UnitPriceCents < 0 {
			return time.Time{}, "Positionen dürfen keine negativen Werte enthalten."
		}
	}

	return desiredDate, ""
}
