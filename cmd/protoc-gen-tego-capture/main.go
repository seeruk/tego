package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/seeruk/tego/internal/protogenx"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	if err := run(); err != nil {
		writeErrorResponse(err)
	}
}

func run() error {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("read code generator request: %w", err)
	}

	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(input, &req); err != nil {
		return fmt.Errorf("decode code generator request: %w", err)
	}

	outputPath, err := outputPathFromParams(req.GetParameter())
	if err != nil {
		return err
	}

	if dir := filepath.Dir(outputPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory %q: %w", dir, err)
		}
	}

	if err := os.WriteFile(outputPath, input, 0o644); err != nil {
		return fmt.Errorf("write captured request %q: %w", outputPath, err)
	}

	fmt.Fprintf(os.Stderr, "captured code generator request to %s\n", outputPath)
	return writeResponse(successResponse())
}

func outputPathFromParams(params string) (string, error) {
	value, ok := protogenx.ParameterValue(params, "path")
	if !ok {
		return "", fmt.Errorf("missing required path option")
	}
	if value == "" {
		return "", fmt.Errorf("path option must not be empty")
	}
	return value, nil
}

func writeErrorResponse(err error) {
	fmt.Fprintln(os.Stderr, err)
	if writeErr := writeResponse(&pluginpb.CodeGeneratorResponse{Error: new(err.Error())}); writeErr != nil {
		fmt.Fprintln(os.Stderr, writeErr)
		os.Exit(1)
	}
}

func writeResponse(resp *pluginpb.CodeGeneratorResponse) error {
	output, err := proto.Marshal(resp)
	if err != nil {
		return fmt.Errorf("encode code generator response: %w", err)
	}
	if _, err := os.Stdout.Write(output); err != nil {
		return fmt.Errorf("write code generator response: %w", err)
	}
	return nil
}

func successResponse() *pluginpb.CodeGeneratorResponse {
	return &pluginpb.CodeGeneratorResponse{
		SupportedFeatures: new(uint64(pluginpb.CodeGeneratorResponse_FEATURE_SUPPORTS_EDITIONS)),
		MinimumEdition:    new(int32(descriptorpb.Edition_EDITION_2023)),
		MaximumEdition:    new(int32(descriptorpb.Edition_EDITION_2024)),
	}
}
