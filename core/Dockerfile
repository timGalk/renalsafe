FROM golang:1.25.1-bookworm AS build
WORKDIR /src
COPY core/go.mod core/go.sum ./
RUN go mod download
COPY core/ ./
RUN CGO_ENABLED=0 go build -trimpath -o /renalsafe-server ./cmd/renalsafe-server
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends poppler-utils ca-certificates && rm -rf /var/lib/apt/lists/* && useradd -u 10001 -m renalsafe
WORKDIR /app
COPY --from=build /renalsafe-server /app/renalsafe-server
COPY data/ /app/data/
USER renalsafe
EXPOSE 8080
ENTRYPOINT ["/app/renalsafe-server","-listen","0.0.0.0:8080","-users","/run/renalsafe-users.json","-seed","/app/data/knowledge.json","-migrate"]
