package main

import (
	"errors"

	"github.com/seeruk/tego/internal/protogenx"
	"google.golang.org/protobuf/compiler/protogen"
)

// TODO: Probably shouldn't be all in main. Let's move it.
func main() {
	options := protogen.Options{}

	options.Run(func(plugin *protogen.Plugin) error {
		rawParams := plugin.Request.GetParameter()
		if protogenx.HasParameterValue(rawParams, "paths", "source_relative") {
			// Using source_relative would generate invalid results, as we're going to generated
			// types with the same name as the types `proto-gen-go` generates.
			// TODO: Could be allowed if types were generated with a prefix or suffix in this case?
			return errors.New("tego does not support 'paths=source_relative'")
		}

		for _, file := range plugin.Files {
			if !file.Generate {
				continue
			}

			// TODO: Generation code...
		}
		return nil
	})
}
