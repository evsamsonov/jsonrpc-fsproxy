docker build --tag jsonrpc-fsproxy .
docker run --rm -e RPC_URL=http://localhost:8080/rpc -v $PWD/dev:/app/dev jsonrpc-fsproxy 
