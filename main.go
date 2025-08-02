package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/mini-ninja-64/flotilla/cmd/root"
)

func main() {
	if err := fang.Execute(context.Background(), root.Cmd()); err != nil {
		os.Exit(1)
	}
}
