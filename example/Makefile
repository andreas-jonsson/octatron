export GOPATH=$(CURDIR)

default: main.go
	go build -v -o example

run: default
	go run -v main.go

race: main.go
	go run -v -race main.go

dep-install:
	go get -v github.com/andreas-t-jonsson/octatron

clean:
	rm -rf src/github.com example
