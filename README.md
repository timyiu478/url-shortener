# A URL shortener service

A URL shortener service is a short link generator, consisting of assigning a unique key of few characters to a specific web page with the ability to redirect to the original URL. This document describes the a simple URL shortener service from requirenets gathering, system architecture design, tech stack selection, database schema design, CI/CD considerations, and possible future works. To know how to run 
the service, please refer to [run_the_service.md](run_the_service.md).

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

This architecture is designed with a focus on how the system scale globally for handling high volume of traffic and tolerating zone/region level failures that can be fully implemented with AWS services and defined as code.

![](assets/deployment_architecture.png)

- Key Technoliges: 
    - Edge Location for reducing propagation delay
    - Geo-Distributed Key-Value Database for fast read/write operations and durability
    - DNS for seperating traffic per region and service discovery
    - Application Load Balancer for load balancing across url-shorten service
    - Web Application Firewall: rate limiting, inspect and filter suspicious HTTP requests
- Key Assumption: 
    - the system is read-heavy based daily life experience. The volume of `GET /{shortenUrl}` requests is much larger than the volume of `POST /newurl` requests.
- The term *region level* denotes the service is highly available and can tolerate at least one availability zone failure
- Even though we say the database is *region level*, no data loss with high probability under entire data center failure
- Main Cache Hierarchy: Edge Location -> Region Level Cache
    - Edge location: absorb most of the requests of the popular URLs
    - Region Level Cache: serves uncached or cache-miss requests for all url-shorten service instances
    - the URL shortening service instances do not maintain individual caches, as this would necessitate complex load balancing techniques to prevent cache hot spots and ensure high cache hit rates across all instances.
- The internal traffic flow is also controlled for security e.g. the ALB only call the url-shorten service (i.e. ALB can't call the database).
- CI/CD consideration: we choose to use **container** as the portable uint of the url-shorten service because
    - it is lightweight which take up less space and are easier to scale
    - immutability: it packaged all service dependences which enable us to deploy and test it consistently from the test environment to the production environment
    - widely adopted solution with large active community
- The url-shorten service is stateless. We can deploy it in anywhere and at anytime.
- We favour availability over consistency when under a network partition.
- All connections between services are secured e.g. all are HTTPS connections.
- Full CQRS pattern(seperate read store and write store) is not applied in this design for simplicity
- Why Key-Value DB?
    - The data model for serving URL shortening is simple that can be represented by a single key-value pair
    - No need for complex queries such as scanning a table and joinning multiple tables
    - key-value DB's read and write performance is much better SQL DB by key sharding technique
    - If we use SQL DB, we may need to add an distributed cache layer for optimize the read performance.
- Seperate url-shorten service instances to multiple shards (at least 1 shard per region) for parallel writes
    - see [implementations_details.md](implementations_details.md) for the details
- Deploying the url-shorten services on the second is optional
- AWS services that we can use: Cloudfront, ALB, DynamoDB, Route53, ElastiCache, WAF, VPC, EKS
    - we will discussed them a bit more in the *Tech Stack* section
- The other necessary systems such as CI/CD system, intrusion detection/prevention systems, and observability system are omitted as intend

### Scaling Mechanism

![scaling_mechanism](assets/scaling_mechanism.png)

The mechanism can be realized in many established solutions that can support different scaling strategy without modifying the core logic of url-shorten service source code. Also, it is applicable for other services that require auto-scaling. If we use kubernetes to orchestrate the containers, we can use the [kubernetes horizontal pod autoscaler](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/) for implementing this mechanism. 

---

## Tech Stack

- EKS:
    - can leverage kubernetes features such as auto-scaling and automated rollout and rollback
    - container can be automatically distributed across multiple EC2 nodes
        - avoid a single node network bandwidth become our service botteneck
    - assume we need strong control on the operating system e.g. we need OS security hardening
    - control plane is managed by AWS which can reduce the maintenance overhead
- VPC:
    - necessary for deploying compute or storage resources on AWS
    - isolate our workload to other AWS users
- CloudFront:
    - more than 400 edge locations across over 90 cities and 48 countries
    - can integrate with WAF seamlessly
- Route53:
    - publish our edge location and load balancer domain names
    - manage internal domain names
    - Mult-AZ Resolver Endpoint SLA: 100% refund when the uptime is less than 99.50% 
- DynamoDB:
    - Multi-AZ replication by default, with Global Tables for multi-region active-active writes, ensuring no single point of failure and region-level durability (RPO=0).
    - adjusts for capacity by automatically scaling tables with zero administration
    - serve more than 10 trillion requests per day with 20 million requests per second at peaks (millisecond latency), over petabytes of storage
    - using the in-memory caching technique called DynamoDB Accelerator (DAX), the read performance of DynamoDB tables can be improved by up to 10 times, even at millions of requests per second (microsecond latency)
 
The above numbers can give us a strong sense that we can build the highly available URL shorten systen that can meet our scaling target 1000+ req/sec with these cloud services.

---

## Dynamodb Table Schema

- table name: `Urls`
- attibute definitions:
    - `short_key` as string 
- hash partition key: `short_key`

Here is the example of how to create a dynamdb table into the local dynamdb instance using aws cli:

```bash
❯ aws dynamodb create-table \
--table-name Urls \
--attribute-definitions AttributeName=short_key,AttributeType=S \
--key-schema AttributeName=short_key,KeyType=HASH \
--region us-east-1 \
--endpoint-url http://localhost:8000 \
--provisioned-throughput ReadCapacityUnits=5,WriteCapacityUnits=5
```

---

## CI/CD Design/Considerations

### CI Pipeline

1. triggerd by pass the code peer review and be able merge the code change into main branch of the git repository
    1. assume we use trank based development git branch strategy
1. code linting
1. static code analysis for identifying vulnerability and unused dependencies
1. run all unit tests
1. run fuzz tests for discovering unknown bugs
1. build container image
1. run integration tests for testing integration with external dependences such as cache and database
1. sign the image to ensure the image is built from trusted source or pipeline
1. push the image to container registry
1. trigger CD pipeline (if the CD pipeline is push-based)

### CD Pipeline per environment and per region

- The CD pipeline is a process of building and testing the deployment artifact for allowing a particular system can be running on a particular environment. For example, if the system running environment is kubernetes worker nodes, the pipeline may need to build a list of kuberentes objects such as Deployment, ConfigMap, Secret, and Namespace.
- We promote the artifact from lower environments such as DEV and STG to production environment.
- In general, the pipelines should be mirrored except the difference related to the environment specific configurations for consistency.
- For each environment, we can use **canary** deployment strategy to deploy the artifact to production environment to reduce the risk of widespread outstage of all users.
- The CD pipeline/system should provide rollback mechanism. For example, if we use ArgoCD and we want to rollback the container image from v1.0.1 to v1.0.0, we just need to simply change the container image version v1.0.0 in the artifact.

---

## Possible Future Works

- make sure the shortenUrl key generation collision ratio is lower than our expectation
- support metrics endpoint and distributed tracing for observability and alert system integration
    - Key Metrics for the web server: 
        - API Performance: request latency, request rate, error rate
        - Resource Utilization: CPU, memory, number of active HTTP connections
        - Key Generation: number of collisions, number of retries
- support multiple domain names
- write REST API specification as code for openness and easier to manage API lifecycle
    - e.g. https://swagger.io/specification/
- support data analysis about the REST API usage
