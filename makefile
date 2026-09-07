#!make

build.linux:
	@GOOS=linux GOARCH=amd64 go build -o gocpc

build.windows:
	@GOOS=windows GOARCH=amd64 go build -o gocpc.exe

build.mac:
	@GOOS=darwin GOARCH=arm64 go build -o gocpc