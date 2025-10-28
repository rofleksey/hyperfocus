package controller

import (
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/dto"
	"hyperfocus/app/util"
	"net/http"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/rofleksey/meg"
	"github.com/samber/oops"
)

func (s *Server) Login(ctx context.Context, request api.LoginRequestObject) (api.LoginResponseObject, error) {
	if !s.limitsService.AllowIpRpm(ctx, "login", 5) {
		return nil, oops.
			With("status_code", http.StatusTooManyRequests).
			Public("Too many requests").
			New("Too many requests")
	}

	token, err := s.authService.Login(ctx, request.Body.Username, request.Body.Password)
	if err != nil {
		return nil, err
	}

	util.GetFiberFromContext(ctx).Cookie(&fiber.Cookie{
		Name:     dto.AuthCookie,
		Value:    token,
		Expires:  time.Now().Add(24 * time.Hour),
		HTTPOnly: true,
		Secure:   meg.Environment == "production",
		Path:     "/",
		SameSite: fiber.CookieSameSiteLaxMode,
	})

	return api.Login200Response{}, nil
}

func (s *Server) Logout(ctx context.Context, _ api.LogoutRequestObject) (api.LogoutResponseObject, error) {
	if !s.limitsService.AllowIpRpm(ctx, "logout", 5) {
		return nil, oops.
			With("status_code", http.StatusTooManyRequests).
			Public("Too many requests").
			New("Too many requests")
	}

	util.GetFiberFromContext(ctx).Cookie(&fiber.Cookie{
		Name:     dto.AuthCookie,
		Value:    "",
		Expires:  time.Now().Add(-1 * time.Hour),
		Path:     "/",
		HTTPOnly: true,
	})

	return api.Logout200Response{}, nil
}

func (s *Server) GetMyself(ctx context.Context, _ api.GetMyselfRequestObject) (api.GetMyselfResponseObject, error) {
	usr := s.authService.ExtractFromCtx(ctx)
	if usr == nil {
		return api.GetMyself401JSONResponse{}, nil
	}

	return api.GetMyself200JSONResponse{
		Username: usr.Username,
		Permissions: pie.Map(s.authService.GetPermissions(usr), func(p string) api.Permission {
			return api.Permission(p)
		}),
	}, nil
}
