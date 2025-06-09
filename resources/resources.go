package resources

import (
	"bufio"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

var resourceManager ResourceManager

type ResourceManager struct {
	wordle string
	GuessableWords map[string]bool
}

func NewResourceManager() *ResourceManager {
	return &ResourceManager {
		wordle: "",
		GuessableWords: make(map[string]bool),
	}
}

func (rm *ResourceManager) LoadResources () {
	// Stuff that loads once
	rm.LoadGuessables()
	if rm.wordle == "" {rm.LoadWordle()}

	// Stuff that loads daily
	// Every day at midnight...
	for {
		// ct = current time
		mst := time.FixedZone("MST", -7*60*60)
		ct := time.Now().In(mst)
		midnight := time.Date(ct.Year(), ct.Month(), ct.Day(), 23, 59, 58, 0, mst)

		diff := midnight.Sub(ct)

		time.Sleep(diff)

		// THIS STUFF RUNS
		rm.LoadWordle()

		time.Sleep(time.Second * 5)

	}
}

func (rm *ResourceManager) LoadGuessables () {
	file, err := os.Open("resources/wordle-All.txt"); if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := scanner.Text()
		rm.GuessableWords[word] = true
	}

	if err := scanner.Err(); err != nil {
		log.Println("Error scanning file:", err)
		return
	}
}

func (rm *ResourceManager) LoadWordle () {
	solutions := make([]string, 0)
	file, err := os.Open("resources/wordle-La.txt"); if err != nil {
		log.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := scanner.Text()
		solutions = append(solutions, word)
	}

	if err := scanner.Err(); err != nil {
		log.Println("Error scanning file:", err)
		return
	}

	index := rand.Intn(len(solutions) - 1) + 1
	log.Println("The word of the day is: ", solutions[index])
	rm.wordle = strings.ToUpper(solutions[index])
}

func (rm *ResourceManager) GetWordle () string {
	return rm.wordle
}



