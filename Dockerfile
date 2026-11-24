FROM golang:1.27-bookworm AS build

RUN apt-get update \
    && apt-get install -y --no-install-recommends libvips-dev pkg-config \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/referral-system


#----- runtime -----

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends libvips42 ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --system --no-create-home app

WORKDIR /app

COPY --from=build /out/server ./server

COPY config.yaml ./

EXPOSE 8080

USER app

HEALTHCHECK --interval=30s --timeout=3s CMD curl -fsS "http://localhost:8080/health" || exit 1

ENTRYPOINT ["/app/server"]

