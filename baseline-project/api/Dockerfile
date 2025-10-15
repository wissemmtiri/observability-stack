# ---------- build stage ----------
FROM golang:1.24 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/api .

# ---------- run stage ----------
FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=build /out/api /app/api

USER 65532:65532

EXPOSE 8080
ENTRYPOINT ["/app/api"]