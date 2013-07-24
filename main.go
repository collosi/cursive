package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	fInputSeparator        = flag.String("is", ",", "input separator")
	fInputComment          = flag.String("ic", "", "input beginning of line comment character")
	fInputFieldsPerLine    = flag.Int("in", -1, "input expected number of fields per line (-1 is any)")
	fInputLazyQuotes       = flag.Bool("iq", false, "input allow 'lazy' quotes")
	fInputTrailingComma    = flag.Bool("il", true, "input allow trailing (last) comma")
	fInputTrimLeadingSpace = flag.Bool("it", false, "input trim leading space")

	fOutputSeparator = flag.String("os", ",", "output separator")
	fOutputCRLF      = flag.Bool("oc", false, "output using CRLF as line ending")

	fIgnoreHeader  = flag.Int("h", 0, "number of header lines to ignore")
	fIgnoreTrailer = flag.Int("t", 0, "number of trailer lines to ignore")
)

type replacement struct {
	field int
	res   string
	re    *regexp.Regexp
	with  string
}

var usage = func() {
	fmt.Fprintf(os.Stderr, "usage: %s [options] [[ <input> ] <output> ]\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "  -rN=<regexp>: regular expression to replace in field N\n")
	fmt.Fprintf(os.Stderr, "  -wN=<replacement>: replacement for field N, where $X denotes submatch\n")
	fmt.Fprintf(os.Stderr, DESCRIPTION)
	os.Exit(1)
}

func main() {
	replacements, err := preparseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s", err)
		usage()
	}
	flag.Usage = usage
	flag.Parse()

	input, output, err := getIO(flag.Args())
	reader := getReader(input)
	writer := getWriter(output)
	ignore := *fIgnoreHeader
	footerBuffer := make([][]string, *fIgnoreTrailer)
	footerBufferLocation := 0
	for err == nil {
		var record []string
		record, err = reader.Read()
		if err != nil {
			break
		}
		if ignore > 0 {
			ignore--
			continue
		}
		if len(replacements) > 0 {
			convertRecord(record, replacements)
		}
		if *fIgnoreTrailer > 0 {
			if footerBuffer[footerBufferLocation] != nil {
				err = writer.Write(footerBuffer[footerBufferLocation])
			}
			footerBuffer[footerBufferLocation] = record
			footerBufferLocation++
			footerBufferLocation = footerBufferLocation % (*fIgnoreTrailer)
		} else {
			err = writer.Write(record)
		}
	}
	writer.Flush()
	if err != io.EOF {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func getReader(r io.Reader) *csv.Reader {
	csvr := csv.NewReader(r)
	if len(*fInputSeparator) > 0 {
		csvr.Comma = rune((*fInputSeparator)[0])
	}
	if len(*fInputComment) > 0 {
		csvr.Comment = rune((*fInputComment)[0])
	}
	csvr.FieldsPerRecord = *fInputFieldsPerLine
	csvr.LazyQuotes = *fInputLazyQuotes
	csvr.TrailingComma = *fInputTrailingComma
	csvr.TrimLeadingSpace = *fInputTrimLeadingSpace
	return csvr
}

func getWriter(w io.Writer) *csv.Writer {
	csvw := csv.NewWriter(w)
	if len(*fOutputSeparator) > 0 {
		csvw.Comma = rune((*fOutputSeparator)[0])
	}
	if *fOutputCRLF {
		csvw.UseCRLF = *fOutputCRLF
	}
	return csvw
}

func preparseFlags() ([]replacement, error) {
	args := os.Args
	newArgs := make([]string, 0, len(args))
	replacements := make([]replacement, 0)
	for i, a := range args {
		if i == 0 {
			newArgs = append(newArgs, args[0])
			continue
		} else if strings.HasPrefix(a, "-r") {
			r, value, err := createOrFindReplacer(a[2:], &replacements)
			if err != nil {
				return nil, err
			}
			r.res = value
		} else if strings.HasPrefix(a, "-w") {
			r, value, err := createOrFindReplacer(a[2:], &replacements)
			if err != nil {
				return nil, err
			}
			r.with = value
		} else if !strings.HasPrefix(a, "-") {
			newArgs = append(newArgs, args[i:]...)
			break
		} else {
			newArgs = append(newArgs, a)
		}
	}
	os.Args = newArgs
	for i := range replacements {
		var err error
		replacements[i].re, err = regexp.Compile(replacements[i].res)
		if err != nil {
			return nil, err
		}
	}
	return replacements, nil
}

func createOrFindReplacer(flag string, replacements *[]replacement) (*replacement, string, error) {
	splits := strings.SplitN(flag, "=", 2)
	if len(splits) != 2 {
		return nil, "", fmt.Errorf("%s: invalid flag", flag)
	}
	value := splits[1]
	field, err := strconv.ParseUint(splits[0], 10, 32)
	if err != nil {
		return nil, "", err
	}
	for i, r := range *replacements {
		if r.field == int(field) {
			return &((*replacements)[i]), value, nil
		}
	}
	*replacements = append(*replacements, replacement{field: int(field)})
	return &((*replacements)[len(*replacements)-1]), value, nil
}

func convertRecord(record []string, replacements []replacement) {
	for _, r := range replacements {
		if len(record) > r.field {
			record[r.field] = r.re.ReplaceAllString(record[r.field], r.with)
		}
	}
}

func getIO(args []string) (input io.ReadCloser, output io.WriteCloser, err error) {
	input = os.Stdin
	output = os.Stdout
	switch len(args) {
	case 0:
		return
	case 1:
		if args[0] == "-" {
			return
		}
		// read from stdin, write to file
		output, err = os.Create(args[0])
	case 2:
		// read from stdin, write to file
		input, err = os.Open(args[0])
		if err != nil {
			return
		}
		output, err = os.Create(args[1])
	default:
		usage()
	}
	return
}

const DESCRIPTION = `
Cursive is a utility for reading and writing "separated value" formats like
CSV and TSV.

INPUT AND OUTPUT
If neither input nor output are specified on the command line, Cursive will
read from standard in and write to standard out.  If just an output file is
specified, Cursive will read from standard in and write to the file.  The
special value "-" may be used to specify output to standard out, in the case
that input from a file, and output to standard out is required.

REPLACEMENT

Cursive can do a "find-and-replace" operation on specific columns in the
input.  The special flags "-rN" and "-wN" can be used to match a regular
expression in the N'th column and replace it with an arbitrary expression.
The regular expression supports RE2 matching language with group capture.
Column numbers start at 0.

Group capture is designated in the regular expression with parenthesis,
and in the replacement expression with "$X", where X is a number starting
from 1.  The special replacement expression "$0" represents the entire
matched expression.

For example, to remove all single digits at the beginning of values in 
the first column you would.

  cursive -r0="^\d(.*)" -w0="$1" input.csv output.csv

The regular expression language supported by cursive is re2. Documentation
can be found here: https://code.google.com/p/re2/wiki/Syntax
`
