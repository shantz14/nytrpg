package main

func getColors(guess string) (WordleStatus, []WordleColor) {
	var colors []WordleColor
	var status WordleStatus

	return status, colors
}

type WordleStatus int

const (
	INGAME WordleStatus = 0
	WIN WordleStatus = 1
	LOSE WordleStatus = 2
)

type WordleColor int

const (
	GREY WordleColor = 0
	YELLOW WordleColor = 1
	GREEN WordleColor = 2
)

