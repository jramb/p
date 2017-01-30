default: run

run:
	go run main.go 

install:
	go install

import:
	go run p.go import "$(CLOCKFILE)"

win:
	env CGO_ENABLED=1 GOOS=windows GOARCH=386 CC="i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp" go build -o p.exe

test:
	(cd tools ; go test)

p.exe:
	go build

linux:
	go build

linux-slim:
	go build --tags "libsqlite3 linux" -o p

server:
	go run main.go server

rpc:
	curl http://localhost:8080/rpc \
		-H "Content-Type: application/json" \
		-d '{"method": "P.Hello", "params": [{"who": "Jörg"}], "id": 1}' | jq

show:
	curl http://localhost:8080/rpc \
		-H "Content-Type: application/json" \
		-d '{"method": "P.Show", "params": [{"timeFrame": "week"}], "id": 1}' | jq

define CMDJS
	{"method": "P.Show",
		"params": [{"timeFrame": "week"}],
		"id": 1}
endef

jsonx:
	cat $(CMDJS) | curl http://localhost:8080/rpc \
		-H "Content-Type: application/json" \
		-d @-

#
#env CGO_ENABLED=1 GOOS=windows GOARCH=386 CC=i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp go install github.com/mattn/go-sqlite3
#

.PHONY: server

