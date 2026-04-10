# blockness-monster
Simple blockchain in Go with http network

## How to run
Start a server
```bash
go run . 8080
```

Supports multiple servers to connect into a network on 8080, 8081, 8082
```bash
go run . 8082
```

Send a request to create a block with a string description
```bash
curl http://localhost:8080/mine -d "Hello, World!"
```

Get the current chain in json
```bash
curl http://localhost:8080/chain
```
