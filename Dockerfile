FROM golang:1.26.4-bookworm AS builder
WORKDIR /build
COPY go.mod ./
RUN go mod download || true
COPY . .
ENV CGO_ENABLED=0
RUN rm -f go.sum && go mod tidy && go build -o zoo-cli .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /build/zoo-cli /usr/local/bin/zoo-cli
ENTRYPOINT ["zoo-cli"]
