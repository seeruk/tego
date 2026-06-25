package main

import (
	"github.com/seeruk/tego/internal/tego"
	"google.golang.org/protobuf/compiler/protogen"
)

func main() {
	options := protogen.Options{}
	options.Run(tego.RunPlugin)
}
