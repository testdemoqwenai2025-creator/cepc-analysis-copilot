# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/server ./cmd/server/

# Runtime stage
FROM python:3.12-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    libfontconfig1 \
    && rm -rf /var/lib/apt/lists/*

# Install Python physics dependencies
COPY python/requirements.txt /app/python/
RUN pip install --no-cache-dir -r /app/python/requirements.txt

# Install CJK fonts for plotting
RUN mkdir -p /usr/share/fonts/truetype/chinese
COPY --from=builder /usr/share/fonts/ /usr/share/fonts/

WORKDIR /app

# Copy Go binary
COPY --from=builder /bin/server /app/server

# Copy Python physics layer
COPY python/ /app/python/

# Copy web templates
COPY web/ /app/web/

# Create data directories
RUN mkdir -p /app/data /app/cache /app/cache/plots

EXPOSE 8080

CMD ["/app/server"]
