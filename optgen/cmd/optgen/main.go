package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"

	"github.com/dolthub/go-mysql-server/optgen/cmd/support"
	"github.com/dolthub/go-mysql-server/sql/expression/function/aggregation"
)

var (
	errInvalidArgCount     = errors.New("invalid number of arguments")
	errUnrecognizedCommand = errors.New("unrecognized command")
)

var (
	pkg = flag.String("pkg", "aggregation", "package name used in generated files")
	out = flag.String("out", "", "output file name of generated code")
)

const useGoFmt = true

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) < 2 {
		flag.Usage()
		exit(errInvalidArgCount)
	}

	cmd := args[0]
	switch cmd {
	case "unaryAggs":
	case "naryAggs":
	case "frame":
	case "frameFactory":
	case "framer":

	default:
		flag.Usage()
		exit(errUnrecognizedCommand)
	}

	sources := flag.Args()[1:]
	readers := make([]io.Reader, len(sources))
	for i, name := range sources {
		file, err := os.Open(name)
		if err != nil {
			exit(err)
		}

		defer file.Close()
		readers[i] = file
	}

	var writer io.Writer
	if *out != "" {
		file, err := os.Create(*out)
		if err != nil {
			exit(err)
		}

		defer file.Close()
		writer = file
	} else {
		writer = os.Stderr
	}

	var err error
	switch cmd {
	case "unaryAggs":
		err = generateUnaryAggs(aggregation.UnaryAggDefs, writer)
	case "naryAggs":
		err = generateNaryAggs(aggregation.NaryAggDefs, writer)
	case "frame":
		err = generateFrames(nil, writer)
	case "frameFactory":
		err = generateFramesFactory(nil, writer)
	case "framer":
		err = generateFramers(nil, writer)
	}

	if err != nil {
		exit(err)
	}
}

// usage is a replacement usage function for the flags package.
func usage() {
	fmt.Fprintf(os.Stderr, "Optgen is a tool for generating optimizer code.\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n")

	fmt.Fprintf(os.Stderr, "\toptgen command [flags] sources...\n\n")

	fmt.Fprintf(os.Stderr, "The commands are:\n\n")
	fmt.Fprintf(os.Stderr, "\taggs generates aggregation definitions and functions\n")
	fmt.Fprintf(os.Stderr, "\n")

	fmt.Fprintf(os.Stderr, "Flags:\n")

	flag.PrintDefaults()

	fmt.Fprintf(os.Stderr, "\n")
}

func exit(err error) {
	fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
	os.Exit(2)
}

func generateUnaryAggs(defines support.GenDefs, w io.Writer) error {
	var gen support.AggGen
	return generate(defines, w, gen.Generate)
}

func generateNaryAggs(defines support.GenDefs, w io.Writer) error {
	var gen support.AggGen
	return generate(defines, w, gen.Generate)
}

func generateFrames(defines support.GenDefs, w io.Writer) error {
	var gen support.FrameGen
	return generate(defines, w, gen.Generate)
}

func generateFramesFactory(defines support.GenDefs, w io.Writer) error {
	var gen support.FrameFactoryGen
	return generate(defines, w, gen.Generate)
}

func generateFramers(defines support.GenDefs, w io.Writer) error {
	var gen support.FramerGen
	return generate(defines, w, gen.Generate)
}

func generate(defines support.GenDefs, w io.Writer, genFunc func(defines support.GenDefs, w io.Writer)) error {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by optgen; DO NOT EDIT.\n\n")
	fmt.Fprintf(&buf, "  package %s\n\n", *pkg)

	genFunc(defines, &buf)

	var b []byte
	var err error

	if useGoFmt {
		b, err = format.Source(buf.Bytes())
		if err != nil {
			// Write out incorrect source for easier debugging.
			b = buf.Bytes()
		}
	} else {
		b = buf.Bytes()
	}

	w.Write(b)
	return err
}
