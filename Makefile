GOOS:=linux
GOARCH:=amd64
MODULE:=github.com/rorski/grpc-job-manager
USER:=ec2-user
INSTANCE:=ec2-1-2-3-4.us-west-2.compute.amazonaws.com
SSH_KEY:=/path/to/.ssh/sshkey

.PHONY: all clean protobufs server client certs deploy test

clean:
	rm -f ./bin/*
	rm -f ./certs/*
	rm -f build.tar

protobufs:
	protoc --go_out=. --go_opt=paths=import --go_opt=module=${MODULE} --go-grpc_out=. --go-grpc_opt=paths=import --go-grpc_opt=module=${MODULE} proto/job.proto

server:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -o ./bin/server ./cmd/server/

client:
	GOOS=${GOOS} GOARCH=${GOARCH} go build -o ./bin/client ./cmd/client/

certs:
	go run ./tools/make_certs.go
	mv ./*.pem certs/
	mv ./*.key certs/

deploy:
	tar -cvf build.tar ./certs ./bin
	scp -i ${SSH_KEY} build.tar ${USER}@${INSTANCE}:

test:
	sudo go test -race -v -timeout 30s ./worker ./internal/api

all: protobufs server client certs
