get the dependencies:

`go get code.google.com/p/go.crypto/ssh`  
`go get github.com/syndtr/goleveldb`

then `go build` and have fun

`whisper generate` to generate a key  
`whisper listen` to run the server  
`whisper --key $keyfile --nick $nick dial` to run the client  

In the client:

`notes/create $title` to create a note  
`notes/get $id` to get a note.  You have to look in the server log for the id right now but it's just an incrementing integer.  
