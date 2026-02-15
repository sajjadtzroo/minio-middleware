FROM golang:1.23-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY .env.server .env

# Build the Go app
RUN go build -o main .

EXPOSE 3000

CMD ["./main"]