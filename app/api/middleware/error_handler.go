// nolint: wrapcheck
package middleware

import (
	"hyperfocus/app/api"
	"log/slog"
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/samber/oops"
)

func ErrorHandler(ctx *fiber.Ctx, err error) error {
	statusCode := http.StatusInternalServerError

	if oopsErr, ok := oops.AsOops(err); ok {
		statusCodeOpt := oopsErr.Context()["status_code"]
		if statusCodeOpt != nil {
			statusCode, _ = statusCodeOpt.(int)
		}
	}

	if statusCode == http.StatusInternalServerError {
		sentry.CaptureException(err)
		slog.ErrorContext(ctx.UserContext(), "Internal Server Error", slog.Any("error", err))
	}

	general := api.General{
		Error:      true,
		Msg:        oops.GetPublic(err, http.StatusText(statusCode)),
		StatusCode: statusCode,
	}

	ctx.Response().Header.Set("Content-Type", "application/json")
	ctx.Status(general.StatusCode)
	return ctx.JSON(general)
}
