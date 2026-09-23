package resources

import (
	"bufio"
	"hash/fnv"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"
)

// The game day runs on fixed UTC-7 (MST, no DST).
var GameZone = time.FixedZone("MST", -7*60*60)

const DateLayout = "2006-01-02"

// Returns the game day of t as YYYY-MM-DD
func DateOf(t time.Time) string {
	return t.In(GameZone).Format(DateLayout)
}

// Returns the current game day as YYYY-MM-DD
func Today() string {
	return DateOf(time.Now())
}

type ResourceManager struct {
	solutions []string
	GuessableWords map[string]bool
}

func NewResourceManager() *ResourceManager {
	return &ResourceManager {
		solutions: make([]string, 0),
		GuessableWords: make(map[string]bool),
	}
}

// Loads everything once at startup. Must finish before the server takes requests,
// after that the manager is read only.
func (rm *ResourceManager) Load() {
	rm.LoadGuessables()
	rm.LoadSolutions()
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

func (rm *ResourceManager) LoadSolutions () {
	file, err := os.Open("resources/wordle-La.txt"); if err != nil {
		log.Fatal("Error opening file:", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		word := strings.TrimSpace(scanner.Text())
		if word != "" {
			rm.solutions = append(rm.solutions, strings.ToUpper(word))
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal("Error scanning file:", err)
	}
	if len(rm.solutions) == 0 {
		log.Fatal("No wordle solutions loaded.")
	}
	log.Println("The word of the day is: ", rm.WordleFor(Today()))
}

// The wordle for a given date. Same date always gives the same word, even across restarts.
func (rm *ResourceManager) WordleFor (date string) string {
	h := fnv.New64a()
	h.Write([]byte(date))
	r := rand.New(rand.NewSource(int64(h.Sum64())))
	return rm.solutions[r.Intn(len(rm.solutions))]
}

func (rm *ResourceManager) GetWordle () string {
	return rm.WordleFor(Today())
}
