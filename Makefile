.PHONY: test

run:
	docker-compose pull
	docker-compose up -d --build

test:
	docker-compose pull
	docker-compose up -d db
	go test ./... -p 1

build:
	env GOOS=linux CGO_ENABLED=0 go build -ldflags="-s -w" -o coshh cmd/main.go
	mkdir -p bin/
	zip bin/coshh.zip coshh
	rm coshh
