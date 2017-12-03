.DEFAULT_GOAL=build
.PHONY: build test run

vet:
	go vet .

generate:
	go generate -x

get:
	go get -u github.com/golang/dep/...
	dep ensure

build: get vet generate
	go build .

clean:
	go clean .

test: build
	go test .

delete:
	go run main.go delete

explore:
	go run main.go --level info explore

provision:
	go run main.go provision --s3Bucket $(S3_BUCKET) --level info

provisionNOP:
	go run main.go provision --s3Bucket $(S3_BUCKET) --noop
