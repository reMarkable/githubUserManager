# syntax=docker/dockerfile:1
FROM golang:1.27-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOFLAGS=-trimpath go build -ldflags="-s -w" -o /githubusermanager .

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /githubusermanager /githubusermanager
ENTRYPOINT ["/githubusermanager"]
