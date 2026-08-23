.PHONY: generate format test race build up down demo benchmark k8s-apply

generate:
	cd proto && protoc --go_out=. --go_opt=module=github.com/5g-core/proto --go-grpc_out=. --go-grpc_opt=module=github.com/5g-core/proto amf.proto smf.proto upf.proto

format:
	gofmt -w amf/*.go smf/*.go upf/*.go ue/*.go

test:
	go test ./amf/... ./smf/... ./upf/... ./ue/... ./proto/...

race:
	go test -race ./amf/... ./smf/... ./upf/... ./ue/...

build:
	go build ./amf ./smf ./upf ./ue

up:
	docker compose up -d --build

down:
	docker compose down

demo:
	docker compose --profile tools run --build --rm ue

benchmark:
	mkdir -p results
	go run ./ue -action benchmark -count 500 -concurrency 50 -scenario mixed -output results/benchmark.csv

k8s-apply:
	kubectl apply -f k8s/
