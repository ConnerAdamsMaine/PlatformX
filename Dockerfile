FROM golang:1.23-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /meshgrid ./cmd/meshgrid
FROM alpine:3.21
COPY --from=build /meshgrid /usr/local/bin/meshgrid
ENTRYPOINT ["meshgrid"]

