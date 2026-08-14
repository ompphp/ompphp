package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/ompphp/ompphp/internal/runtime"
)

func main() {
	entry := flag.String("entry", "gamemode.php", "PHP gamemode entry file")
	flag.Parse()
	r := runtime.New(context.Background(), nil, log.Default())
	defer r.Close()
	if err := r.Load(*entry); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}
