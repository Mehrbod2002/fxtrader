# FXTrader

FXTrader is a multi-component forex trading platform that combines a Go backend API, a Python MetaTrader 5 bridge, and an MQL5 expert advisor for publishing market data. The system supports account management, trade execution, copy-trading, alerts, and WebSocket based streaming so it can power web or mobile trading clients.

## Project summary

- Goal: provide an end-to-end trading stack that can back a Profitilo listing or a standalone deployment.
- Tech stack: Go (Gin) for the REST/WebSocket API, MongoDB for persistence, Python for the MetaTrader bridge, and MQL5 for on-terminal price publishing.
- Deliverables: authenticated trading workflows, admin management flows, streaming ticks and trades, and deployment scaffolding for local or containerized environments.

## Components

- **Go backend (`backend/`)**
  - REST API built with Gin with public user flows, authenticated trading endpoints, and admin-only management routes.
  - MongoDB persistence for users, accounts, symbols, trades, alerts, copy-trade subscriptions, transactions, and audit logs.
  - WebSocket hub for streaming trades and price updates plus a dedicated socket server that connects to the MetaTrader bridge.
  - OpenAPI/Swagger docs served at `/swagger` with the spec available at `/docs/swagger.json`.
- **MetaTrader bridge (`metatrader/`)**
  - Python service that connects to an MT5 terminal, listens to backend WebSocket events, and manages trade lifecycle steps such as placement, modification, closing, and cleanup.
  - Periodically processes ticks and pings the backend to keep the trading channel alive.
- **MQL expert advisor (`mql/market.mq5`)**
  - Publishes bid/ask ticks for the active symbol to the backend price endpoint (default `https://api.crypex.org/api/v1/prices`).
  - Can be pointed to a locally running backend by updating the `BackendURL` input parameter.

## Features

- User signup/login with JWT-based authentication and optional admin login for elevated actions.
- Account creation, balance transfers, and referral tracking.
- Trade placement, modification, closing, and streaming, plus transaction approval/denial workflows for deposits and withdrawals.
- Symbol catalog management and rule configuration for risk controls.
- Alert creation and time-based alert processing with optional WebSocket notifications.
- Copy-trading subscriptions with leader request approvals and leader listings.
- Health check endpoint at `/health` for deployment monitoring.

## Quick start

1. Clone the repo and ensure Docker and Docker Compose are available.
2. Start the backend (includes MongoDB):

   ```bash
   cd backend
   docker-compose up --build
   ```

3. In another terminal, start the MetaTrader bridge once your MT5 terminal is running:

   ```bash
   cd metatrader
   python -m venv .venv
   source .venv/bin/activate
   pip install -r requirements.txt
   python main.py
   ```

4. Open Swagger at `http://localhost:7000/swagger/` to explore and test API routes.

## Configuration reference

### Backend environment

| Variable | Purpose | Default |
| --- | --- | --- |
| `ADDRESS` / `PORT` | HTTP bind address/port | `0.0.0.0` / `7000` |
| `MONGO_URI` | MongoDB connection string | `mongodb://admin:secret@mongodb:27017/?authSource=admin` |
| `ADMIN_USER` / `ADMIN_PASS` | Seeded admin credentials | `admin` / `admin` |
| `JWT_SECRET` | Token signing secret | `secret` |
| `MT5_HOST` / `MT5_PORT` | Location of the MetaTrader socket server | `mt5` / `1950` |
| `LISTEN_PORT` | Port exposed for the MetaTrader bridge WebSocket | `1950` |

### MetaTrader bridge settings

Configured in `metatrader/config/settings.py`:

- `APP_NAME` / `APP_VERSION`: metadata for logs and status.
- `BACKEND_WS_URL`: WebSocket URL for the backend socket server.
- `MT5_LOGIN`, `MT5_PASSWORD`, `MT5_SERVER`, `MT5_PATH`: connection info for the MT5 terminal.
- `PING_INTERVAL`: seconds between bridge-to-backend keepalive pings.

## Running the backend

### With Docker Compose

```bash
cd backend
docker-compose up --build
```

The backend listens on `0.0.0.0:7000` by default and connects to the bundled MongoDB instance. Adjust ports or credentials in `backend/docker-compose.yaml` as needed.

### With Go locally

1. Create a `.env` file in `backend/` (optional). Defaults are applied when variables are absent.
2. Set environment variables as needed:
   - `ADDRESS` (default `0.0.0.0`)
   - `PORT` (default `7000`)
   - `MONGO_URI` (default `mongodb://admin:secret@mongodb:27017/?authSource=admin`)
   - `ADMIN_USER` / `ADMIN_PASS` for the seeded admin account
   - `JWT_SECRET` for signing tokens
   - `MT5_HOST` / `MT5_PORT` to reach the MetaTrader socket server
   - `LISTEN_PORT` for the socket server that the MetaTrader bridge connects to
3. Run the server:

```bash
cd backend
go run ./cmd/server
```

Swagger UI is available at `http://localhost:7000/swagger/` once the server is running.

## Running the MetaTrader bridge

1. Install dependencies:

```bash
cd metatrader
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

2. Configure the connection to your MT5 terminal and backend by editing `metatrader/config/settings.py` if needed.
3. Start the service (ensure MT5 is running and reachable):

```bash
python main.py
```

The bridge maintains a WebSocket connection to the backend socket server (`LISTEN_PORT`) and drives trade execution in MT5.

## Using the MQL expert advisor

1. Open `mql/market.mq5` in MetaEditor.
2. Update the `BackendURL` input to point at your running backend price endpoint if different from the default.
3. Attach the EA to a chart with DLLs enabled to stream price ticks to the backend.

## Repository structure

- `backend/` – Go API server, middleware, services, repositories, WebSocket hub, and Swagger docs.
- `metatrader/` – Python MT5 bridge handling WebSocket events and trade orchestration.
- `mql/` – MQL5 expert advisor that posts live prices to the backend.
- `config/` and `k8s/` – Deployment configuration scaffolding.

## Additional notes

- The backend seeds an admin user at startup based on `ADMIN_USER`/`ADMIN_PASS` if it does not exist.
- All API routes are namespaced under `/api/v1`; use JWT tokens for authenticated user routes and the admin credentials for `/api/v1/admin/*` endpoints.
- Static chart assets (if present) are served from `/static`, and a `GET /chart` helper is provided to fetch `symbol.html` when available.
