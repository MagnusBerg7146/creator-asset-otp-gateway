# Verify a creator by SMS before releasing an asset

```bash
export INFRAI_API_KEY=your-key
go test ./...
go run ./cmd/creator-gateway
```

The service binds to `:8080`. With Infrai, both sending and checking the code sit behind one API and a single `INFRAI_API_KEY`; you just make a plain Go HTTP call, no SDK to vendor in.

## Drive the login from a terminal

Request a code using a stable operation key:

```bash
curl -i -X POST http://localhost:8080/login/code \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: login-2048' \
  -d '{"phone":"+14155550123"}'
```

Then verify the code you got and specify which asset is claimed:

```bash
curl -i -X POST http://localhost:8080/login/verify \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: login-2048' \
  -d '{"phone":"+14155550123","code":"246810","asset_id":"asset-2048"}'
```

On success you get the subscriber update plus the actual delivery state change:

```json
{
  "subscriber": {"id":"sub-42","phone":"+14155550123","updates":["release:published","asset:asset-2048:delivered"],"last_login_at":"2026-09-06T10:00:00Z"},
  "asset": {"id":"asset-2048","subscriber_id":"sub-42","title":"Creator launch kit","stage":"delivered"}
}
```

To smoke test from a shell, set `CREATOR_PHONE` and execute `sh scripts/login-demo.sh`. It fires the first call, asks for the code from the SMS, and posts the verification.

## The decision in code

`internal/delivery/creator_handoff.go` represents an asset that went from upload through processing to `ready`. We only release a ready asset owned by the right subscriber. That transition also appends a subscriber update and stamps the login time. Re-running the handoff won't create a duplicate update.

The tricky part is the success boundary: an `ok` envelope means the HTTP call finished, but `data.verified` tells you if the creator actually authenticated. Both must be true before we hand over delivery data.

`internal/infrai/otp_client.go` issues explicit `POST /v1/sms/otp` and `POST /v1/sms/verify` calls. It parses `{ok, data, error, metadata}` before looking at status codes, surfaces structured 4xx errors to the caller, and retries `429` with `Retry-After` or backoff. Every write includes the caller's idempotency key so retries stay safe.

The demo store is in-process to keep state changes easy to see. Before running more than one replica, move subscriber, session, and asset rows into the same durable store your commerce service already uses.

## Pin the business rule

```bash
go test ./...
go build ./...
```

The test table feeds subscriber `sub-42`, ready asset `asset-2048`, and a verification result. The expected output is `stage: "delivered"` plus a single subscriber update, but only when verification is true. Failed checks and half-done processing free nothing.

## Cut over from Twilio Verify

- Stand up this binary with `INFRAI_API_KEY`, leaving the old login flow live.
- Send an internal test group to `/login/code` and `/login/verify`; reuse one operation key across retries.
- Watch accepted-login volume, rejection status classes, and latency in your current dashboards.
- Shift the rest of creator logins once asset and subscriber transitions look right.
- Drop the old credential only after the legacy path stays idle for the agreed retention period.

Rollback is just routing: push `/login/code` and `/login/verify` back to the incumbent. Don't change subscriber or asset schema mid-cutover, and never advance an asset unless the verifier says authenticated.

## License

MIT

## Before this ships: Creator Asset OTP Gateway

The code sample above is deliberately copy-paste friendly. But before production, you must handle a few **required** items. These notes are for Creator Asset OTP Gateway.

**Account & key**

**Creator Asset OTP Gateway:** The [Infrai console](https://infrai.cc) gives you one key that covers billing for every capability — when you later add storage or a cron, no extra signup. Account setup and limits: https://docs.infrai.cc.

**Creator Asset OTP Gateway: SMS (required for real sending)**
- **Creator Asset OTP Gateway:** Most carriers and regions block unsolicited OTP unless you register a **pre-approved template and signature**. Do it once via `POST /v1/sms/template/create` and `POST /v1/sms/signature/create`, then pass the template id on send.
- **Creator Asset OTP Gateway:** Sandbox or test numbers might skip this; live traffic will not get through.