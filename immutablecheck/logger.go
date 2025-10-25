package immutablecheck

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func NewMicroDeltaIterator() func() int64 {
	prev := time.Now()
	first := true
	return func() int64 {
		if first {
			first = false
			// reset baseline to now so next call measures time after this point
			prev = time.Now()
			return 0
		}
		now := time.Now()
		delta := now.Sub(prev).Microseconds()
		prev = now
		return delta
	}
}

type logLevel int

const (
	warn = iota
	info
	errr
	dbug
)

// function to log things to a .log file for debugging
func putLog(level logLevel, s string) {
	f, err := os.OpenFile("debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("Error opening log file:", err)
		return
	}
	defer f.Close()

	var lvl string
	switch level {
	case warn:
		lvl = "[WARN]"
	case info:
		lvl = "[INFO]"
	case errr:
		lvl = "[ERROR]"
	case dbug:
		lvl = "[DBUG]"
	default:
		lvl = "[INFO]"
	}

	delta := microDeltaIter()
	line := fmt.Sprintf("+%8d us %s %s\n", delta, lvl, s)

	if _, err := f.WriteString(line); err != nil {
		fmt.Println("Error writing to log file:", err)
	}
}

// format is json like
func Pretty_print_immutables(immutables *map[string]immutableInfo) string {
	var sb strings.Builder
	sb.WriteString("Immutable Types Detected:\n{\n")
	for typeName, info := range *immutables {
		sb.WriteString(fmt.Sprintf("  \"%s\": { \"pos\": %d }\n", typeName, info.pos))
	}
	sb.WriteString("}\n")
	return sb.String()
}
