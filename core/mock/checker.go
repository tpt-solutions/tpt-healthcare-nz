package mock

import (
	"context"

	"github.com/PhillipC05/tpt-healthcare/core/ddi"
)

// DDIChecker is a configurable fake implementing ddi.Checker for unit tests.
type DDIChecker struct {
	Interactions []ddi.Interaction
	Err          error
}

func (c *DDIChecker) Check(_ context.Context, _ ddi.CheckRequest) ([]ddi.Interaction, error) {
	return c.Interactions, c.Err
}
