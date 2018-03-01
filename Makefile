default: week

show:
	go run main.go show week

week:
	go run main.go week

run:
	go run main.go

install:
	go install

import:
	go run p.go import "$(CLOCKFILE)"

win32:
	env CGO_ENABLED=1 GOOS=windows GOARCH=386 CC="i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp" go build -o p32.exe

win:
	env CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC="x86_64-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp" go build -o p.exe

prep-sqlite3-32: # as su?
	env CGO_ENABLED=1 GOOS=windows GOARCH=386 CC="i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp" go install github.com/mattn/go-sqlite3

prep-sqlite3: # as su?
	env CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC="x86_64-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp" go install github.com/mattn/go-sqlite3

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

show-r:
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
#env CGO_ENABLED=1 GOOS=windows GOARCH=386 CC="i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp" go install github.com/mattn/go-sqlite3
#

.PHONY: server

