module blah

go 1.19

replace github.com/SundaeSwap-finance/ogmigo/store/badgerstore => ../../store/badgerstore

replace github.com/SundaeSwap-finance/ogmigo => ../..

require github.com/SundaeSwap-finance/ogmigo v0.0.0-00010101000000-000000000000

require (
	github.com/aws/aws-sdk-go v1.44.197 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/sync v0.1.0 // indirect
)
