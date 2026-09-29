package domain

import "errors"

var ErrOrderNotFound = errors.New("order not found")
var ErrOrderCannotBeCancelled = errors.New("order cannot be cancelled in its current state")
