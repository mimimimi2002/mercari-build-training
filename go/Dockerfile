FROM golang:1.24-alpine

WORKDIR /app

RUN apk add --no-cache git gcc musl-dev
COPY go.mod go.sum ./

RUN go mod download

COPY . .

ENV CGO_ENABLED=1

# compile
RUN go build cmd/api/main.go

RUN addgroup -S mercari && adduser -S trainee -G mercari
# RUN chown -R trainee:mercari /path/to/db

RUN chown -R trainee:mercari db
RUN chown -R trainee:mercari images

USER trainee

CMD ["./main"]
