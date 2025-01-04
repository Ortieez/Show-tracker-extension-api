FROM golang:1.23.4

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o showtracker main.go

ENV PORT=8080
EXPOSE $PORT

VOLUME /app/cache

CMD ["./showtracker"]
