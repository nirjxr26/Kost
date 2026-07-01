FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /kost ./cmd/kost/

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=build /kost /kost
USER 65534
ENTRYPOINT ["/kost"]
