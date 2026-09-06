### Layered Architecture (Separation of Concerns)

The codebase is organized into distinct layers, each with a specific responsibility:
- **Handlers:** Handle HTTP requests and responses, parse input, and format output.
- **Services:** Contain the core business logic for URL shortening and retrieval.
- **Repositories:** Manage data access to the database (Amazon DynamoDB). The repository abstraction isolates data operations, making it easy to swap out backend storage implementations without affecting the service or handler layers.
- **Config:** Manage configuration loading from environment variables.

This separation ensures that each layer is independent, making the codebase easier to maintain, test, and extend.

### Key Generation

The service produces a short key matching the 9-character regex pattern `/[a-zA-Z0-9]{9}/`. It utilizes base62 encoding (`0-9`, `A-Z`, `a-z`), where 1 character is reserved for regional sharding and 8 characters are randomly generated. This massive key space easily supports trillions of URLs with an extremely low collision probability.

**Pros:**
- Simple and keeps the service completely stateless.
- Supports parallel writes per region without requiring cross-region coordination or locking.

**Cons:**
- Slight reduction of the total randomized key space due to the dedicated shard prefix character.
- Collision rate relies on cryptographic pseudo-random distribution rather than pre-generated sequential tracking.

**Collision Handling:**
Implements a retry-on-collision logic (up to 3 attempts) for duplicate key errors. The retry logic is concurrency-safe within a region and uses a conditional `PutItem` operation with the `ConditionExpression: attribute_not_exists(short_key)` to ensure absolute uniqueness before committing writes.

### Health Checks

Added health check endpoints `/healthz` and `/readyz` for determining the liveness and readiness of the instance by external orchestrators (such as Kubernetes or AWS Application Load Balancers).
