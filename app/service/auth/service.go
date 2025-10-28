package auth

import (
	"context"
	"database/sql"
	"errors"
	"hyperfocus/app/api"
	"hyperfocus/app/config"
	"hyperfocus/app/database"
	"hyperfocus/app/dto"
	"hyperfocus/app/service/settings"
	"hyperfocus/app/util"
	"hyperfocus/app/util/telemetry"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rofleksey/meg"
	"github.com/rofleksey/rbac"
	"github.com/samber/do"
	"github.com/samber/oops"
	"go.opentelemetry.io/otel/attribute"
)

var serviceName = "auth"

type Service struct {
	cfg             *config.Config
	queries         database.TxQueries
	tracing         *telemetry.Tracing
	settingsService *settings.Service

	policy rbac.Policy
}

func New(di *do.Injector) (*Service, error) {
	cfg := do.MustInvoke[*config.Config](di)

	rolesMap := cfg.Auth.CustomRoles
	if len(rolesMap) == 0 {
		rolesMap = map[string][]api.Permission{
			dto.RoleAdmin:         {"*"},
			dto.RoleAuthenticated: {api.PermissionAuthenticated},
			dto.RoleAnonymous:     {},
		}
	}

	policyBuilder := rbac.NewPolicyBuilder()

	for _, permission := range dto.AllPermissions {
		if err := policyBuilder.RegisterPermission(string(permission)); err != nil {
			return nil, oops.Errorf("failed to register permission %s: %w", permission, err)
		}
	}

	for roleName, permissions := range rolesMap {
		if err := policyBuilder.RegisterRole(roleName); err != nil {
			return nil, oops.Errorf("failed to register role %s: %w", roleName, err)
		}

		for _, permission := range permissions {
			if err := policyBuilder.Grant(roleName, string(permission)); err != nil {
				return nil, oops.Errorf("failed to grant permission %s for role %s: %w", permission, roleName, err)
			}
		}
	}

	return &Service{
		cfg:             do.MustInvoke[*config.Config](di),
		queries:         do.MustInvoke[database.TxQueries](di),
		tracing:         do.MustInvoke[*telemetry.Tracing](di),
		settingsService: do.MustInvoke[*settings.Service](di),
		policy:          policyBuilder.Build(),
	}, nil
}

func (s *Service) IsGranted(usr *database.User, grantStr api.Permission) bool {
	var roles []string

	if usr == nil {
		roles = []string{dto.RoleAnonymous}
	} else {
		roles = []string{dto.RoleAuthenticated}
		roles = append(roles, usr.Roles...)
	}

	return s.policy.IsGranted(string(grantStr), roles...)
}

func (s *Service) HasRole(role string) bool {
	return s.policy.RoleExists(role)
}

func (s *Service) GetPermissions(usr *database.User) []string {
	var roles []string

	if usr == nil {
		roles = []string{dto.RoleAnonymous}
	} else {
		roles = []string{dto.RoleAuthenticated}
		roles = append(roles, usr.Roles...)
	}

	return s.policy.GetPermissions(roles...)
}

func (s *Service) ExtractFromCtx(ctx context.Context) *database.User {
	userOpt := ctx.Value(util.UserContextKey)
	if userOpt == nil {
		return nil
	}

	result, ok := userOpt.(*database.User)
	if !ok {
		return nil
	}

	return result
}

func (s *Service) ExtractFromLocals(locals func(key string, value ...interface{}) interface{}) *database.User {
	userOpt := locals(string(util.UserContextKey))
	if userOpt == nil {
		return nil
	}

	result, ok := userOpt.(*database.User)
	if !ok {
		return nil
	}

	return result
}

func (s *Service) ValidateApiKey(ctx context.Context, key string) (*database.User, error) {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "validate_api_key")
	defer span.End()

	appSettings, err := s.settingsService.Get(ctx)
	if err != nil {
		return nil, s.tracing.Error(span, oops.Errorf("settingsService.Get: %w", err))
	}

	if appSettings.ApiKey == nil || len(*appSettings.ApiKey) == 0 || meg.GetPtrOrZero(appSettings.ApiKey) != key {
		return nil, s.tracing.Error(span, oops.
			Public("Invalid API key").
			Errorf("invalid API key"))
	}

	adminUser, err := s.queries.GetUser(ctx, "admin")
	if err != nil {
		return nil, s.tracing.Error(span, oops.Errorf("failed to get admin user: %w", err))
	}

	s.tracing.Success(span)

	return &adminUser, nil
}

func (s *Service) ValidateToken(ctx context.Context, token *jwt.Token) (*database.User, error) {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "validate_token")
	defer span.End()

	username, err := token.Claims.GetSubject()
	if err != nil {
		return nil, s.tracing.Error(span, oops.Errorf("token.Claims.GetSubject: %w", err))
	}

	issuedAt, err := token.Claims.GetIssuedAt()
	if err != nil {
		return nil, s.tracing.Error(span, oops.Errorf("token.Claims.GetIssuedAt: %w", err))
	}

	tokenUser, err := s.queries.GetUser(ctx, username)
	if err != nil {
		return nil, s.tracing.Error(span, oops.Errorf("failed to get token user: %w", err))
	}

	lastSessionReset := tokenUser.LastSessionReset
	if lastSessionReset != nil && issuedAt.Before(*lastSessionReset) {
		return nil, s.tracing.Error(span, oops.Errorf("token is invalidated"))
	}

	s.tracing.Success(span)

	return &tokenUser, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	ctx, span := s.tracing.StartServiceSpan(ctx, serviceName, "login")
	defer span.End()

	span.SetAttributes(attribute.String("username", username))

	loginUser, err := s.queries.GetUser(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", s.tracing.Error(span, oops.
				With("status_code", http.StatusUnauthorized).
				Public("Invalid username or password").
				Errorf("Invalid username or password"))
		}

		return "", s.tracing.Error(span, oops.Errorf("failed to get user by username: %w", err))
	}

	if !meg.CheckPasswordHash(loginUser.PasswordHash, password) {
		return "", s.tracing.Error(span, oops.
			With("status_code", http.StatusUnauthorized).
			Public("Invalid username or password").
			Errorf("Invalid username or password"))
	}

	claims := jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 24 * 30).Unix(),
		"sub": username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenStr, err := token.SignedString([]byte(s.cfg.Auth.JWT.Secret))
	if err != nil {
		return "", s.tracing.Error(span, oops.Errorf("failed to sign token: %w", err))
	}

	s.tracing.Success(span)

	return tokenStr, nil
}
