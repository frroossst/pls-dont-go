package immutablecheck

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

type logLevel int

const (
	warn = iota
	info
	errr
	dbug
)

type logLocation int

const (
	stderr = iota
	filelog
	nowhere
)

var (
	logDestination    string
	logLoc            logLocation
	logFile           *os.File
	logMutex          sync.Mutex
	logDestinationSet bool
)

func SetLogDestination(dest string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	logDestination = dest
	logDestinationSet = true

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}

	switch dest {
	case "":
		// no destination specified, suppress logs
		logLoc = nowhere
	case "stderr":
		logLoc = stderr
	default:
		// It's a file path
		logLoc = filelog
	}
}

func putLog(level logLevel, s string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	if !logDestinationSet {
		logLoc = nowhere
	}

	where := logLoc

	if where == nowhere {
		return
	}

	var lvl string
	switch level {
	case warn:
		lvl = "[WARN]  "
	case info:
		lvl = "[INFO]  "
	case errr:
		lvl = "[ERROR] "
	case dbug:
		lvl = "[DBUG]  "
	default:
		lvl = "[INFO]  "
	}

	line := fmt.Sprintf("%s %s\n", lvl, s)

	switch where {
	case stderr:
		fmt.Fprint(os.Stderr, line)
	case filelog:
		if logFile == nil {
			var err error
			logFile, err = os.OpenFile(logDestination, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error opening log file %s: %v\n", logDestination, err)
				return
			}
		}

		if _, err := logFile.WriteString(line); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing to log file: %v\n", err)
		}
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
