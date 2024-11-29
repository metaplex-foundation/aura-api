FROM quay.io/projectquay/golang:1.22 AS builder

WORKDIR /app

COPY . .

RUN make build

FROM alpine:3.20

WORKDIR /app
RUN apk add ca-certificates
COPY --from=builder /app/api /app
COPY --from=builder /app/db /db
COPY --from=builder /app/creds /creds
ENV PATH="/app:${PATH}"

ENTRYPOINT [ "./api" ]
