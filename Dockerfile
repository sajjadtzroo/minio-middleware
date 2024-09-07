FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY .env.server .env
COPY . .

# Build the Go app
RUN go build -o main .

EXPOSE 3000

CMD ["./main"]