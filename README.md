# rosen tss project

This project is used for keygen, sign and regroup operations on eddsa and ecdsa protocols in threshold signature.
Base of the project is tss-lib of binance. For massage passing you should run ts-guard project and set it's port as p2pPort argument.

### build command 
```bash
make build
```

### config

just set home address and log level you need.

### run command
```bash
./roesnTss [options]
  -configFile string
        config file (default "./conf/conf.env")
  -p2pPort string
        p2p port (e.g. 8080) (default "8080")
  -port string
        project port (e.g. 4000) (default "4000")
  -publishPath string
        publish path of p2p (e.g. /p2p/send) (default "/p2p/send")
  -subscriptionPath string
        subscriptionPath for p2p (e.g. /p2p/channel/subscribe) (default "/p2p/channel/subscribe")
```

There are 6 endpoint.
1. keygen
   * used to generate key between peers 
   ```bash
     curl --location --request POST 'localhost:4000/keygen' \
     --header 'Content-Type: application/json' \
     --data-raw '{
     "peersCount" : 3,
     "threshold" : 2,
     "crypto": "eddsa"
     }'
     ```
2. sign
   * used to sign a message between peers 
   ```bash
     curl --location --request POST 'localhost:4001/sign' \
     --header 'Content-Type: application/json' \
     --data-raw '{
     "crypto" :"eddsa",
     "message" : "951103106cb7dce7eb3bb26c99939a8ab6311c171895c09f3a4691d36bfb0a70",
     "callBackUrl": "http://localhost:5051"
     }'
     ```
3. regroup
   * used to regroup and recreate and re-sharing key between new peers
   ```bash
     curl --location --request POST 'localhost:4000/regroup' \
     --header 'Content-Type: application/json' \
     --data-raw '{
     "peersCount" : 6,
     "newThreshold" : 2,
     "oldThreshold": 2,
     "crypto": "eddsa",
     "peerState": 0
     }'
     ```
4. message
   * used as an entry  to get messages from ts-guard.
   ```json
   {
     "message": "message",
     "channel": "channel",
     "sender": "sender"
   }
     ```
5. import
   * used to import private key of peer
   ```bash
   curl --location --request POST 'localhost:4000/import' \
    --header 'Content-Type: application/json' \
    --data-raw '{
    "private" : "c1a8a35de3d73936608ea9ab2070bbcb10c2361220943f0a5c30d7f04d81db4d9dd35bb9380eca988ce09afbc4158c7127a8cf82fcc63d126ca4322090dd0bf6",
    "crypto": "eddsa"
    }'
    ```
6. export
   * for exporting all peer have in the home folder


