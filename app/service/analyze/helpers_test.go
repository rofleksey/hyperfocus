package analyze

import (
	"context"
	"hyperfocus/app/client/twitch_live"
	"testing"

	"github.com/go-playground/assert/v2"
	"github.com/samber/do"
	"github.com/stretchr/testify/require"
)

func TestSelectStreamQuality(t *testing.T) {
	di := do.New()

	client, err := twitch_live.NewClient(di)
	require.NoError(t, err)
	require.NotNil(t, client)

	qualities, err := client.GetM3U8(context.Background(), "poltos_tv")
	require.NoError(t, err)
	require.NotNil(t, qualities)

	quality, err := selectOptimalStreamQuality(qualities)
	require.NoError(t, err)
	assert.Equal(t, "1920x1080", quality.Resolution)
}
