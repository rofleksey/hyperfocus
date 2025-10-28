package dto

import "hyperfocus/app/api"

const (
	AuthCookie = "hyperfocus_auth"
)

const (
	RoleAdmin         = "admin"
	RoleAuthenticated = "authenticated"
	RoleAnonymous     = "anonymous"
)

var AllPermissions = []api.Permission{
	api.PermissionUserList, api.PermissionUserCreate, api.PermissionUserDelete, api.PermissionUserRoles,
	api.PermissionSettingsEdit,
	api.PermissionAuthenticated,
}
