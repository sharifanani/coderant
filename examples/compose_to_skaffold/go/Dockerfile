FROM docker.io/golang:stretch
COPY ./src /app
WORKDIR /app/
CMD go run main.go -ip 0.0.0.0 -port 8080
