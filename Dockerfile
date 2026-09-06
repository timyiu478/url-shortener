FROM golang:1.26 as build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app ./cmd/api

FROM gcr.io/distroless/static:nonroot
COPY --from=build /app /app
EXPOSE 8080
USER nonroot
ENTRYPOINT ["/app"]
