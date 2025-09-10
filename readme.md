# go-tari-c29-solo-stratum

A basic solo stratum compatible with current C29 miners that is designed to allow fee-free solo mining to the Tari blockchain using the Cuckatoo29 algo

## Installation from source
* This assumes you already have a modern version (Tested with 1.24.4) of Go available 
```bash
git clone https://github.com/Snipa22/go-tari-c29-solo-stratum.git
cd go-tari-c29-solo-stratum
go build .
```
This has only been tested on linux at this time, but there's no reason it shouldn't work on windows just as well.

# Usage
Running the executable with --help will give standard help options, in general, the minimum is suggested:  
```bash
go-tari-c29-solo-stratum -node-grpc-address <tari node with mining GRPC enabled> -pool-wallet-address <tari address>
```
The application uses the hatchling/jagtech node pool by default, but it's much lower latency to use your own local node.