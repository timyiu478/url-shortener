## Layered Architecture (Separation of Concerns)

The code is organized into distinct layers, each with a specific responsibility:

- Handlers: Handle HTTP requests and responses, parse input, and return output.
- Services: Contain the business logic for URL shortening and retrieval.
- Repositories: Manage data access to the database (Aurora MySQL) and cache (ElastiCache Redis).
- Config: Manage configuration loading from environment variables.

This separation ensures that each layer is independent, making the code easier to maintain, test, and extend. For example, you can swap out the database implementation in the repository layer without affecting the service or handler layers.

## Key Generation

- produce a 9-character key using Base62 (0-9, A-Z, a-z), providing $ 62^9 \approx 2^{53.6} $ (~13 quintillion) combinations, sufficient for billions of URLs with low collision probability.
- implement the retry-on-collision logic (up to 5 attempts) for duplicate key errors (MySQL error code 1062).
    - The retry-on-collision logic is concurrency-safe 
        - row-level locking and MVVC prevents read-write conflicts and write-write conflicts


## Scalability and Performance

- Cache-Aside: Redirects (GET) use ElastiCache for fast reads, handling the read-heavy workload and supporting 1000+ req/s. Aurora’s indexing (INDEX idx_short_key) ensures efficient lookups for cache misses.
- Stateless Service: The key generation logic keeps the service stateless because it does not rely on any stateful components.
