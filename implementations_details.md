## Layered Architecture (Separation of Concerns)

The code is organized into distinct layers, each with a specific responsibility:

- Handlers: Handle HTTP requests and responses, parse input, and return output.
- Services: Contain the business logic for URL shortening and retrieval.
- Repositories: Manage data access to the database (Aurora MySQL) and cache (ElastiCache Redis).
- Config: Manage configuration loading from environment variables.

This separation ensures that each layer is independent, making the code easier to maintain, test, and extend. For example, you can swap out the database implementation in the repository layer without affecting the service or handler layers.

## Key Generation

- produce a short key by randomly generating a 8 numbers with base62 (0-9, A-Z, a-z) encoding, providing $62^8$, sufficient for millions of URLs with low collision probability.
    - the 1 character is reserved for sharding per region
    - pros: 
        - simple
        - keeps the service stateless
        - supports parallel writes per region without worrying about collisions across regions
    - cons:
        - reduce the key space by 1 character
        - no test result provided for collision rate
        - no real world usage data to validate the approach
- implement the retry-on-collision logic (up to 3 attempts) for duplicate key errors.
    - The retry-on-collision logic is **concurrency-safe** within region.
       - uses a conditional `PutItem` with `ConditionExpression: attribute_not_exists(short_key)` to ensure the key doesn’t exist before writing 
          - https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/V2globaltables_HowItWorks.html#V2globaltables_HowItWorks.consistency-modes

