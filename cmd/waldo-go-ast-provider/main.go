package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/mirage-security/waldo/protocol"
	"github.com/mirage-security/waldo/providers/goast"
	"github.com/mirage-security/waldo/providers/providercmd"
)

func main() {
	if err := run(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, input io.Reader, output io.Writer) error {
	return providercmd.Run(input, output, func(request protocol.Request) ([]protocol.CodeFact, error) {
		return goast.Analyze(ctx, request.Root)
	})
}
