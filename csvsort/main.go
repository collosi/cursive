package main

import (
	"flag"
	"fmt"
	"github.com/laslowh/cursive/common"
	"os"
	"strconv"
	"strings"
)

var (
	fInputSeparator        = flag.String("is", ",", "input separator")
	fInputTabSeparator     = flag.Bool("its", false, "input separator is the tab character (overrides -is)")
	fInputComment          = flag.String("ic", "", "input beginning of line comment character")
	fInputFieldsPerLine    = flag.Int("in", -1, "input expected number of fields per line (-1 is any)")
	fInputLazyQuotes       = flag.Bool("iq", false, "input allow 'lazy' quotes")
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
	fColumns         = flag.String("c", "", "a comma-separated list of column indices or ranges to be used for sort ordering; default is all columns")
	fReverse         = flag.Bool("r", false, "reverse sort order")
)

var usage = func() {
	fmt.Fprintf(os.Stderr, "usage: %s [options] [ <input> ]\n", os.Args[0])
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
	if len(fieldRanges) == 0 {
		fieldRanges = append(fieldRanges, &common.FieldRange{0, -1, 's'})
	}

	proc := common.CSVProcessor{
		InputSeparator:        *fInputSeparator,
		InputTabSeparator:     *fInputTabSeparator,
		InputComment:          *fInputComment,
		InputFieldsPerLine:    *fInputFieldsPerLine,
		InputLazyQuotes:       *fInputLazyQuotes,
		InputTrimLeadingSpace: *fInputTrimLeadingSpace,

		OutputFile:      *fOutputFile,
		OutputSeparator: *fOutputSeparator,
		OutputCRLF:      *fOutputCRLF,

		IgnoreBeginning: *fIgnoreBeginning,
		IgnoreEnd:       *fIgnoreEnd,
		NoHeader:        *fNoHeader,
		LineNumbers:     *fLineNumbers,
		ZeroBased:       *fZeroBased,
	}

	err = proc.OpenIO(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error opening file\n", err)
		os.Exit(1)
	}

	err = proc.Sort(createSortFunc(fieldRanges), *fReverse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func cmp(a, b string, flag byte) int {
	switch flag {
	case 'n':
		f1, err1 := strconv.ParseFloat(strings.TrimSpace(a), 64)
		f2, err2 := strconv.ParseFloat(strings.TrimSpace(b), 64)
		if err1 != nil && err2 != nil {
			goto strcmp
		}
		if err1 != nil {
			return -1
		}
		if err2 != nil {
			return 1
		}
		switch {
		case f1 < f2:
			return -1
		case f2 < f1:
			return 1
		default:
			return 0
		}
	default:
	}
strcmp:
	min := len(b)
	if len(a) < len(b) {
		min = len(a)
	}
	diff := 0
	for i := 0; i < min && diff == 0; i++ {
		diff = int(a[i]) - int(b[i])
	}
	if diff == 0 {
		diff = len(a) - len(b)
	}
	return diff
	return 0
}

func createSortFunc(ranges []*common.FieldRange) common.CSVCompareFunc {
	return func(r1 []string, r2 []string) bool {
		for _, r := range ranges {
			if r.End < 0 {
				c := cmp(r1[r.Start], r2[r.Start], r.Flag)
				switch {
				case c < 0:
					return true
				case c == 0:
					// continue
				case c > 0:
					return false
				}
			}
			for i := r.Start; i <= r.End; i++ {
				c := cmp(r1[i], r2[i], r.Flag)
				switch {
				case c < 0:
					return true
				case c == 0:
					// continue
				case c > 0:
					return false
				}
			}
		}
		return false
	}
}

const DESCRIPTION = `
csvsort - sort lines of CSV files by field

csvsort is part of the Cursive toolkit, and is analogous to the Unix 'sort'
command.  Cursive is a set of utilities for reading and writing "separated
value" formats like CSV and TSV.

Typical usage of csvsort would read from a CSV file, sort on a specific field,
and output the sorted lines.  For example,

  csvsort -c=3 input.csv

would sort the data in input.csv by field three.  Note that by default, csvsort
assumes that there is a header row that is passed through unaltered.

INPUT AND OUTPUT

If <input> is not specified on the command line, csvsort will read from
standard in.   If no "-o" flag is provided, csvsort will write to standard
out.

The "-c" flag allows the user to specify a subset of the input fields
for sorting, as a comma-separated list of field ranges.  Sort will be performed
in lexocographic order based on these output columns.

  cursive -c="4,5n"

will sort first by the fourth column, then by the fifth.  An "n" may be added
to a columns number to specify a numeric sort.

Field ranges can be either a single field number, or a start field and end 
field separated by a hypen.  For example, to sort by the first five
fields and the "tenth" field:

  cursive -c="1-4,10" input.csv

Field numbers start at 1.

`
