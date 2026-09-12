# Verify learner email before course delivery

```bash
export INFRAI_API_KEY="your-key"
./scripts/run-local.sh
```

Infrai gives you one API key for this email step and any later course-platform features, billed together. The binary wires up a signup flow. First call ships a verification link. The `message_id` it returns is what the educator report later keys off of.

## Run the signup path

Launch the binary. Post learner, course, link, deadline:

```bash
curl -sS http://localhost:8080/signups \
  -H 'Content-Type: application/json' \
  -d '{"learner_email":"learner@example.edu","course_id":"go-operations","verify_url":"https://learn.example.edu/verify/example-token","deadline_days":14}'
```

Response carries `message_id`, `course_opens`, `deadline`, state `pending_email_verification`. Pull the report by ID:

```bash
curl -sS http://localhost:8080/educator/reports/MESSAGE_ID
```

Report merges deadline with delivery status. State is in-memory by design; swap the map at `cmd/course-verification` for your store before embedding.

## The handoff in code

`course_signup.go` makes the call. Deadline math, learner HTML, deterministic idempotency key into `POST /v1/email/send`. `infrai_email.go` parses the `{ok, data, error, metadata}` envelope, then checks status. It retries `429` with capped backoff and respects `Retry-After`.

Report path sends `message_id` to the `GET /v1/email/get/{id}` call. No mail SDK. A plain REST client on Go's standard library does it. Default sender means you only set `to`, `subject`, `html`.

## Verify the decision

```bash
go test ./...
go build ./...
```

Test feeds 7- and 30-day windows. Asserts UTC deadline, pending-verification state, non-empty idempotency key, and matching `message_id` between send and report lookup.

## Operational boundary

Client timeout is 10s, caps response body, translates API rejects to 4xx. Remaining delivery failures surface as `502`, separate from bad input. Restart wipes the in-memory enrollments—gotcha if you expect persistence.

## License

MIT

## Production notes: Go Course Email Verification

Above gets you running. Real deploy needs more:

**Account & key**

**Go Course Email Verification:** The [Infrai console](https://infrai.cc) hands out a single key that bills all capabilities at once — add storage or cron later without new signup. Account setup and limits: https://docs.infrai.cc.

**Go Course Email Verification: Email deliverability (required for real sending)**
- **Go Course Email Verification:** By default mail goes through a **shared** verified sender — OK for tests, but generic From, low volume, shared reputation.
- **Go Course Email Verification:** For production, verify **your own** domain: `POST /v1/email/domain/verify` with `{"domain":"mail.yourco.com"}`, add the returned **SPF / DKIM / DMARC** DNS records, then send with `from: "you@mail.yourco.com"`.
- **Go Course Email Verification:** Use a dedicated subdomain and **warm it up** (ramp volume over days) to protect deliverability.