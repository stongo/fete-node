# Fete-Node
A peer-to-peer network for implementing Threshold Signature Scheme (TSS), for ECDSA and EDDSA.

(Eventually) Implements the [tss-lib](https://github.com/bnb-chain/tss-lib/) to generate keys, sign messages, and regenerate keys.

## Local Testing for Development

1. `make all`
2. Enabled logging by executing this in shell `export IPFS_LOGGING=debug`
3. Setup multiple nodes, with different repos and ports
```
go run ./... -port=4001 -repo="$HOME/.fetenode-0"
go run ./... -port=4002 -repo="$HOME/.fetenode-1"
go run ./... -port=4003 -repo="$HOME/.fetenode-2"
```
4. Note the peer address of each peer, and write them to `$HOME/.peers.cfg`, one per line
5. Restart all nodes, adding a flag for peer discovery
```
go run ./... -port=4001 -repo="$HOME/.fetenode-0" -peer-list="$HOME/.peers.cfg"
```
6. Check JSON RPC endpoint
```
curl -v localhost:4001/rpc
curl -H "Content-Type: application/json" -X POST --data '{"jsonrpc":"2.0","method":"message_sign","params":["sign this"],"id":67}' localhost:5000/rpc
```

## Production Deployment

1. A Docker image is available on [Dockerhub](https://hub.docker.com/r/stongo/fete-node)
2. Install [asdf](https://asdf-vm.com/guide/getting-started.html) 
3. `cd deploy/terraform`
4. `asdf install` will install the neceesary version of terraform
5. Run `terraform plan` and if it succeeds, run `terraform apply` 
