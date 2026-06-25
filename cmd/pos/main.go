package main

import (
	"fmt"

	"github.com/seeruk/tego/proto"
)

func main() {
	builder := proto.OneOfMany_builder{
		StringValue: new("fuck off"),
	}

	oom := builder.Build()

	fmt.Println(oom.WhichValue())
}
