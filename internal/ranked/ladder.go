package ranked

import (
	"strconv"
	"strings"

	"nytrpg/internal/protocol"
)

// Every rank, lowest first. Each tier (Iron to Diamond) has divisions 3, 2 and 1,
// 1 being the best, each 100 elo wide: Iron 2 starts at 500, Diamond 1 at 1800.
// Iron 3 takes everything below 500, and Master everything from 1900.
var Ladder = func() []protocol.RankTier {
	var tiers []protocol.RankTier
	for _, family := range []string{"iron", "bronze", "silver", "gold", "diamond"} {
		for div := 3; div >= 1; div-- {
			tiers = append(tiers, protocol.RankTier{
				ID:     family + "-" + strconv.Itoa(div),
				Family: family,
				Name:   strings.ToUpper(family[:1]) + family[1:] + " " + strconv.Itoa(div),
				MinElo: 400 + 100*len(tiers),
			})
		}
	}
	tiers[0].MinElo = 0
	return append(tiers, protocol.RankTier{ID: "master", Family: "master", Name: "Master", MinElo: 400 + 100*len(tiers)})
}()

// The rank an elo is in
func TierFor(elo int) protocol.RankTier {
	t := Ladder[0]
	for _, tier := range Ladder {
		if elo >= tier.MinElo {
			t = tier
		}
	}
	return t
}
