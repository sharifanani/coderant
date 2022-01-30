FROM docker.io/golang:stretch
RUN apt-get update && apt-get install -y supervisor
COPY ./src /app
WORKDIR /app/
RUN go build -o ./example_server main.go
COPY supervisord.conf /etc/supervisor/conf.d/example_backend.conf
CMD supervisord -n -c /etc/supervisor/supervisord.conf