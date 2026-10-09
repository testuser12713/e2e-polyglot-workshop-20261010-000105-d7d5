# Kfz-Werkstatt-Kundenportal

Ein Kundenportal für eine Kfz-Werkstatt: Kundinnen und Kunden fragen online einen
Werkstatttermin an, verfolgen den Status ihres Auftrags mit Auftragsnummer und
Kennzeichen und sehen die Rechnung ein. Werkstattmitarbeiter melden sich an und
bearbeiten die Aufträge in einer Auftragsliste mit Statusfilter und
Kennzeichensuche, erfassen Arbeitszeit- und Teile-Positionen, setzen den Status
und sehen die wichtigsten Kennzahlen im Dashboard. Das Produkt besteht aus drei
eigenständig laufenden Diensten — einer Werkstatt-API in Go, einem
Rechnungs-Worker in Python und einer Web-App mit Vite/React/TypeScript —, die
gemeinsam PostgreSQL und Valkey nutzen.

## Tech-Stack

- **API** (`backend/`): Go, `net/http` aus der Standardbibliothek, `pgx` für
  PostgreSQL, `valkey-go` für die Warteschlange, `bcrypt` für Passwort-Hashes.
- **Worker** (`worker/`): Python; zieht Fertigmeldungen aus Valkey und erzeugt
  Rechnungen.
- **Web-App** (`frontend/`): Vite, React, TypeScript.
- **Datenbank**: PostgreSQL 18.
- **Warteschlange**: Valkey 9.1.
- **Start**: alle Dienste werden über `RUN.json` gestartet.

## Installation

Voraussetzungen: Go (aktuelle Version) und Docker für die lokalen Datenbanken.

```bash
# 1. PostgreSQL und Valkey lokal starten (gleiche Images wie im Betrieb)
docker compose up -d

# 2. Konfiguration setzen. Kein Geheimnis steht im Repository:
export DATABASE_URL="postgres://workshop:workshop@127.0.0.1:5432/workshop?sslmode=disable"
export VALKEY_URL="redis://127.0.0.1:6379/0"
export API_PORT="8080"
export CORS_ALLOWED_ORIGIN="http://localhost:5173"
export WORKSHOP_HOURLY_RATE_CENTS="9500"
export BOOTSTRAP_EMPLOYEE_EMAIL="werkstatt@example.de"
export BOOTSTRAP_EMPLOYEE_PASSWORD="$(openssl rand -hex 16)"   # Geheimnis, pro Lauf erzeugt
export BOOTSTRAP_EMPLOYEE_NAME="Werkstatt-Team"
export LOGIN_RATE_LIMIT_PER_MINUTE="10"

# 3. Abhängigkeiten laden und die API starten
cd backend
go mod download
go run .
```

Beim Start legt die API das Datenbankschema automatisch an und gibt den Port aus,
auf dem sie lauscht (`workshop API listening on port 8080`). Ohne erreichbares
PostgreSQL meldet der Dienst den Fehler und beendet sich — es gibt **keinen**
SQLite- oder In-Memory-Ersatz.

## Starten mit `RUN.json`

`RUN.json` beschreibt jede startbare Komponente maschinenlesbar. Der
Startvertrag ist:

| Baustein | Typ | Zweck |
| --- | --- | --- |
| `db` | database (postgres) | PostgreSQL 18 |
| `queue` | database (valkey) | Valkey 9.1 |
| `api` | server (go) | Werkstatt-API, Port aus `API_PORT`, Health `GET /api/health` |

Konfiguration und Geheimnisse kommen ausschließlich aus der Umgebung
(`DATABASE_URL`, `VALKEY_URL`, `BOOTSTRAP_EMPLOYEE_PASSWORD` u. a.). Das
Bootstrap-Passwort wird pro Lauf erzeugt und nie im Repository abgelegt.

## Produktions-Build

```bash
cd backend
go build -o workshop-api .
./workshop-api
```

## Verwendung — API

Basis-URL: `http://localhost:8080`. Alle Beträge sind ganzzahlige Cent. Jede
fehlgeschlagene Antwort hat denselben Fehlerkörper:

```json
{"error":{"code":"not_found","message":"Zu diesen Angaben wurde kein Auftrag gefunden."}}
```

### Öffentlich (Kundenbereich)

| Methode | Pfad | Body | Antwort |
| --- | --- | --- | --- |
| `POST` | `/api/public/customers` | `{name,email,phone}` | `201` Customer; `409` E-Mail vergeben; `422` ungültig |
| `GET` | `/api/public/customers/{id}` | — | `200` Customer; `404` |
| `POST` | `/api/public/vehicles` | `{plate,make,model,mileage}` | `201` Vehicle; `409` Kennzeichen vergeben |
| `POST` | `/api/public/orders` | `{customer,vehicle,desired_date,problem,items[]}` | `201` `{order_number,status:"requested"}`; `422` |
| `GET` | `/api/public/orders/{order_number}?plate=<plate>` | — | `200` `{order,invoice}`; unbekannt oder falsches Kennzeichen: `404` |
| `GET` | `/api/public/orders/{order_number}/invoice?plate=<plate>` | — | `200` Invoice; `404` |

Beispiel — Auftrag anfragen:

```bash
curl -X POST http://localhost:8080/api/public/orders \
  -H 'Content-Type: application/json' \
  -d '{
    "customer": {"name":"Anna Beispiel","email":"anna@example.de","phone":"0151 1234567"},
    "vehicle":  {"plate":"M-AB 1234","make":"VW","model":"Golf","mileage":120000},
    "desired_date": "2025-03-07",
    "problem": "Bremsen quietschen vorne.",
    "items": [
      {"kind":"labor","description":"Bremsen prüfen","quantity":1,"hours":1.5,"unit_price_cents":9500}
    ]
  }'
```

```json
{"order_number":"AW-2025-000123","status":"requested"}
```

### Werkstattbereich (Bearer-Token)

Alle `/api/shop/*`-Endpunkte außer dem Login erwarten den Header
`Authorization: Bearer <token>`. Fehlt ein gültiges Token, antwortet die API mit
`401` im einheitlichen Fehlerkörper.

| Methode | Pfad | Body | Antwort |
| --- | --- | --- | --- |
| `POST` | `/api/shop/login` | `{email,password}` | `200` `{token,employee}`; `401`; `429` ratenbegrenzt |
| `GET` | `/api/shop/orders?status=<status>&plate=<plate>` | — | `200` `{orders:[OrderSummary]}` |
| `GET` | `/api/shop/orders/{order_number}` | — | `200` OrderDetail |
| `POST` | `/api/shop/orders/{order_number}/status` | `{status}` | `200` `{order_number,status}`; `409` unerlaubter Übergang |
| `POST` | `/api/shop/orders/{order_number}/items` | ItemInput | `201` OrderItem; `422` |
| `GET` | `/api/shop/dashboard` | — | `200` `{open_orders,finished_today,revenue_month_cents}` |
| `GET` | `/api/health` | — | `200` `{status:"ok"}` |

Statuswerte: `requested`, `confirmed`, `in_progress`, `done`, `picked_up`
(UI-Beschriftungen „angefragt", „bestätigt", „in Arbeit", „fertig", „abgeholt").

### Datentypen (Auszug)

- `Customer`: `{id,name,email,phone}`
- `Vehicle`: `{id,plate,make,model,mileage}`
- `ItemInput`: `{kind:"labor"|"part",description,quantity,hours,unit_price_cents}`
- `OrderItem`: `{id,kind,description,quantity,hours,unit_price_cents,amount_cents}`
- `Invoice`: `{invoice_number,order_number,items[],net_cents,tax_cents,gross_cents,created_at}`

## Funktionen

- Öffentliche Terminanfrage mit Kunden- und Fahrzeugdaten, Wunschtermin,
  Problembeschreibung und ersten Positionen.
- Statusabfrage per Auftragsnummer **und** Kennzeichen samt Statusverlauf.
- Rechnungseinsicht im Kundenbereich (Positionen, Netto, 19 % MwSt., Brutto).
- Werkstatt-Login mit gehashten Passwörtern, Sitzungen und Ratenbegrenzung.
- Auftragsliste mit Statusfilter und Kennzeichensuche, Statuswechsel nach
  erlaubten Übergängen, Erfassung von Arbeitszeit- und Teile-Positionen.
- Dashboard mit offenen Aufträgen, heute fertiggestellten Aufträgen und
  Monatsumsatz.
- Fertigmeldungen landen in der Valkey-Liste `workshop:completed_orders`; der
  Rechnungs-Worker erzeugt daraus die Rechnung und eine Postausgang-Meldung.

## Tests

```bash
cd backend
go test ./...
```

Die Handler-Tests laufen gegen eine echte PostgreSQL-Instanz (`DATABASE_URL`).
Ohne gesetzte `DATABASE_URL` wird der Datenbank-Integrationstest übersprungen.
