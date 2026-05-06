build:
	# build tool to check for errors
	go build ./...
build-bin:
	# build binary
	go build -o ghclassroom
build-exe:
	# build executable for windows
	GOOS=windows GOARCH=amd64 go build -o ghclassroom.exe .
