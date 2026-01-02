FROM golang:1.22-alpine

WORKDIR /app

COPY *.go ./

RUN go build -o slack-simulator .

EXPOSE 9001

CMD ["./slack-simulator"]
