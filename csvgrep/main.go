package main

import (
	"flag"
	"fmt"
	"github.com/laslowh/cursive/common"
	"os"
	"regexp"
	"strconv"
	"strings"
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

	fFilterMode   = flag.Bool("f", true, "filter non matching rows")
	fInvertFilter = flag.Bool("v", false, "invert filter (filter matching rows)")
)

type replacement struct {
	field     int
	res       string
	re        *regexp.Regexp
	isReplace bool
	with      string
}

var usage = func() {
	fmt.Fprintf(os.Stderr, "usage: %s [options] [[ <input> ] <output> ]\n", os.Args[0])
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "  -rN=<regexp>: regular expression to match in field N\n")
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

	if *fInputTabSeparator {
		*fInputSeparator = "\t"
	}
	if *fOutputSeparator == "" {
		*fOutputSeparator = *fInputSeparator
	}

	procFunc := func(record []string, buffer []string, isHeader bool, lineNo int) ([]string, error) {
		return processRecord(replacements, record, buffer, isHeader, lineNo, *fFilterMode, *fInvertFilter)
	}

	proc := common.CSVProcessor{
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
	}

	err = proc.OpenIO(flag.Args())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error opening file\n", err)
		os.Exit(1)
	}

	err = proc.Process(procFunc, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func processRecord(replacements []replacement, record []string, buffer []string, isheader bool, lineNo int, filterMode, invert bool) ([]string, error) {
	buflen := len(buffer)
	buffer = append(buffer, record...)
	record = buffer[buflen:]
	if replacements == nil || len(replacements) == 0 || isheader {
		return buffer, nil
	}

	for _, r := range replacements {
		if r.field < 0 || r.field >= len(record) {
			return nil, fmt.Errorf("%d: no such field in record of length %d", r.field, len(record))
		}
		if filterMode && r.re.MatchString(record[r.field]) == invert {
			return nil, nil
		}
		if r.isReplace {
			record[r.field] = r.re.ReplaceAllString(record[r.field], r.with)
		}
	}
	return buffer, nil
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
			r.isReplace = true
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
	field -= 1
	for i, r := range *replacements {
		if r.field == int(field) {
			return &((*replacements)[i]), value, nil
		}
	}
	*replacements = append(*replacements, replacement{field: int(field)})
	return &((*replacements)[len(*replacements)-1]), value, nil
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

The "-of" flag allows the user to specify a subset of the input fields
for output, as a comma-separated list of field ranges.  Field ranges can
be either a single field number, or a start field and end field separated by
a hypen.  For example, to output the first five
fields and the "tenth" field:

  cursive -of="0-4,10" input.csv output.csv

Field number start at 0, and so the field labelled "10" in the example
above is actually the 11th field.

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
the first column you would use

  cursive -r0="^\d(.*)" -w0="$1" input.csv output.csv

The regular expression language supported by cursive is re2. Documentation
can be found here: https://code.google.com/p/re2/wiki/Syntax
`
