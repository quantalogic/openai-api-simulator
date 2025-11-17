## Multi-stage build: build the binary and copy to a small runtime image
FROM golang:1.22-alpine AS build
RUN apk add --no-cache git
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /workspace/server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /workspace/server /app/server
EXPOSE 3080
ENTRYPOINT ["/app/server"]
CMD ["-port", "3080"]
