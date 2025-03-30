package pkg

import (
	"encoding/json"
	"errors"
	"strings"

	jwtware "github.com/gofiber/contrib/jwt"
	"gitlab.com/voice-keyboard/backend-go/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/valyala/fasthttp"
)

type APIErrorHandler struct {
	logger logger.Logger
}

func (eh *APIErrorHandler) HandleError(ctx *fiber.Ctx, err error) error {
	// Status code defaults to 500
	code := fiber.StatusInternalServerError

	// Retrieve the custom status code if it's a *fiber.Error
	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
	}

	dump := DumpHttpRequest(ctx.Request())
	eh.logger.Error(err.Error(), dump)

	ctx.Status(code)
	body := NewResponseBodyError(err)
	_ = ctx.JSON(body)
	// Return from handler
	return nil
}

func (eh *APIErrorHandler) HandleAuthError(ctx *fiber.Ctx, err error) error {
	code := fiber.StatusUnauthorized
	if err.Error() == jwtware.ErrJWTMissingOrMalformed.Error() {
		code = fiber.StatusBadRequest
	}
	dump := DumpHttpRequest(ctx.Request())
	eh.logger.Error(err.Error(), dump)

	ctx.Status(code)
	body := NewResponseBodyError(err)
	_ = ctx.JSON(body)
	// Return from handler
	return nil
}

func DumpHttpRequest(req *fasthttp.Request) map[string]interface{} {
	dump := make(map[string]interface{})
	dump["url"] = req.URI().String()
	dump["method"] = string(req.Header.Method())
	dump["headers"] = strings.ReplaceAll(req.Header.String(), "\r\n", " ")
	if strings.Contains(string(req.Header.ContentType()), "multipart/form-data") {
		dump["body"] = "<binary contains>"
	} else {
		mp := new(map[string]interface{})
		err := json.Unmarshal(req.Body(), mp)
		if err != nil {
			dump["body"] = mp
		} else {
			dump["body"] = strings.ReplaceAll(strings.ReplaceAll(string(req.Body()), "\n", ""), "  ", "")
		}
	}
	return dump
}

func NewAPIErrorHandler(logger logger.Logger) *APIErrorHandler {
	return &APIErrorHandler{logger}
}
