# jsonrpc-fsproxy

It passes requests to [JSON-RPC](https://www.jsonrpc.org/specification) server via files

## How it works

*jsonrpc-fsproxy* reads input file, passes each line to JSON-RPC server and writes response to output file.

##  Usage

```bash
jsonrpc-fsproxy [INPUT_FILE_PATH] [OUTPUT_FILE_PATH] [RPC_URL]
```

Argument  | Description 
------------- | -------------
INPUT_FILE_PATH | Path to input file
OUTPUT_FILE_PATH | Path to output file
RPC_URL | JSON-RPC server URL

### docker 

Image: [evsamsonov/jsonrpc-fsproxy](https://hub.docker.com/r/evsamsonov/jsonrpc-fsproxy)

```bash
$ docker pull evsamsonov/jsonrpc-fsproxy
$ docker run --rm -it -v ${PWD}/dev:/app/dev evsamsonov/jsonrpc-fsproxy /app/jsonrpc-fsproxy dev/rpcin dev/rpcout http://rpc_url
```

### go get

```bash
$ go get github.com/evsamsonov/jsonrpc-fsproxy
$ jsonrpc-fsproxy input_file_path output_file_path http://rpc-url
```

## Implementation of client

Language  | Link 
------------- | -------------
Lua | https://github.com/evsamsonov/quik-quotes-exporter/blob/master/src/jsonrpc_fsproxy_client.lua
