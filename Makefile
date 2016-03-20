default: linux

install:
	go install

import:
	go run p.go import "$(CLOCKFILE)"

win:
	env CGO_ENABLED=1 GOOS=windows GOARCH=386 CC="i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp" go build -o p.exe


linux:
	go build

linux-slim:
	go build --tags "libsqlite3 linux" -o p


#
#env CGO_ENABLED=1 GOOS=windows GOARCH=386 CC=i686-w64-mingw32-gcc -fno-stack-protector -D_FORTIFY_SOURCE=0 -lssp go install github.com/mattn/go-sqlite3
#
