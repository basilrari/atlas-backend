package response

import (
	"github.com/gofiber/fiber/v2"
)

// SuccessBody is the standardized success JSON shape.
type SuccessBody struct {
	Status   string      `json:"status"`
	Message  string      `json:"message"`
	Data     interface{} `json:"data"`
	Metadata interface{} `json:"metadata,omitempty"`
}

// ErrorBody is the standardized error JSON shape.
type ErrorBody struct {
	Status string       `json:"status"`
	Error  ErrorDetail  `json:"error"`
}

// ErrorDetail is the nested error object.
type ErrorDetail struct {
	Message    string      `json:"message"`
	StatusCode int         `json:"statusCode"`
	Details    interface{} `json:"details,omitempty"`
}

const statusSuccess = "success"
const statusError = "error"

// Success sends a 200 OK response with the standard success format.
func Success(c *fiber.Ctx, message string, data interface{}, metadata interface{}) error {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return c.Status(fiber.StatusOK).JSON(SuccessBody{
		Status:   statusSuccess,
		Message:  message,
		Data:     data,
		Metadata: metadata,
	})
}

// SuccessCreated sends a 201 Created response with the standard success format.
func SuccessCreated(c *fiber.Ctx, message string, data interface{}, metadata interface{}) error {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	return c.Status(fiber.StatusCreated).JSON(SuccessBody{
		Status:   statusSuccess,
		Message:  message,
		Data:     data,
		Metadata: metadata,
	})
}

// Error sends a response with the standard error format.
func Error(c *fiber.Ctx, message string, statusCode int, details interface{}) error {
	if details == nil {
		details = map[string]interface{}{}
	}
	return c.Status(statusCode).JSON(ErrorBody{
		Status: statusError,
		Error: ErrorDetail{
			Message:    message,
			StatusCode: statusCode,
			Details:    details,
		},
	})
}

// Unauthorized sends 401 with the same shape as other errors (status "error", error.message).
// Use this for auth middleware so all errors are consistent.
func Unauthorized(c *fiber.Ctx, message string) error {
	return Error(c, message, fiber.StatusUnauthorized, nil)
}
