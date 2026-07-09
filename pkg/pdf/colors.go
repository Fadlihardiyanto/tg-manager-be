package pdf

import (
	"strings"

	"github.com/johnfercher/maroto/v2/pkg/props"
)

var (
	navyHeader  = &props.Color{Red: 15, Green: 23, Blue: 42}
	navyMid     = &props.Color{Red: 30, Green: 41, Blue: 59}
	blue        = &props.Color{Red: 29, Green: 78, Blue: 216}
	blueSubtle  = &props.Color{Red: 59, Green: 130, Blue: 246}
	slate       = &props.Color{Red: 30, Green: 41, Blue: 59}
	muted       = &props.Color{Red: 100, Green: 116, Blue: 139}
	mutedLight  = &props.Color{Red: 148, Green: 163, Blue: 184}
	borderGray  = &props.Color{Red: 226, Green: 232, Blue: 240}
	surfaceGray = &props.Color{Red: 248, Green: 250, Blue: 252}
	white       = &props.Color{Red: 255, Green: 255, Blue: 255}

	greenBg   = &props.Color{Red: 220, Green: 252, Blue: 231}
	greenText = &props.Color{Red: 22, Green: 101, Blue: 52}
	amberBg   = &props.Color{Red: 254, Green: 243, Blue: 199}
	amberText = &props.Color{Red: 120, Green: 53, Blue: 15}
	redBg     = &props.Color{Red: 254, Green: 226, Blue: 226}
	redText   = &props.Color{Red: 153, Green: 27, Blue: 27}
	grayBg    = &props.Color{Red: 241, Green: 245, Blue: 249}
	grayText  = &props.Color{Red: 71, Green: 85, Blue: 105}
)

func statusColors(status string) (bg *props.Color, fg *props.Color, label string) {
	switch status {
	case "paid", "settlement":
		return greenBg, greenText, "LUNAS"
	case "pending":
		return amberBg, amberText, "MENUNGGU PEMBAYARAN"
	case "expired":
		return redBg, redText, "KADALUARSA"
	case "failed", "deny", "cancel", "failure":
		return redBg, redText, "GAGAL"
	default:
		return grayBg, grayText, strings.ToUpper(status)
	}
}
