# Stage 1: Build the Go binary without CGO
FROM golang:1.26-alpine AS builder

# Set GOEXPERIMENT and disable CGO
ENV CGO_ENABLED=0
ENV GOEXPERIMENT=jsonv2

WORKDIR /app

# Install git for fallback cloning if building outside the root context
RUN apk add --no-cache git

# Copy everything from the build context.
COPY . .

# If the build context is cmd/bmx-server, then go.work will not exist in the root of the builder.
# We fallback to cloning the parent repository and copying the local context over it to preserve local modifications.
RUN if [ ! -f go.work ]; then \
      echo "Building from subdirectory context, cloning parent repository for workspace files..." && \
      git clone https://github.com/diamondburned/go-butterflymx.git /src && \
      cp -rf . /src/cmd/bmx-server && \
      rm -rf /app/* /app/.* 2>/dev/null || true && \
      mv /src/* /src/.* /app/ 2>/dev/null || true; \
    fi

# Build bmx-server statically
RUN go build -ldflags="-s -w" -o /app/bmx-server ./cmd/bmx-server

# Stage 2: Final Home Assistant Addon Image
FROM ghcr.io/home-assistant/base:latest

# Copy compiled binary from builder
COPY --from=builder /app/bmx-server /usr/bin/bmx-server

# Copy run script from builder
COPY --from=builder /app/cmd/bmx-server/run.sh /run.sh
RUN chmod a+x /run.sh

CMD [ "/run.sh" ]
