// Copyright Contributors to the Open Cluster Management project
package clustersync

// NOTE:
// This test helper file is duplicated  in other packages because we haven't found
// a good way to share across packages.

import (
	"fmt"
	"os"
)

// Supress console output to prevent log messages from polluting test output.
func supressConsoleOutput() func() {
	fmt.Println("\t  !!! Test is supressing log output to stderr. !!!")

	nullFile, _ := os.Open(os.DevNull)
	stdErr := os.Stderr
	os.Stderr = nullFile

	return func() {
		defer func(nullFile *os.File) {
			_ = nullFile.Close()
		}(nullFile)
		os.Stderr = stdErr
	}
}
