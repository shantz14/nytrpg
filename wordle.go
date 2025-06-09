package main

import (
	"strings"

)

const GUESSES_ALLOWED = 5

func getColors(guess string, guessCount int, word string, guessables *map[string]bool) (bool, WordleStatus, []WordleColor) {
	valid, status, colors := colorMyBoxes(guess, word, guessCount, guessables)

	return valid, status, colors
}

func colorMyBoxes(guess string, word string, guessCount int, guessables *map[string]bool) (bool, WordleStatus, []WordleColor){
	var status WordleStatus
	var valid bool
	_, ok := (*guessables)[strings.ToLower(guess)]; if !ok {
		// TODO: wordles of different lengths
		valid = false
		status = INGAME
		colors := []WordleColor{GREY, GREY, GREY, GREY, GREY}
		return valid, status, colors
	}
	valid = true

	letterCounts := countLetters(word)
	colors := make([]WordleColor, len(word))

	lettersCounted := make(map[rune]int)
	for _, letter := range guess {
		lettersCounted[letter] = 0
	}

	//greys
	for i := range guess {
		colors[i] = GREY
	}
	//greens
	for i, letter := range guess {
		if (guess[i] == word[i]) {
			colors[i] = GREEN
			lettersCounted[letter] += 1
		}
	}
	//yellows
	for i, letter := range guess {
		if (lettersCounted[letter] < letterCounts[letter] && strings.ContainsRune(word, letter) && colors[i] == GREY) {
			colors[i] = YELLOW
			lettersCounted[letter]++
		}
	}

	for _, value := range colors {
		if value == YELLOW || value == GREY {
			status = INGAME
			if guessCount == GUESSES_ALLOWED {
				status = LOSE
			}
			break
		}
		status = WIN
	}

	return valid, status, colors
}


func countLetters(word string) map[rune]int {
	letters := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterCounts := make(map[rune]int)
	for _, letter := range letters {
		letterCounts[letter] = 0
	}
	for _, letter := range word {
		letterCounts[letter] += 1
	}
	return letterCounts
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

