package query

import (
	"github.com/mudgallabs/tantra/apires"
	"github.com/mudgallabs/tantra/service"
)

type Cursor struct {
	After  *string `query:"after" schema:"after" json:"after"`
	Before *string `query:"before" schema:"before" json:"before"`
	Limit  *int    `query:"limit" schema:"limit" json:"limit,omitempty"`
}

func (c *Cursor) Validate(maxLimit, defaultLimit int) error {
	if c.Limit == nil || (c.Limit != nil && *c.Limit <= 0) {
		c.Limit = &defaultLimit
	}

	if *c.Limit > maxLimit {
		c.Limit = &maxLimit
	}

	var errs service.InputValidationErrors

	if c.After != nil && c.Before != nil {
		errs.Add(apires.NewApiError("Invalid cursor", "Cannot have both 'after' and 'before' set", "cursor", c))
	}

	if len(errs) > 0 {
		return errs
	}

	return nil
}
