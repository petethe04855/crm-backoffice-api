// Command richmenu creates the two LINE OA rich menus (plan L6) once:
//
//	guest   default for everyone: one big "สมัครสมาชิก / เข้าสู่ระบบ" area
//	member  linked per member after they join: card, code entry, rewards,
//	        coupons, points, profile
//
// Usage (images 2500×1686 PNG/JPEG, ≤ 1 MB, designed with the areas below):
//
//	go run ./cmd/richmenu -liff https://liff.line.me/<LIFF id> \
//	    -guest guest.png -member member.png
//
// The token comes from LINE_CHANNEL_ACCESS_TOKEN (environment or .env). The
// command prints LINE_MEMBER_RICH_MENU_ID to put in backoffice-api's .env.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"backoffice-api/pkg/line"

	"github.com/joho/godotenv"
)

const (
	width  = 2500
	height = 1686
)

type bounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type action struct {
	Type  string `json:"type"`
	URI   string `json:"uri"`
	Label string `json:"label,omitempty"`
}

type area struct {
	Bounds bounds `json:"bounds"`
	Action action `json:"action"`
}

type richMenu struct {
	Size        map[string]int `json:"size"`
	Selected    bool           `json:"selected"`
	Name        string         `json:"name"`
	ChatBarText string         `json:"chatBarText"`
	Areas       []area         `json:"areas"`
}

// GuestMenu is one area covering the whole image.
func GuestMenu(liff string) richMenu {
	return richMenu{
		Size: map[string]int{"width": width, "height": height}, Selected: true,
		Name: "guest", ChatBarText: "สมัครสมาชิก",
		Areas: []area{{Bounds: bounds{0, 0, width, height}, Action: action{Type: "uri", URI: liff + "/", Label: "สมัครสมาชิก"}}},
	}
}

// MemberMenu is a 3 × 2 grid, left to right, top to bottom.
func MemberMenu(liff string) richMenu {
	cells := []struct{ path, label string }{
		{"/home", "บัตรสมาชิก"}, {"/code", "กรอกรหัส"}, {"/rewards", "แลกรางวัล"},
		{"/coupons", "คูปอง"}, {"/points", "ประวัติแต้ม"}, {"/profile", "โปรไฟล์"},
	}
	w, h := width/3, height/2
	areas := make([]area, 0, len(cells))
	for i, c := range cells {
		col, row := i%3, i/3
		cw := w
		if col == 2 {
			cw = width - 2*w // absorb rounding
		}
		areas = append(areas, area{Bounds: bounds{col * w, row * h, cw, h}, Action: action{Type: "uri", URI: liff + c.path, Label: c.label}})
	}
	return richMenu{
		Size: map[string]int{"width": width, "height": height}, Selected: true,
		Name: "member", ChatBarText: "เมนูสมาชิก", Areas: areas,
	}
}

func readImage(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(data) > 1024*1024 {
		return nil, "", fmt.Errorf("%s is larger than 1 MB (LINE limit)", path)
	}
	ct := http.DetectContentType(data)
	if ct != "image/png" && ct != "image/jpeg" {
		return nil, "", fmt.Errorf("%s must be PNG or JPEG (got %s)", path, ct)
	}
	return data, ct, nil
}

func create(ctx context.Context, c *line.Client, menu richMenu, image string) (string, error) {
	data, ct, err := readImage(image)
	if err != nil {
		return "", err
	}
	id, err := c.CreateRichMenu(ctx, menu)
	if err != nil {
		return "", fmt.Errorf("create %s menu: %w", menu.Name, err)
	}
	if err := c.UploadRichMenuImage(ctx, id, data, ct); err != nil {
		return "", fmt.Errorf("upload %s image: %w", menu.Name, err)
	}
	return id, nil
}

func main() {
	liff := flag.String("liff", "", "LIFF URL, e.g. https://liff.line.me/1234567890-AbCdEfGh")
	guest := flag.String("guest", "", "guest menu image (2500×1686)")
	member := flag.String("member", "", "member menu image (2500×1686)")
	flag.Parse()

	_ = godotenv.Load()
	token := strings.TrimSpace(os.Getenv("LINE_CHANNEL_ACCESS_TOKEN"))
	switch {
	case token == "":
		log.Fatal("LINE_CHANNEL_ACCESS_TOKEN is not set")
	case !strings.HasPrefix(*liff, "https://liff.line.me/"):
		log.Fatal("-liff must be https://liff.line.me/<LIFF id>")
	case *guest == "" || *member == "":
		log.Fatal("-guest and -member images are required")
	}
	base := strings.TrimRight(*liff, "/")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := line.New(token, "")

	guestID, err := create(ctx, c, GuestMenu(base), *guest)
	if err != nil {
		log.Fatal(err)
	}
	memberID, err := create(ctx, c, MemberMenu(base), *member)
	if err != nil {
		log.Fatal(err)
	}
	if err := c.SetDefaultRichMenu(ctx, guestID); err != nil {
		log.Fatalf("set default menu: %v", err)
	}
	fmt.Printf("guest rich menu  %s (now the default)\nmember rich menu %s\n\nAdd to backoffice-api .env:\nLINE_MEMBER_RICH_MENU_ID=%s\n", guestID, memberID, memberID)
}
