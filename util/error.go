package util

import (
	"fmt"
	"os"
)

func ErrorCheck(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR %v\n", err)
	}
}
