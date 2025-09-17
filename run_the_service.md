# How to run the service

Here we briefly describe how to run the service

1. run dynamodb instance
2. create the dynamodb table

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

3. build the go project or docker image

4. configure the environment variables

```
ENV PORT=8080
ENV AWS_REGION=us-east-1
ENV AWS_END_POINT=http://localhost:8000
ENV DYNAMODB_TABLE=Urls
ENV SHARD_ID=0
```

5. configure the aws credentials

6. run the service
