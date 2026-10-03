package api

import "github.com/gofiber/fiber/v2"

type Error struct {
}

type Response struct {
	Data   any      `json:"data"`
	Errors []string `json:"errors"`
}

func Success(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Data:   data,
		Errors: []string{},
	})
}

func Failure(c *fiber.Ctx, errs ...string) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Data:   nil,
		Errors: errs,
	})
}
