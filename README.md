# A URL shortener service

A URL shortener service is a short link generator, consisting of assigning a unique key of few characters to a specific web page with the ability to redirect to the original URL. This document descri[...]

> This service is not production ready.

## Requirements

### Functional Requirements

We need to provide a simple REST API that supports two endpoints:

#### 1. URL submission

Request:

```json
POST /newurl
{
    "domain": "shortenurl.org",
    "url": "https://www.google.com"
}
```

Response Payload:

```json
{
    "url": "https://www.google.com",
    "shortenUrl": "https://shortenurl.org/g20hi3k9Z"
}
```

#### 2. Shorten redirect URL

Request:

```
GET /{shortenUrl}
shortenUrl - regex /[a-zA-Z0-9]{9}/
```

Response:

```
GET /g20hi3k9Z
HTTP 304 to saved link (e.g. https://www.google.com according to the previous example)
```

Note about redirect behavior

- This project deliberately returns HTTP 304 (Not Modified) for the GET /{shortenUrl} endpoint in the current implementation. This is an intentional choice for the exercise: the service signals the client that the resource should be handled externally.
- Many clients (including curl without flags and some browsers) will not automatically follow a 304 as a redirect. If you want to follow redirects when testing from the command line, use a client that follows redirects or the `-L` flag with `curl`:

```bash
# follow redirects
curl -L http://localhost:8080/<shortKey>

# or inspect headers to see Location (if you handle redirect differently)
curl -I http://localhost:8080/<shortKey>
```

If you prefer standard redirect semantics you can change the handler to return 302/307 — see internal/handlers/handlers.go.

#### REST API Assumptions

- anyone can call both two endpoints without authentication and authorization
- the `domain` field of the `POST /newurl` request payload is used to allow user to control the domain name of the shorten url
    - we assume that `domain` value always is valid that can be resolved to the our owned IP(s)
- the `shortenUrl` needs to be unique across different domains
    - i.e. `https://shortenurl.org/g20hi3k9Z` and `https://anotherdomain.com/g20hi3k9Z` can not co-exist
    - reason: the web server may not know the domain name of the shorten url when handling the `GET /{shortenUrl}` request
- the `POST /newurl` can process the same payload
    - orginal url to shorten url is 1 to many relationship
- the `GET /{shortenUrul}` always return the HTTP response with status code 304
    - we don't need to support [ETag](https://en.wikipedia.org/wiki/HTTP_ETag)
      mechanism

### Non-Functional Requirements

- High Availability: highly available and no single point of failure.
- Scalability: 
    - scaling target: 1000+ req/s, after scaling-up/out without major code change.

---

## System Architecture Design

### Deployment Architecture

This architecture is designed with a focus on how the system scale globally for handling high volume of traffic and tolerating zone/region level failures that can be fully implemented with AWS services.

![](assets/deployment_architecture.png)

... (rest of doc unchanged)

---

## CI / Local development checks

The repository includes a lightweight GitHub Actions CI workflow (see `.github/workflows/ci.yml`) that runs on push and pull-request. The CI performs the following checks:

- `gofmt` validation (ensures code is formatted)
- `go vet` static checks
- `go test ./...` runs unit tests
- `go build ./...` verifies the project compiles

This CI is intentionally focused on fast feedback for development branches. The local development flow mirrors CI: run `gofmt`, `go vet`, `go test`, and build before opening a PR.

Local quick commands

```bash
# format check
if [ -n "$(gofmt -l .)" ]; then echo "Run gofmt -w ."; exit 1; fi

# static analysis
go vet ./...

# unit tests
go test ./... -v

# build
go build ./...
```

If you run the service locally with Docker Compose the Prometheus instance scrapes the `/metrics` endpoint exposed by the API at `api:8080`; open Prometheus at http://localhost:9090 to explore metrics.
