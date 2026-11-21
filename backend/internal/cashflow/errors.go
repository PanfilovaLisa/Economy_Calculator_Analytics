package cashflow

import "fmt"

type DealError struct {
    DealID int
    Err    error
}

func (e *DealError) Error() string {
    return fmt.Sprintf("deal %d: %v", e.DealID, e.Err)
}

func (e *DealError) Unwrap() error { return e.Err }