// Package testutil contains the common test utilities.
package testutil

import (
	"fmt"
	"math"
	"reflect"
)

func CheckEqual(a, b interface{}) error {
	if !reflect.DeepEqual(a, b) {
		return fmt.Errorf("%+v != %+v", a, b)
	}
	return nil
}

// checkPctDiff checks to see whether a is within p*100% of b, returning an
// error if not.
func CheckPctDiff(a, b, p float64) error {
	d := math.Abs(a-b) / b
	if d > p {
		return fmt.Errorf("PctDiff between %v and %v is %v > %v", a, b, d, p)
	}
	return nil
}
