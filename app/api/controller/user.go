package controller

import (
	"context"
	"hyperfocus/app/api"
	"hyperfocus/app/api/mapper"

	"github.com/elliotchance/pie/v2"
	"github.com/samber/oops"
)

func (s *Server) GetUser(ctx context.Context, req api.GetUserRequestObject) (api.GetUserResponseObject, error) {
	usr, err := s.userService.Get(ctx, req.Username)
	if err != nil {
		return nil, oops.Errorf("userService.Get: %w", err)
	}

	return api.GetUser200JSONResponse(mapper.MapUser(usr)), nil
}

func (s *Server) CreateUser(ctx context.Context, req api.CreateUserRequestObject) (api.CreateUserResponseObject, error) {
	if err := s.userService.Create(ctx, req.Body); err != nil {
		return nil, oops.Errorf("Create: %w", err)
	}

	return api.CreateUser200Response{}, nil
}

func (s *Server) EditUser(ctx context.Context, req api.EditUserRequestObject) (api.EditUserResponseObject, error) {
	if err := s.userService.Edit(ctx, req.Username, req.Body); err != nil {
		return nil, oops.Errorf("Edit: %w", err)
	}

	return api.EditUser200Response{}, nil
}

func (s *Server) DeleteUser(ctx context.Context, req api.DeleteUserRequestObject) (api.DeleteUserResponseObject, error) {
	if err := s.userService.Delete(ctx, req.Username); err != nil {
		return nil, oops.Errorf("Delete: %w", err)
	}

	return api.DeleteUser200Response{}, nil
}

func (s *Server) SearchUsers(ctx context.Context, req api.SearchUsersRequestObject) (api.SearchUsersResponseObject, error) {
	list, count, err := s.userService.Search(ctx, req.Body.Query, req.Body.Offset, req.Body.Limit)
	if err != nil {
		return nil, oops.Errorf("SearchUsers: %w", err)
	}

	return api.SearchUsers200JSONResponse{
		TotalCount: count,
		Users:      pie.Map(list, mapper.MapUser),
	}, nil
}
