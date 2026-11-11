# Frontend Prototype (M4 Kickoff)

This directory contains an early Next.js dashboard prototype.

## Implemented

- Setup status check
- One-time enrollment request
- Login and logout
- Account security operations:
   - list active sessions
   - revoke session
   - re-authenticate
   - change password
- Read-only dashboard snapshot:
  - current user/session summary
  - configuration status
  - websites list
  - policies list
- Events page scaffold:
   - filter form (site/action/rule/time range)
   - API query wiring to `/api/v1/events`
   - clear unavailable-state messaging when backend events API is not yet implemented

## Run locally

1. Start the Go API in another terminal from repository root:

   ```sh
   go run ./cmd/ariba -dev-http -origin http://localhost:3000
   ```

2. In this directory:

   ```sh
   npm install
   npm run dev
   ```

3. Open:

- `http://localhost:3000/`
- `http://localhost:3000/setup`
- `http://localhost:3000/dashboard`
- `http://localhost:3000/account`
- `http://localhost:3000/events`

## Notes

- This is not the final integrated runtime UI.
- Activation/jobs, events pages, certificate workflows, and admin management pages are still pending.
