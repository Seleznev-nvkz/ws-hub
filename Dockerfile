ARG GO_VERSION=1.12

# stage 1
FROM golang:${GO_VERSION}-alpine AS ws-hub-builder
RUN apk add --no-cache ca-certificates git

WORKDIR /src
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ ./

RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags="-s -w" -installsuffix cgo -o /app .


# stage 2
FROM scratch as ws-hub
WORKDIR /root/
COPY --from=ws-hub-builder /app /app

EXPOSE 8080 6379
ENTRYPOINT ["/app"]