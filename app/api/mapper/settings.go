package mapper

import (
	"hyperfocus/app/api"
	"hyperfocus/app/database"

	"github.com/rofleksey/meg"
)

func MapSettings(s database.Setting) api.Settings {
	return api.Settings{
		ApiKey: meg.GetPtrOrZero(s.ApiKey),
	}
}
