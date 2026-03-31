FROM golang:1.26.1-bookworm AS builder
ARG GITHUB_TOKEN
ARG GITHUB_ACTOR
WORKDIR /build
ENV GOPRIVATE=github.com/luxfi/*
ENV GONOSUMDB=github.com/luxfi/*
ENV GONOSUMCHECK=github.com/luxfi/*
RUN git config --global url."https://${GITHUB_ACTOR}:${GITHUB_TOKEN}@github.com/".insteadOf "https://github.com/"
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN rm -f go.sum && go mod tidy && CGO_ENABLED=0 go build -o zoo-cli .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /build/zoo-cli /usr/local/bin/zoo-cli
ENTRYPOINT ["zoo-cli"]
