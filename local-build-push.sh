GOOS=linux GOARCH=amd64 go build -o manager
docker-buildx build --platform linux/amd64 -t wxt432/godzilla-operator:dev -f Dockerfile.local --push .
rm -rf manager