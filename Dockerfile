# Single-stage Dockerfile using pre-built binary
# Build binary first: make build.linux
FROM alpine:3.20

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata wget

WORKDIR /app

# Platform argument for multi-arch support
ARG TARGETARCH=amd64

# Copy pre-built binary (built outside Docker with make build.linux)
COPY build/linux/${TARGETARCH}/analyzer .

# Create non-root user
RUN addgroup -g 1000 analyzer && \
    adduser -D -u 1000 -G analyzer analyzer && \
    chown -R analyzer:analyzer /app

USER analyzer

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/api/health || exit 1

# Run the binary
ENTRYPOINT ["./analyzer"]
CMD ["serve", "--addr", ":8080"]
