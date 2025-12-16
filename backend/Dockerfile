FROM golang:1.24-alpine AS builder

# TÖRÖLD KI INNEN A GOARCH=amd64-et!
ENV CGO_ENABLED=0 \
    GOOS=linux
    # A GOARCH-ot a build parancs fogja megadni automatikusan

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# FONTOS: Itt olvassuk be, milyen architektúrára épít a Docker éppen
ARG TARGETARCH

# És átadjuk a Go fordítónak a $TARGETARCH változót
RUN CGO_ENABLED=0 GOOS=linux GOARCH=$TARGETARCH go build -o /app/main .

# ... (A többi rész változatlan maradhat, a FROM alpine stb.) ...
FROM alpine:3.21 AS final
RUN apk add --no-cache ca-certificates
WORKDIR /root/
COPY --from=builder /app/main .
COPY static ./static
COPY templates ./templates
EXPOSE 8080
CMD ["./main"]