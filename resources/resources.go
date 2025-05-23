package resources

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

var resourceManager ResourceManager

type ResourceManager struct {
	wordle string
	guessableWords map[string]bool
}

func NewResourceManager() *ResourceManager {
	return &ResourceManager {
		wordle: "",
		guessableWords: make(map[string]bool),
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
		ct := time.Now()
		midnight := time.Date(ct.Year(), ct.Month(), ct.Day(), 23, 59, 58, ct.Nanosecond(), time.UTC)

		diff := ct.Sub(midnight)

		time.Sleep(diff)

		// THIS STUFF RUNS
		rm.LoadWordle()

		// maybe?
		time.Sleep(time.Second * 5)
	}
}

func (rm *ResourceManager) LoadGuessables () {
	file, err := os.Open("resources/wordle-All.txt"); if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := scanner.Text()
		rm.guessableWords[word] = true
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error scanning file:", err)
		return
	}
}

func (rm *ResourceManager) LoadWordle () {
	solutions := make([]string, 0)
	file, err := os.Open("resources/wordle-La.txt"); if err != nil {
		fmt.Println("Error opening file:", err)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := scanner.Text()
		solutions = append(solutions, word)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error scanning file:", err)
		return
	}

	index := rand.Intn(len(solutions) - 1) + 1
	fmt.Println("The word of the day is: ", solutions[index])
	rm.wordle = strings.ToUpper(solutions[index])
}

func (rm *ResourceManager) GetWordle () string {
	return rm.wordle
}



