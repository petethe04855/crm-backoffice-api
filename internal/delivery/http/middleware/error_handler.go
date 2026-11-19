package middleware

import (
	"errors"
	"log"

	"backoffice-api/pkg/response"

	"github.com/gofiber/fiber/v2"
)

// ErrorHandler turns unhandled errors into the standard error envelope and
// never leaks internal error text to clients.
func ErrorHandler(c *fiber.Ctx, err error) error {
	var fe *fiber.Error
	if errors.As(err, &fe) {
		code := "HTTP_ERROR"
		switch fe.Code {
		case fiber.StatusNotFound:
			code = "NOT_FOUND"
		case fiber.StatusMethodNotAllowed:
			code = "METHOD_NOT_ALLOWED"
		case fiber.StatusRequestEntityTooLarge:
			code = "PAYLOAD_TOO_LARGE"
		}
		return response.Error(c, fe.Code, code, fe.Message)
	}
	log.Printf("[ERROR] %s %s: %v", c.Method(), c.Path(), err)
	return response.InternalServerError(c, "Internal server error")
}
