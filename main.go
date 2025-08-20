package main

import (
	"fmt"
	"os"
	"os/user"
	"squ1d++/repl"
)

func main() {
	user, err := user.Current()
	if err != nil {
		panic(err)
	}
	fmt.Printf("Hello %s! This is SQU1D++!\n",
		user.Username)
	repl.Start(os.Stdin, os.Stdout)
}
