FROM golang:1.25-slim

WORKDIR /nolejje

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o nolejje .

CMD ["./nolejje"]
