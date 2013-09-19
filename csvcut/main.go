package main

import (
	"flag"
	"fmt"
	"github.com/laslowh/cursive/common"
	"io"
	"math"
	"os"
)

var (
	fInputSeparator        = flag.String("is", ",", "input separator")
	fInputTabSeparator     = flag.Bool("its", false, "input separator is the tab character (overrides -is)")
	fInputComment          = flag.String("ic", "", "input beginning of line comment character")
	fInputFieldsPerLine    = flag.Int("in", -1, "input expected number of fields per line (-1 is any)")
	fInputLazyQuotes       = flag.Bool("iq", false, "input allow 'lazy' quotes")
	fInputTrailingComma    = flag.Bool("il", true, "input allow trailing (last) comma")
	fInputTrimLeadingSpace = flag.Bool("it", false, "input trim leading space")

	fOutputFile      = flag.String("o", "", "output file; defaults to stdout")
	fOutputSeparator = flag.String("os", "", "output separator; defaults to input separator")
	fOutputCRLF      = flag.Bool("oc", false, "output using CRLF as line ending")

	fIgnoreBeginning = flag.Int("bi", 0, "number of lines to ignore at beginning of file")
	fIgnoreEnd       = flag.Int("ei", 0, "number of lines to ignore at end of file")
	fNoHeader        = flag.Bool("h", false, "no header row, will create default headers")
	fLineNumbers     = flag.Bool("l", false, "insert a column of line numbers at the front of the output")
	fZeroBased       = flag.Bool("z", false, "when interpreting or displaying column numbers, use zero-based numbering")
	fNames           = flag.Bool("n", false, "display column names and indices from the input and exit")
	fColumns         = flag.String("c", "", "a comma-separated list of column indices or ranges to be extracted; default is all columns")
	fDeleteEmpty     = flag.Bool("d", false, "after cutting, delete rows which are completely empty")
)

var usage = func() {
	fmt.Fprintf(os.Stderr, "usage: %s [options] [[ <input> ] <output> ]\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, DESCRIPTION)
	os.Exit(1)
}

func main() {
	flag.Usage = usage
	flag.Parse()
	if *fInputTabSeparator {
		*fInputSeparator = "\t"
	}
	if *fOutputSeparator == "" {
		*fOutputSeparator = *fInputSeparator
	}
	fieldRanges, err := common.ParseFieldRanges(*fColumns)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error parsing columns\n", err)
		os.Exit(1)
	}

	procFunc := func(record []string, buffer []string, isHeader bool, lineNo int) ([]string, error) {
		return processRecord(fieldRanges, record, buffer, isHeader, lineNo)
	}
	if *fNames {
		if *fNoHeader {
			fmt.Fprintf(os.Stderr, "-n and -h are incompatible")
			os.Exit(1)
		}
		procFunc = func(record []string, buffer []string, isHeader bool, line int) ([]string, error) {
			printNames(os.Stdout, record)
			os.Exit(0)
			return nil, nil
		}
	}

	proc := common.CSVProcessor{
		ProcessFunc:           procFunc,
		InputSeparator:        *fInputSeparator,
		InputTabSeparator:     *fInputTabSeparator,
		InputComment:          *fInputComment,
		InputFieldsPerLine:    *fInputFieldsPerLine,
		InputLazyQuotes:       *fInputLazyQuotes,
		InputTrailingComma:    *fInputTrailingComma,
		InputTrimLeadingSpace: *fInputTrimLeadingSpace,

		OutputFile:      *fOutputFile,
		OutputSeparator: *fOutputSeparator,
		OutputCRLF:      *fOutputCRLF,

		IgnoreBeginning: *fIgnoreBeginning,
		IgnoreEnd:       *fIgnoreEnd,
		NoHeader:        *fNoHeader,
		LineNumbers:     *fLineNumbers,
		ZeroBased:       *fZeroBased,
		Columns:         *fColumns,
		DeleteEmpty:     *fDeleteEmpty,
	}

	err = proc.OpenIO(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error opening file\n", err)
		os.Exit(1)
	}

	err = proc.Process()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func processRecord(fieldRanges []*common.FieldRange, record []string, buffer []string, isHeader bool, line int) ([]string, error) {
	if fieldRanges == nil {
		return append(buffer, record...), nil
	}

	for _, r := range fieldRanges {
		if r.Start >= len(record) {
			return nil, fmt.Errorf("%d: no such field in record of length %d", r.Start, len(record))
		}
		if r.End < 0 {
			buffer = append(buffer, record[r.Start])
		} else {
			for i := r.Start; i <= r.End; i++ {
				buffer = append(buffer, record[i])
			}
		}
	}
	return buffer, nil
}

func printNames(output io.Writer, ns []string) {
	n := len(ns)
	nchars := int(math.Ceil(math.Log10(float64(n+1)))) + 1
	format := fmt.Sprintf("%% %dd: %%s\n", nchars)
	for i, n := range ns {
		fmt.Fprintf(output, format, i+1, n)
	}
}

const DESCRIPTION = `
Filter and truncate CSV files. Like unix "cut" command, but for tabular data.

INPUT AND OUTPUT

If neither input nor output are specified on the command line, Cursive will
read from standard in and write to standard out.  If just an output file is
specified, Cursive will read from standard in and write to the file.

The "-c" flag allows the user to specify a subset of the input fields
for output, as a comma-separated list of field ranges.  Field ranges can
be either a single field number, or a start field and end field separated by
a hypen.  For example, to output the first five
fields and the "tenth" field:

  cursive -c="1-4,10" input.csv

Field numbers start at 1.

`
