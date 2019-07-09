package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func getPokerId(user User) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s.poker.%d", s.BotToken, user.Id)))
	secret := hex.EncodeToString(sum[:])
	return s.ServiceId + ":" + secret[:14]
}

// func deposit () {
//    POST https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/invoices' -d '{"fields": {"amount": {"stringValue": "3"}, "accountId": {"stringValue": "VU6sH6jZnltQYg3L4hQC"}, "
// state": {"stringValue": "requested"}}}' -H 'Content-Type: application/json
//   -> { "name": "projects/ln-pkr/databases/(default)/documents/invoices/8cXo0FNxkMrskS2vOAAv" }
//
//   GET https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/invoices/8cXo0FNxkMrskS2vOAAv
//   -> {
//     "fields": {
//  "payment_request": {
//       "stringValue": "lnbc30n1pwj8kc4pp5ahp4qr42dvcpsnjaxql806xg2g2sa39x4ykzmggh26d3gp5dlz0sdpvdp68gurn8ghj7mrfva58gmnfdenj6ur0ddjhytnrdakscqzpgxqyp2xqgs76ewjr0w8w60flltkqkyk2qx8hf3w2s55e2dneuj7kgs7spgsqycsj8vwwq28y5t2zpyc63c7xwmxeccqrdf4a2c0wr3dzguq275sqe3yt48"
//     }
//     }
//   }
// }
//
// func balance () {
//   curl 'https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/accounts/VU6sH6jZnltQYg3L4hQC'
//   {
//     "name": "projects/ln-pkr/databases/(default)/documents/accounts/VU6sH6jZnltQYg3L4hQC",
//     "fields": {
//       "balance": {
//         "integerValue": "0"
//       }
//     },
//   }
// }
//
// func withdraw () {
//   curl -X POST 'https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/payments' -d '{"fields": {"payment_request": {"stringValue": "lnbc1u1pwj8he7pp5p0m08uzxyra7ruf98wycrucqyexqu73cdhe2qxacfyxk8nd2jdtsdqgv9nkz6twxqyz5vqcqp2rzjqwgtt4zf9hp02vvw2ge6kt8t7m2gj9ygrge7765ud0xmkse6mxrdqzxjkyqqw4sqqqqqqqqqqqqqqqqqrcuwha4jx2qgz2nfn2m3g7js8ty5vth0zlkykutduswz6jq3nt99fp4mcy2gqhpqmw3ggvf8nudxut0q34p8n96tf3evr84q05rfygpwsqtehere"}, "accountId": {"stringValue": "hiQbmFqKIQ5vhRr9RFNC"}, "state": {"stringValue": "requested"}}}' -H 'Content-Type: application/json'
// }
//
// func tables () {
//     curl 'https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/tables' | jq '.documents | map({name, playing: .fields.playing.integerValue})'
// }
//
// func online () {
//   https://firestore.googleapis.com/v1/projects/ln-pkr/databases/(default)/documents/players
//   {
//     documents: [
//       {chips: {integerValue: "200"}}
//     ]
//   }
// }
