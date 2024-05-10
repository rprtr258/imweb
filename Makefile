run:
	go run .

watch:
	reflex --start-service -r '\.go' -- go run .
