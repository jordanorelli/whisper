get the dependencies:

`go get code.google.com/p/go.crypto/ssh`  
`go get github.com/syndtr/goleveldb`

then `go build` and have fun

`whisper generate` to generate a key  
`whisper listen` to run the server  
`whisper --key $keyfile --nick $nick dial` to run the client  

In the client:

`notes/create $title` to create a note  
`notes/list` to list the notes you have created  
`notes/get $id` to get a note by id  

`msg/send $recipient` send a message to `$recipient`  
`msg/list` list messages that you have received  
`msg/get $id` to fetch and decrypt a message by id  
