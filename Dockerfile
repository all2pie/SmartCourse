# syntax=docker/dockerfile:1
# One image recipe for every service: docker build --build-arg SERVICE=identity .

FROM golang:1.27-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
ARG SERVICE
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    test -n "$SERVICE" || (echo "build arg SERVICE is required" && exit 1); \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/${SERVICE}

# Static binary on distroless: no shell or package manager, runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
