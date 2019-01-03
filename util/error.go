package util

import (
	"fmt"
	"os"
)

func ErrorCheck(err error) {
	fmt.Fprintf(os.Stderr, "ERROR %v\n", err)
}
