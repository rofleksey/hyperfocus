package mapper

import (
	"hyperfocus/app/api"
	"hyperfocus/app/database"
)

func MapUser(u database.User) api.User {
	return api.User{
		Created:  u.Created,
		Roles:    u.Roles,
		Username: u.Username,
	}
}
