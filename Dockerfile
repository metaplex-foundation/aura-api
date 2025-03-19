# ===== Builder Stage =====
FROM quay.io/projectquay/golang:1.22 AS builder
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG BINARY
WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -a -v -installsuffix cgo -o ${BINARY} ./cmd/${BINARY}

# ===== Final Stage =====
FROM alpine:3.20
ARG BINARY
ENV BINARY=${BINARY}
WORKDIR /app

RUN apk add --no-cache ca-certificates && \
    addgroup -g 10001 appgroup && \
    adduser -D -G appgroup -u 10001 appuser
COPY --from=builder /app/${BINARY} /app/${BINARY}
COPY --from=builder /app/db /db
COPY --from=builder /app/creds /creds
ENV PATH="/app:${PATH}"
USER appuser
ENTRYPOINT ["sh", "-c", "exec ./${BINARY}"]