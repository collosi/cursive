package common

import (
	"strconv"
	"strings"
)

type FieldRange struct {
	Start int
	End   int
}

func ParseFieldRanges(ranges string) ([]*FieldRange, error) {
	if ranges == "" {
		return nil, nil
	}
	splits := strings.Split(ranges, ",")
	var frs []*FieldRange
	for _, r := range splits {
		fr, err := parseFieldRange(r)
		if err != nil {
			return nil, err
		}
		frs = append(frs, fr)
	}
	return frs, nil
}

func parseFieldRange(str string) (*FieldRange, error) {
	splits := strings.SplitN(str, "-", 2)
	i, err := strconv.ParseInt(splits[0], 10, 32)
	if err != nil {
		return nil, err
	}
	if len(splits) == 1 {
		return &FieldRange{Start: int(i) - 1, End: -1}, nil
	}
	i2, err := strconv.ParseInt(splits[1], 10, 32)
	if err != nil {
		return nil, err
	}
	return &FieldRange{Start: int(i) - 1, End: int(i2) - 1}, nil
}
