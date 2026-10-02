// Defines errors that other parts of the application can recognize, such as
// a missing order or a rejected cancellation. The HTTP handler uses these
// errors to choose the response status.

package domain

import "errors"

var ErrOrderNotFound = errors.New("order not found")
var ErrOrderCannotBeCancelled = errors.New("order cannot be cancelled in its current state")
var ErrInvalidOrder = errors.New("order is invalid")
var ErrInvalidUUID = errors.New("UUID is invalid")