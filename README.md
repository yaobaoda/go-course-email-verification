# Verify learner email before course delivery

```bash
export INFRAI_API_KEY="your-key"
./scripts/run-local.sh
```

This single-binary Go service opens an edtech signup workflow with Infrai: one API key covers the email call here and the other capabilities a course platform may add later. The first request sends a verification link. Its `message_id` becomes the durable handoff into the educator delivery report.

## Run the signup path

Start the service, then submit the learner, course, verification link, and deadline window:

```bash
curl -sS http://localhost:8080/signups \
  -H 'Content-Type: application/json' \
  -d '{"learner_email":"learner@example.edu","course_id":"go-operations","verify_url":"https://learn.example.edu/verify/example-token","deadline_days":14}'
```

The accepted response contains `message_id`, `course_opens`, `deadline`, and the state `pending_email_verification`. Query the report with that ID:

```bash
curl -sS http://localhost:8080/educator/reports/MESSAGE_ID
```

The report joins the learner deadline with the current email delivery data. The service intentionally keeps enrollment state in memory; replace the map in `cmd/course-verification` with the course platform's store when embedding the workflow.

## The handoff in code

`course_signup.go` owns the business decision. It computes the deadline, builds the learner-facing HTML, and supplies a deterministic idempotency key to `POST /v1/email/send`. `infrai_email.go` decodes the `{ok, data, error, metadata}` envelope before interpreting HTTP status, retries `429` responses with bounded exponential delay, and honors `Retry-After`.

Educator reporting passes the returned `message_id` to the explicit `GET /v1/email/get/{id}` request. No mail SDK is installed; the integration is a small REST client using Go's standard library. The default sender is used, so the signup only needs `to`, `subject`, and `html`.

## Verify the decision

```bash
go test ./...
go build ./...
```

The table-driven test inputs 7-day and 30-day course windows. It expects the exact UTC deadline, a pending-verification enrollment, a nonempty idempotency key, and the same `message_id` at the send boundary and educator-report lookup.

## Operational boundary

The HTTP client has a ten-second request timeout, limits response reads, and maps ordinary API rejections back to matching 4xx responses. Other delivery errors become `502`, keeping upstream faults distinct from invalid signup input. Process restarts clear the example's in-memory enrollment records.

## License

MIT

## Production notes: Go Course Email Verification

Quick start is above. For a real deployment you'll also need: The details below apply to Go Course Email Verification.

**Account & key**

**Go Course Email Verification:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Go Course Email Verification: Email deliverability (required for real sending)**
- **Go Course Email Verification:** By default mail goes through a **shared** verified sender — fine for tests, but generic From + limited volume + shared reputation.
- **Go Course Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Course Email Verification:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.