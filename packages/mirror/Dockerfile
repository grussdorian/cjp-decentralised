# Build the CJP mirror daemon. Multi-stage to keep the final image tiny.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /mirror .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && adduser -D -u 1000 mirror
USER mirror
COPY --from=build /mirror /usr/local/bin/cjp-mirror
ENTRYPOINT ["/usr/local/bin/cjp-mirror"]
