package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hyperfocus/app/api"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/dto"
	"hyperfocus/app/service/auth"
	"hyperfocus/app/util"
	"hyperfocus/app/util/telemetry"
	"log/slog"
	"net/http"
	"time"

	"github.com/elliotchance/pie/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rofleksey/meg"
	"github.com/samber/do"
	"github.com/samber/oops"
)

var serviceName = "user"

type Service struct {
	cfg         *config.Config
	transactor  database.TxTransactor
	queries     database.TxQueries
	tracing     *telemetry.Tracing
	authService *auth.Service
}

func New(di *do.Injector) (*Service, error) {
	return &Service{
		cfg:         do.MustInvoke[*config.Config](di),
		transactor:  do.MustInvoke[database.TxTransactor](di),
		queries:     do.MustInvoke[database.TxQueries](di),
		tracing:     do.MustInvoke[*telemetry.Tracing](di),
		authService: do.MustInvoke[*auth.Service](di),
	}, nil
}

func (s *Service) Search(ctx context.Context, query string, offset, limit int) ([]database.User, int, error) {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "search")
	defer span.End()

	query = "%" + util.EscapeLikeQuery(query) + "%"

	users, err := s.queries.SearchUsers(ctx, database.SearchUsersParams{
		Query:  query,
		Limit:  int64(limit),
		Offset: int64(offset),
	})
	if err != nil {
		return nil, 0, s.tracing.Error(span, oops.Errorf("SearchUsers: %w", err))
	}

	count, err := s.queries.CountUsers(ctx, query)
	if err != nil {
		return nil, 0, s.tracing.Error(span, oops.Errorf("CountUsers: %w", err))
	}

	s.tracing.Success(span)

	return users, int(count), nil
}

func (s *Service) validateRoles(ctx context.Context, roles []string) error {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "validate_roles")
	defer span.End()

	usr := s.authService.ExtractFromCtx(ctx)
	if usr == nil {
		return s.tracing.Error(span, oops.
			With("status_code", http.StatusForbidden).
			Public("Access denied").
			Errorf("invalid user"))
	}

	if pie.Contains(roles, dto.RoleAdmin) {
		return s.tracing.Error(span, oops.
			With("status_code", http.StatusForbidden).
			Public("Can't create new admins").
			Errorf("can't create new admins"))
	}

	for _, role := range roles {
		if !s.authService.HasRole(role) {
			return s.tracing.Error(span, oops.With("status_code", http.StatusNotFound).
				Public(fmt.Sprintf("Role '%s' does not exist", role)).
				Errorf("role %s does not exist", role))
		}

		if !pie.Contains(usr.Roles, role) && !pie.Contains(usr.Roles, dto.RoleAdmin) {
			return s.tracing.Error(span, oops.
				With("status_code", http.StatusForbidden).
				Public(fmt.Sprintf("You don't have access to role '%s'", role)).
				Errorf("you don't have access to role: %s", role))
		}
	}

	s.tracing.Success(span)

	return nil
}

func (s *Service) Get(ctx context.Context, username string) (database.User, error) {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "get")
	defer span.End()

	usr, err := s.queries.GetUser(ctx, username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return database.User{}, s.tracing.Error(span, oops.
				With("status_code", http.StatusNotFound).
				Public("User not found").
				Errorf("user not found"))
		}

		return database.User{}, s.tracing.Error(span, oops.Errorf("GetUser: %w", err))
	}

	s.tracing.Success(span)

	return usr, nil
}

func (s *Service) Create(ctx context.Context, req *api.CreateUserRequest) error {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "create")
	defer span.End()

	if err := s.validateRoles(ctx, req.Roles); err != nil {
		return s.tracing.Error(span, oops.Errorf("validateRoles: %w", err))
	}

	passwordHash, err := meg.HashPassword(req.Password)
	if err != nil {
		return s.tracing.Error(span, oops.Errorf("HashPassword: %w", err))
	}

	if err := s.queries.CreateUser(ctx, database.CreateUserParams{
		Username:     req.Username,
		PasswordHash: passwordHash,
		Created:      time.Now(),
		Roles:        req.Roles,
	}); err != nil {
		return s.tracing.Error(span, oops.Errorf("CreateUser: %w", err))
	}

	s.tracing.Success(span)

	return nil
}

func (s *Service) Edit(ctx context.Context, username string, req *api.EditUserRequest) error {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "edit")
	defer span.End()

	usr := s.authService.ExtractFromCtx(ctx)
	if usr == nil {
		return s.tracing.Error(span, oops.
			With("status_code", http.StatusForbidden).
			Public("Access denied").
			Errorf("invalid user"))
	}

	err := s.transactor.Transaction(ctx, func(ctx context.Context, _ pgx.Tx, qtx database.TxQueries) error {
		userToEdit, err := qtx.GetUser(ctx, username)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return oops.
					With("status_code", http.StatusNotFound).
					Public("User not found").
					Errorf("user not found")
			}

			return oops.Errorf("GetUser: %w", err)
		}

		if pie.Contains(userToEdit.Roles, dto.RoleAdmin) && !pie.Contains(usr.Roles, dto.RoleAdmin) {
			return oops.
				With("status_code", http.StatusForbidden).
				Public("Can't edit admins").
				Errorf("can't edit admins")
		}

		if req.Password != nil {
			pwHash, err := meg.HashPassword(*req.Password)
			if err != nil {
				return oops.Errorf("HashPassword: %w", err)
			}

			if err = qtx.SetUserPasswordHash(ctx, database.SetUserPasswordHashParams{
				Username:     username,
				PasswordHash: pwHash,
			}); err != nil {
				return oops.Errorf("SetUserPasswordHash: %w", err)
			}
		}

		if err = s.validateRoles(ctx, req.Roles); err != nil {
			return oops.Errorf("validateRoles: %w", err)
		}

		if err = qtx.SetUserRoles(ctx, database.SetUserRolesParams{
			Username: username,
			Roles:    req.Roles,
		}); err != nil {
			return oops.Errorf("SetUserRoles: %w", err)
		}

		return nil
	})
	if err != nil {
		return s.tracing.Error(span, oops.Errorf("transactor.Transaction: %w", err))
	}

	s.tracing.Success(span)

	return nil
}

func (s *Service) Delete(ctx context.Context, username string) error {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "delete")
	defer span.End()

	usr := s.authService.ExtractFromCtx(ctx)
	if usr == nil {
		return s.tracing.Error(span, oops.Errorf("invalid user"))
	}

	err := s.transactor.Transaction(ctx, func(ctx context.Context, _ pgx.Tx, qtx database.TxQueries) error {
		userToDelete, err := qtx.GetUser(ctx, username)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return oops.
					With("status_code", http.StatusNotFound).
					Public("User not found").
					Errorf("user not found")
			}

			return oops.Errorf("GetUser: %w", err)
		}

		if pie.Contains(userToDelete.Roles, dto.RoleAdmin) && !pie.Contains(usr.Roles, dto.RoleAdmin) {
			return oops.
				With("status_code", http.StatusForbidden).
				Public("Can't delete admins").
				Errorf("can't delete admins")
		}

		if err = qtx.DeleteUser(ctx, username); err != nil {
			return oops.Errorf("DeleteUser: %w", err)
		}

		return nil
	})
	if err != nil {
		return oops.Errorf("transactor.Transaction: %w", err)
	}

	s.tracing.Success(span)

	return nil
}

func (s *Service) Init(ctx context.Context) error {
	passHash, err := meg.HashPassword(s.cfg.Admin.Password)
	if err != nil {
		return oops.Errorf("failed to hash password: %w", err)
	}

	if err = s.queries.CreateUser(ctx, database.CreateUserParams{
		Created:      time.Now(),
		Username:     "admin",
		PasswordHash: passHash,
		Roles:        []string{dto.RoleAdmin},
	}); err != nil {
		// in case admin already exists
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil
		}

		return oops.Errorf("failed to create admin user: %w", err)
	}

	slog.Info("Admin user created")

	return nil
}
