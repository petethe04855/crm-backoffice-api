package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMenusCoverTheImage(t *testing.T) {
	g := GuestMenu("https://liff.line.me/x")
	require.Len(t, g.Areas, 1)
	require.Equal(t, "https://liff.line.me/x/", g.Areas[0].Action.URI)

	m := MemberMenu("https://liff.line.me/x")
	require.Len(t, m.Areas, 6)
	area := 0
	for _, a := range m.Areas {
		b := a.Bounds
		require.LessOrEqual(t, b.X+b.Width, width)
		require.LessOrEqual(t, b.Y+b.Height, height)
		area += b.Width * b.Height
	}
	require.Equal(t, width*(height/2)*2, area, "the grid tiles the image without gaps")
	require.Equal(t, "https://liff.line.me/x/code", m.Areas[1].Action.URI)
}
