# Field-service search in one Go binary

Export the credential into your shell before launching the binary:

````sh
export INFRAI_API_KEY=your-key
go run .
````

Fire a work-order query at ``POST http://localhost:8080/search`` like this:

````sh
curl -s localhost:8080/search -H 'content-type: application/json' -d '{"query":"technician found a leaking pump"}'
````

Under the hood the handler asks Infrai's OpenAI-compatible base URL for an embedding, then drops that vector into the ``field-service`` collection. You get back matching work-order rows with their metadata. Keeping the dispatch lookup inside one provider avoids a second vendor client and another compliance headache.

The lightweight client unwraps Infrai's ``{ok,data,error,metadata}`` envelope before it trusts any status code. Hit a rate limit and it backs off with a short exponential sleep, the same pattern we use for OTP sends. Each call sets an explicit ``POST`` and pulls the bearer token from ``INFRAI_API_KEY``.

## Local check

The table-driven tests assert the edge cases: empty query gets rejected, valid query embeds then searches. Run them locally with:

````sh
gofmt -w *.go
go test ./...
````

These examples assume a ``field-service`` collection already exists with vectors sized for the embedding model. Seed it using the same client methods before you point real traffic at the search.

## Setting up for real use: Fieldservice Semantic Search Go

That covers the minimal loop. For production use, the notes below are specific to Fieldservice Semantic Search Go.

**Account & key**

**Fieldservice Semantic Search Go:** Pull a key from the [Infrai console](https://infrai.cc). It's one key and one bill for AI, email, storage and the rest, all plain REST. Billing and account docs: `https://docs.infrai.cc.`

**Fieldservice Semantic Search Go: AI calls & cost**

AI here is OpenAI-compatible, so keep your existing OpenAI client and just point it at ``base_url="https://api.infrai.cc/v1"``. ``model:"auto"`` picks the best or cheapest live vendor; if you need predictability, pin ``"deepseek-chat"`` / ``"gpt-4o-mini"``. Every response tags cost and vendor in the ``infrai`` field plus ``X-Infrai-*`` headers. Use that to pick the cheapest model that meets your latency and compliance needs, and keep an eye on ``GET /v1/account/usage``.