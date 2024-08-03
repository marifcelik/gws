test:
	go test ./...

bench:
	go test -benchmem -run=^$$ -bench . github.com/marifcelik/gws

cover:
	go test -coverprofile=./bin/cover.out --cover ./...
