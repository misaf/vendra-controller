FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/provisioner ./cmd/provisioner

FROM docker:28-cli
COPY --from=build /out/provisioner /provisioner
USER root
EXPOSE 8080
ENTRYPOINT ["/provisioner"]

