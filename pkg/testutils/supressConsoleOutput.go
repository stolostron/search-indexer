package testutils

import (
	"fmt"
	"os"
)

// Supress console output to prevent log messages from polluting test output.
func SupressConsoleOutput() func() {
	fmt.Println("\t  !!!!! Test is supressing log output to stderr. !!!!!")

	nullFile, _ := os.Open(os.DevNull)
	stdErr := os.Stderr
	os.Stderr = nullFile

	return func() {
		defer nullFile.Close()
		os.Stderr = stdErr
	}
}
