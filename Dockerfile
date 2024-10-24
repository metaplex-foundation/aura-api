FROM golang:1.22-alpine3.19 as builder

WORKDIR /app

COPY . .

RUN --mount=type=ssh CGO_ENABLED=1 GOOS=linux go build -a -v -installsuffix cgo --tags "sqlite_foreign_keys" ./cmd/api

FROM alpine:3.19
RUN apk add ca-certificates
#FIX of alpine can't find binary file
RUN apk add --no-cache libc6-compat
COPY --from=builder /app/api /usr/bin/

COPY --from=builder /app/db /db
COPY --from=builder /app/creds /creds
