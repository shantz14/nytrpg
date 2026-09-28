// Command icongen draws the 16x16 pixel art icons for class abilities, from the
// text grids below, into client/static/assets/abilities.
//
//	go run ./cmd/icongen                       # or: make icons
//	go run ./cmd/icongen -preview sheet.png    # also a big sheet of all of them
//
// Each character in a grid is one pixel, looked up in palette. '.' is clear.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const size = 16

var palette = map[rune]color.NRGBA{
	'k': {0x1a, 0x16, 0x22, 0xff}, // outline
	'w': {0xf4, 0xf1, 0xe8, 0xff}, // white
	's': {0xc9, 0xcc, 0xd6, 0xff}, // steel
	'S': {0x7d, 0x82, 0x94, 0xff}, // dark steel
	'e': {0x8a, 0x85, 0x80, 0xff}, // stone
	'E': {0x4d, 0x49, 0x46, 0xff}, // dark stone
	'b': {0x8a, 0x5a, 0x2b, 0xff}, // brown
	'B': {0x5c, 0x3a, 0x1b, 0xff}, // dark brown
	'y': {0xf2, 0xc1, 0x4e, 0xff}, // gold
	'Y': {0xb0, 0x7d, 0x24, 0xff}, // dark gold
	'o': {0xf0, 0x8a, 0x3c, 0xff}, // orange
	'r': {0xd8, 0x43, 0x3b, 0xff}, // red
	'R': {0x8e, 0x1f, 0x1f, 0xff}, // dark red
	'u': {0x4a, 0x8f, 0xe0, 0xff}, // blue
	'U': {0x27, 0x4f, 0x99, 0xff}, // dark blue
	'c': {0x8f, 0xee, 0xf7, 0xff}, // cyan
	'p': {0xb0, 0x6e, 0xf0, 0xff}, // purple
	'P': {0x5e, 0x2d, 0x91, 0xff}, // dark purple
	'g': {0x5f, 0xbf, 0x5a, 0xff}, // green
	'G': {0x2f, 0x7a, 0x35, 0xff}, // dark green
}

// Named after the ability or passive IDs in internal/classes
var icons = map[string][]string{
	"slash": {
		"................",
		".............sw.",
		"............sws.",
		"...........sws..",
		"..........sws...",
		".........sws....",
		"........sws..r..",
		".......sws..r...",
		"......sws..r..r.",
		"..YY.sws..r..r..",
		"...YYws..r..r...",
		"....yY......r...",
		"...bYYY.........",
		"..bB..Y.........",
		".bB.............",
		"BB..............",
	},
	"shields_up": {
		"................",
		"..kkkkkkkkkkkk..",
		"..kyyyyyyyyyyk..",
		"..kyuuuuuuuuyk..",
		"..kyuuuwwuuuyk..",
		"..kyuuuwwuuuyk..",
		"..kyuwwwwwwuyk..",
		"..kyuwwwwwwuyk..",
		"..kyuuuwwuuuyk..",
		"..kyUuuwwuuUyk..",
		"...kyUuwwuUyk...",
		"...kyUUwwUUyk...",
		"....kyUUUUyk....",
		".....kyUUyk.....",
		"......kyyk......",
		".......kk.......",
	},
	"determination": {
		"................",
		".......o........",
		"......oo........",
		"......oyo.......",
		".....oyyo..o....",
		"....ooyyoo.oo...",
		"...oooyyyoooo...",
		"...ooyywyyooo...",
		"..oooyywwyyooo..",
		"..ooyywwwwyyoo..",
		"..ooyywwwwyyoo..",
		"..rooyywwyyoor..",
		"...roooyyooor...",
		"....rroooorr....",
		"......rrrr......",
		"................",
	},
	"pommel_strike": {
		"......sSSs......",
		"......sSSs......",
		"......sSSs......",
		"....YYYYYYYY....",
		"...YyyyyyyyyY...",
		"....YYYYYYYY....",
		"......bBBb......",
		"......bBBb......",
		"......bBBb......",
		".....YyyyyY.....",
		"....YyywyyyY....",
		"....YyyyyyyY....",
		".o...YYYYYY...o.",
		"..o..........o..",
		"o...r..oo..r...o",
		"..o...r..r...o..",
	},
	"cripple": {
		"................",
		"..EE............",
		".EwwE...........",
		".EwwwE..........",
		"..EwwwE.........",
		"...EwwwE........",
		"....EwwwE.r.....",
		".....EwwR..r....",
		"......R..r......",
		"....r..RwwwE....",
		".....r..EwwwE...",
		".........EwwwE..",
		"..........EwwwE.",
		"...........EwwE.",
		"............EE..",
		"................",
	},
	"aggressive": {
		"................",
		".s............s.",
		"..s..........s..",
		"...s...rr...s...",
		"....s.rRRr.s....",
		".....sRrrRs.....",
		"......s..s......",
		".......ss.......",
		".......ss.......",
		"......s..s......",
		"..Y..s....s..Y..",
		"...Ys......sY...",
		"...bY......Yb...",
		"..b..........b..",
		".b............b.",
		"................",
	},
	"scry": {
		"................",
		".....pppppp.....",
		"...ppPppppppp...",
		"..pPwwpppppppp..",
		"..pPwpppppppPp..",
		".pppppppppppPPp.",
		".ppppppcpppPPPp.",
		".pppppcccppPPPp.",
		".ppppppcpppPPPp.",
		"..pppppppppPPp..",
		"..ppppppppPPpp..",
		"...ppppppPPpp...",
		"....YYYYYYYY....",
		"...YyyyyyyyyY...",
		"..YyyyyyyyyyyY..",
		"..YYYYYYYYYYYY..",
	},
	"seeing_eye": {
		"................",
		"................",
		"................",
		"......kkkk......",
		"...kkkwwwwkkk...",
		"..kwwwUUUUwwwk..",
		".kwwwUuuwuUwwwk.",
		"kwwwUuukkuuUwwwk",
		"kwwwUuukkuuUwwwk",
		".kwwwUuuuuUwwwk.",
		"..kwwwUUUUwwwk..",
		"...kkkwwwwkkk...",
		"......kkkk......",
		"................",
		"................",
		"................",
	},
	"magic_missile": {
		"................",
		"...........PP...",
		"..........PccP..",
		".........Pcwwcp.",
		"........pPcwwcP.",
		".......ppPPccP..",
		"......ppPp.PP...",
		".....ppPp.......",
		"....ppPp........",
		"...ppPp.........",
		"..pPp...........",
		"..Pp............",
		"..p.............",
		"................",
		"................",
		"................",
	},
	"illusion": {
		"................",
		".....pppppp.....",
		"...pp......pp...",
		"..p...cccc...p..",
		".p...c....c...p.",
		".p..c..pp..c..p.",
		"p..c..p..p..c..p",
		"p..c..p.pp..c..p",
		"p..c..p....c...p",
		"p..c...pppp...p.",
		".p..c........p..",
		".p...cc....pp...",
		"..p....cccc.....",
		"...pp...........",
		".....pppp.......",
		"................",
	},
	"reshape_reality": {
		"................",
		"......yyyy......",
		"....yy....yy....",
		"...y..uuuu..yY..",
		"..y.uuggguu.YY..",
		"..yuuggguuuu....",
		".y.uguuugguuu.y.",
		".yuuuuuuggguu.y.",
		".y.ugguuuuguu.y.",
		"...uuggguuuuu.y.",
		"....uuuuguuu.y..",
		"..YY.uuuuuu.y...",
		"..Yy.......y....",
		"....yy....yy....",
		"......yyyy......",
		"................",
	},
	"wise": {
		"................",
		"..B..........B..",
		"..BB........BB..",
		"..bBbbbbbbbbBb..",
		".bbwwwbbbbwwwbb.",
		".bwwyywbbwyywwb.",
		".bwykywbbwykywb.",
		".bwwyywbbwyywwb.",
		".bbwwwboobwwwbb.",
		".bbbbbbbobbbbbb.",
		"..bbBbBbbBbBbb..",
		"..bBbBbBBbBbBb..",
		"...bbBbBBbBbb...",
		"....bbbbbbbb....",
		".....o....o.....",
		"................",
	},
	"pickpocket": {
		"................",
		"......BBBB......",
		".......bb.......",
		"......BbbB......",
		".....bbbbbb.....",
		"....bbbbbbbb....",
		"...bbbbyybbbb...",
		"...bbbyYYybbb...",
		"..bbbbyYYybbbb..",
		"..bbbbbyybbbbb..",
		"..bbbbbbbbbbbb..",
		"..BbbbbbbbbbbB..",
		"...BBbbbbbbBB...",
		"....BBBBBBBB....",
		"................",
		"................",
	},
	"cheat": {
		"................",
		"...kkkkkkkkkk...",
		"...kwrwwwwwwk...",
		"...kwwwwwwwwk...",
		"...kwrrwwrrwk...",
		"...krrrrrrrrk...",
		"...krrrrrrrrk...",
		"...kwrrrrrrwk...",
		"...kwwrrrrwwk...",
		"...kwwwrrwwwk...",
		"...kwwwwwwwwk...",
		"...kwwwwwwrwk...",
		"...kkkkkkkkkk...",
		"................",
		"................",
		"................",
	},
	"feint": {
		"................",
		"................",
		"................",
		".pppp......pppp.",
		"pPPPPp....pPPPPp",
		"pPkkPPppppPPkkPp",
		"pPkkkPPPPPPkkkPp",
		"pPPkPPPPPPPPkPPp",
		".pPPPPp..pPPPPp.",
		"..ppp......ppp..",
		"................",
		"................",
		"................",
		"................",
		"................",
		"................",
	},
	"under_their_nose": {
		"................",
		".kkkkkkkkkkkkkk.",
		".kggggggggggggk.",
		".kygggwwwwggggk.",
		".kyygwwggwwgggk.",
		".kyyyggggwwgggk.",
		".kyyyyggwwggggk.",
		".kyyyyywwgggggk.",
		".kyyyyywwgggggk.",
		".kyyyyyyygggggk.",
		".kyyyyywwygggGk.",
		".kyyyyywwyyggGk.",
		".kyyyyyyyyyyggk.",
		".kyyyyyyyyyyygk.",
		".kkkkkkkkkkkkkk.",
		"................",
	},
	"confuse": {
		"................",
		".SSSSS..........",
		".SsssS...ooo....",
		".SskSS.....oo...",
		".SksSS......o...",
		".SsssS.....ooo..",
		".SSSSS......o...",
		"................",
		"...o......SSSSS.",
		"..ooo.....SsssS.",
		"...o......SkkSS.",
		"...oo.....SsksS.",
		"....ooo...SsssS.",
		"..........SSSSS.",
		"................",
		"................",
	},
	"sneaky": {
		"................",
		".....GGGGGG.....",
		"...GGGGGGGGGG...",
		"..GGGkkkkkkGGG..",
		".GGGkkkkkkkkGGG.",
		".GGkkkkkkkkkkGG.",
		".GGkkykkkkykkGG.",
		".GGkkyykkyykkGG.",
		".GGkkkkkkkkkkGG.",
		".GGGkkkkkkkkGGG.",
		"..GGGkkkkkkGGG..",
		"..GGGGkkkkGGGG..",
		"...GGGGGGGGGG...",
		"..GgGGGGGGGGgG..",
		".GgGGGGGGGGGGgG.",
		"................",
	},
	"minor_prayer": {
		"................",
		".......y........",
		"......yyy.......",
		"......yoy.......",
		"......yoy.......",
		".......k........",
		"......wwww......",
		"......wwws......",
		"......wwws......",
		"......wwws......",
		"......wwws......",
		"......wwws......",
		"....YYYYYYYY....",
		"...YyyyyyyyyY...",
		"....YYYYYYYY....",
		"................",
	},
	"mend": {
		"................",
		"................",
		"......GGGG......",
		"......GggG......",
		"......GggG......",
		"......GggG......",
		"..GGGGGggGGGGG..",
		"..GggggwwggggG..",
		"..GggggwwggggG..",
		"..GGGGGggGGGGG..",
		"......GggG......",
		"......GggG......",
		"......GggG......",
		"......GGGG......",
		"................",
		"................",
	},
	"purify": {
		"................",
		".......u........",
		"......uuu.......",
		"......ucu.......",
		".....uccuu......",
		".....ucuuu......",
		"....uccuuuu.....",
		"....ucuuuuu.....",
		"...uucuuuuuu....",
		"...uuuuuuuuu....",
		"...uuuuuuuUu..w.",
		"...uuuuuuUUu.www",
		"....uuuuUUu...w.",
		".....UUUUU......",
		"................",
		"................",
	},
	"major_prayer": {
		"................",
		".......y........",
		"...y...y...y....",
		"....y..y..y.....",
		".....yyyyy......",
		"....yywwwyy.....",
		".yyyywwwwwyyyy..",
		"....yywwwyy.....",
		".....yyyyy......",
		"....y..y..y.....",
		"...y...y...y....",
		".......y........",
		"................",
		"...YYYYYYYYY....",
		"..YyyyyyyyyyY...",
		"................",
	},
	"divine_intervention": {
		"................",
		".......yy.......",
		"......yYYy......",
		"......yYYy......",
		".....yYYYYy.....",
		".....yYwwYy.....",
		"....yYwkkwYy....",
		"....yYwkkwYy....",
		"...yYYYwwYYYy...",
		"...yYYYYYYYYy...",
		"..yyyyyyyyyyyy..",
		"................",
		"..y...y..y...y..",
		".y....y..y....y.",
		"................",
		"................",
	},
	"divine_will": {
		"................",
		"....yyyyyyyy....",
		"...yY......Yy...",
		"....yyyyyyyy....",
		"................",
		".ww..........ww.",
		"wwww...uu...wwww",
		".wwww.uuuu.wwww.",
		"..wwwwuUUuwwww..",
		"...wwwuUUuwww...",
		"....wwuUUuww....",
		"......uUUu......",
		"......u..u......",
		"................",
		"................",
		"................",
	},
	// The energy meter's pips
	"energy": {
		"................",
		".........yyk....",
		"........yyyk....",
		".......yyyk.....",
		"......yyyk......",
		".....yyyk.......",
		"....yyyyyyyyk...",
		"...kkkkyyyyk....",
		"......yyyk......",
		".....yyyk.......",
		"....yyyk........",
		"...yyk..........",
		"..yk............",
		"................",
		"................",
		"................",
	},
}

func draw(name string, rows []string) (*image.NRGBA, error) {
	if len(rows) != size {
		return nil, fmt.Errorf("%s: %d rows, want %d", name, len(rows), size)
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y, row := range rows {
		if len([]rune(row)) != size {
			return nil, fmt.Errorf("%s row %d: %d pixels, want %d", name, y, len([]rune(row)), size)
		}
		for x, ch := range row {
			if ch == '.' {
				continue
			}
			c, ok := palette[ch]
			if !ok {
				return nil, fmt.Errorf("%s row %d: no color %q", name, y, ch)
			}
			img.SetNRGBA(x, y, c)
		}
	}
	return img, nil
}

func save(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func main() {
	out := flag.String("out", "client/static/assets/abilities", "where the icons go")
	preview := flag.String("preview", "", "also write every icon, scaled up, to this PNG")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}

	names := make([]string, 0, len(icons))
	for name := range icons {
		names = append(names, name)
	}
	slices.Sort(names)

	const scale, gap = 8, 4
	sheet := image.NewNRGBA(image.Rect(0, 0, len(names)*(size*scale+gap), size*scale))
	for i, name := range names {
		img, err := draw(name, icons[name])
		if err != nil {
			log.Fatal(err)
		}
		if err := save(filepath.Join(*out, name+".png"), img); err != nil {
			log.Fatal(err)
		}
		for y := range size * scale {
			for x := range size * scale {
				sheet.Set(i*(size*scale+gap)+x, y, img.At(x/scale, y/scale))
			}
		}
	}
	if *preview != "" {
		if err := save(*preview, sheet); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("%d icons in %s: %s\n", len(names), *out, strings.Join(names, ", "))
}
