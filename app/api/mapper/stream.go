package mapper

import (
	"hyperfocus/app/api"
	"hyperfocus/app/database"

	"github.com/rofleksey/meg"
)

func MapStream(s database.Stream) api.Stream {
	return api.Stream{
		Name:      s.ID,
		Nicknames: meg.NonNilSlice(s.PlayerNames),
	}
}
