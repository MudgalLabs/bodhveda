package bodhveda

import (
	"fmt"
	"strings"
)

type Error struct {
	// Message is something that can be shown to the API users on the UI.
	Message string `json:"message"`

	// Technical details regarding the error usually for API developers.
	Description string `json:"description"`

	// Indicates which part of the request triggered the error.
	PropertyPath string `json:"property_path,omitempty"`

	// Shows the value causing the error.
	InvalidValue any `json:"invalid_value,omitempty"`
}

// BodhvedaError matches your API error shape
type BodhvedaError struct {
	// A string indicating the outcome of the request.
	// Typically `success` for successful operations and
	// `error` represents a failure in the operation.
	Status string `json:"status"`

	// HTTP response status code.
	StatusCode int `json:"status_code"`

	// A message explaining what has happened.
	Message string `json:"message"`

	// A list of errors to explain what was wrong in the request body
	// usually when the input fails some validation.
	Errors []Error `json:"errors,omitempty"`
}

func (e *BodhvedaError) Error() string {
	if len(e.Errors) > 0 {
		// Collect all sub-errors
		var details []string
		for _, err := range e.Errors {
			// Show property and value if present
			if err.PropertyPath != "" {
				details = append(details,
					fmt.Sprintf("%s (%v): %s",
						err.PropertyPath,
						err.InvalidValue,
						err.Message,
					),
				)
			} else {
				details = append(details,
					fmt.Sprintf("%s: %s", err.Message, err.Description),
				)
			}
		}
		return fmt.Sprintf("HTTP %d %s: %s | Details: [%s]",
			e.StatusCode,
			e.Status,
			e.Message,
			strings.Join(details, "; "),
		)
	}

	// Simple error without validation details
	return fmt.Sprintf("HTTP %d %s: %s",
		e.StatusCode,
		e.Status,
		e.Message,
	)
}
