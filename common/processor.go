package common

import (
	"bufio"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
)

type RecordFunc func(record []string, buffer []string, isHeader bool, lineNo int) ([]string, error)

type CSVProcessor struct {
	InputSeparator        string
	InputTabSeparator     bool
	InputComment          string
	InputFieldsPerLine    int
	InputLazyQuotes       bool
	InputTrimLeadingSpace bool

	OutputFile      string
	OutputSeparator string
	OutputCRLF      bool

	IgnoreBeginning int
	IgnoreEnd       int
	NoHeader        bool
	LineNumbers     bool
	ZeroBased       bool

	input  io.Reader
	output io.Writer
}

func (proc *CSVProcessor) OpenIO(args []string) error {
	var err error
	proc.input = os.Stdin
	proc.output = os.Stdout
	switch len(args) {
	case 0:
	case 1:
		proc.input, err = os.Open(args[0])
	default:
		return errors.New("too many arguments")
	}
	if err != nil {
		return err
	}
	if proc.OutputFile != "" {
		proc.output, err = os.Open(proc.OutputFile)
	}

	ignore := proc.IgnoreBeginning
	if ignore > 0 {
		buffered := bufio.NewReader(proc.input)
		for ignore > 0 {
			_, isPrefix, err := buffered.ReadLine()
			if err != nil {
				return err
			}
			if isPrefix {
				errors.New("line too long\n")
				os.Exit(1)
			}
			ignore--
		}
		proc.input = buffered
	}

	return err
}

type CSVCompareFunc func(r1 []string, r2 []string) bool

type sortableCSV struct {
	less   CSVCompareFunc
	values [][]string
}

func (scsv *sortableCSV) Len() int {
	return len(scsv.values)
}

func (scsv *sortableCSV) Less(i, j int) bool {
	return scsv.less(scsv.values[i], scsv.values[j])
}

func (scsv *sortableCSV) Swap(i, j int) {
	scsv.values[i], scsv.values[j] = scsv.values[j], scsv.values[i]
}

func (proc *CSVProcessor) Sort(f CSVCompareFunc, reverse bool) error {
	reader := proc.getReader()
	writer := proc.getWriter()

	c, err := reader.ReadAll()
	if err != nil {
		return err
	}
	if proc.IgnoreEnd > 0 {
		if len(c) < proc.IgnoreEnd {
			return errors.New("entire file was ignored because of value of 'ignore end'")
		}
		c = c[:len(c)-proc.IgnoreEnd]
	}
	sortRef := c
	if !proc.NoHeader {
		sortRef = sortRef[1:]
	}
	var sortInterface sort.Interface = &sortableCSV{f, sortRef}
	if reverse {
		sortInterface = sort.Reverse(sortInterface)
	}
	sort.Sort(sortInterface)
	if proc.NoHeader && len(c) > 0 {
		header := make([]string, 0, len(c[0])+1)
		if proc.LineNumbers {
			header = append(header, "N")
		}
		header = append(header, createHeaderRecord(len(c[0]))...)
		err = writer.Write(header)
		if err != nil {
			return err
		}
	}
	if proc.LineNumbers {
		offset := 1
		if proc.ZeroBased {
			offset = 0
		}
		for i, line := range c {
			expanded := make([]string, 0, len(line)+1)
			expanded = append(expanded, strconv.Itoa(i+offset))
			expanded = append(expanded, line...)
			err = writer.Write(expanded)
			if err != nil {
				return err
			}
		}
	} else {
		return writer.WriteAll(c)
	}
	return nil
}

func (proc *CSVProcessor) Process(processFunc RecordFunc, deleteEmpty bool) error {
	var err error

	reader := proc.getReader()
	writer := proc.getWriter()

	footerBuffer := make([][]string, proc.IgnoreEnd)
	footerBufferLocation := 0
	line := 1
	if proc.ZeroBased {
		line = 0
	}
	if !proc.NoHeader {
		line -= 1
	}
	isFirst := true
	for err == nil {
		var record []string
		var outputRecord []string
		record, err = reader.Read()
		if err != nil {
			break
		}
		first := "N"
		if isFirst && proc.NoHeader {
			buffer := make([]string, 0, len(record)+1)
			if proc.LineNumbers {
				buffer = append(buffer, first)
			}
			outputRecord, err = processFunc(createHeaderRecord(len(record)), buffer, true, line)
			if err != nil {
				break
			}
			if outputRecord != nil {
				err = writer.Write(outputRecord)
			}
			if err != nil {
				break
			}
			isFirst = false
		}

		buffer := make([]string, 0, len(record)+1)
		if proc.LineNumbers {
			if !isFirst {
				first = strconv.Itoa(line)
			}
			buffer = append(buffer, first)
		}
		outputRecord, err = processFunc(record, buffer, (!proc.NoHeader) && isFirst, line)
		if err != nil {
			break
		}

		if proc.IgnoreEnd > 0 {
			if footerBuffer[footerBufferLocation] != nil {
				record := footerBuffer[footerBufferLocation]
				if record != nil {
					err = writer.Write(record)
				}
			}
			if !deleteEmpty || !isEmptyRecord(outputRecord, proc.LineNumbers) {
				footerBuffer[footerBufferLocation] = outputRecord
				footerBufferLocation++
				footerBufferLocation = footerBufferLocation % (proc.IgnoreEnd)
			}
		} else {
			if !deleteEmpty || !isEmptyRecord(outputRecord, proc.LineNumbers) {
				if outputRecord != nil {
					err = writer.Write(outputRecord)
				}
			}
		}
		if err != nil {
			break
		}
		isFirst = false
		line++
	}
	writer.Flush()
	if err == io.EOF {
		return nil
	}
	return err
}

func (proc *CSVProcessor) getReader() *csv.Reader {
	csvr := csv.NewReader(proc.input)
	if len(proc.InputSeparator) > 0 {
		csvr.Comma = rune((proc.InputSeparator)[0])
	}
	if len(proc.InputComment) > 0 {
		csvr.Comment = rune((proc.InputComment)[0])
	}
	csvr.FieldsPerRecord = proc.InputFieldsPerLine
	csvr.LazyQuotes = proc.InputLazyQuotes
	csvr.TrimLeadingSpace = proc.InputTrimLeadingSpace
	return csvr
}

func (proc *CSVProcessor) getWriter() *csv.Writer {
	csvw := csv.NewWriter(proc.output)
	if len(proc.OutputSeparator) > 0 {
		csvw.Comma = rune((proc.OutputSeparator)[0])
	}
	if proc.OutputCRLF {
		csvw.UseCRLF = proc.OutputCRLF
	}
	return csvw
}

func createHeaderRecord(sz int) (header []string) {
	header = make([]string, 0, sz)
	for i := 0; i < sz; i++ {
		header = append(header, fmt.Sprintf("C%d", i+1))
	}
	return
}

func isEmptyRecord(r []string, ignoreFirst bool) bool {
	for i, c := range r {
		if ignoreFirst && i == 0 {
			continue
		}
		if c != "" && c != `""` {
			return false
		}
	}
	return true
}
