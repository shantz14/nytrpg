// Package characters is the HTTP API for an account's characters: list, create
// and delete. Playing one happens by connecting to /ws with its id.
package characters

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"nytrpg/internal/auth"
	"nytrpg/internal/classes"
	"nytrpg/internal/protocol"
	"nytrpg/internal/store"
)

const MaxNameLen = 20

type Service struct {
	store *store.Store
	auth  *auth.Service
}

func New(s *store.Store, a *auth.Service) *Service {
	return &Service{store: s, auth: a}
}

func Info(c store.Character) protocol.CharacterInfo {
	return protocol.CharacterInfo{ID: c.ID, Slot: c.Slot, Name: c.Name, Class: c.Class}
}

type listRes struct {
	// One per slot, null if the slot is empty
	Slots   []*protocol.CharacterInfo `json:"slots"`
	Classes []protocol.ClassInfo      `json:"classes"`
}

type createReq struct {
	Slot  int    `json:"slot"`
	Name  string `json:"name"`
	Class string `json:"class"`
}

// GET lists, POST creates, DELETE ?id= deletes. All need a Bearer token.
func (svc *Service) Handle(w http.ResponseWriter, r *http.Request) {
	p, ok := svc.auth.PlayerFromRequest(r)
	if !ok {
		http.Error(w, "Invalid token.", http.StatusUnauthorized)
		return
	}
	switch r.Method {
	case http.MethodGet:
		svc.list(w, p)
	case http.MethodPost:
		svc.create(w, r, p)
	case http.MethodDelete:
		svc.delete(w, r, p)
	default:
		http.Error(w, "Method not allowed.", http.StatusMethodNotAllowed)
	}
}

func (svc *Service) list(w http.ResponseWriter, p store.Player) {
	chars, err := svc.store.Characters(p.ID)
	if err != nil {
		dbError(w, "listing characters", err)
		return
	}
	res := listRes{Slots: make([]*protocol.CharacterInfo, store.CharacterSlots), Classes: classes.Infos()}
	for _, c := range chars {
		info := Info(c)
		res.Slots[c.Slot] = &info
	}
	writeJSON(w, res)
}

func (svc *Service) create(w http.ResponseWriter, r *http.Request, p store.Player) {
	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if n := utf8.RuneCountInString(req.Name); n == 0 || n > MaxNameLen {
		http.Error(w, "Name must be 1-20 characters.", http.StatusBadRequest)
		return
	}
	if _, ok := classes.Get(classes.ID(req.Class)); !ok {
		http.Error(w, "Unknown class.", http.StatusBadRequest)
		return
	}
	if req.Slot < 0 || req.Slot >= store.CharacterSlots {
		http.Error(w, "Bad slot.", http.StatusBadRequest)
		return
	}

	c, taken, err := svc.store.InsertCharacter(p.ID, req.Slot, req.Name, req.Class)
	if err != nil {
		dbError(w, "creating character", err)
		return
	}
	if taken {
		http.Error(w, "That slot is taken.", http.StatusConflict)
		return
	}
	writeJSON(w, Info(c))
}

func (svc *Service) delete(w http.ResponseWriter, r *http.Request, p store.Player) {
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		http.Error(w, "Bad id.", http.StatusBadRequest)
		return
	}
	found, err := svc.store.DeleteCharacter(p.ID, id)
	if err != nil {
		dbError(w, "deleting character", err)
		return
	}
	if !found {
		http.Error(w, "No such character.", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// The character a player wants to play, if it's theirs and not deleted
func (svc *Service) Playable(p store.Player, id int) (store.Character, bool) {
	c, found, err := svc.store.CharacterByID(id)
	if err != nil {
		slog.Error("looking up character", "err", err)
		return c, false
	}
	return c, found && !c.Deleted && c.PlayerID == p.ID
}

func dbError(w http.ResponseWriter, what string, err error) {
	slog.Error(what, "err", err)
	http.Error(w, "Database error.", http.StatusInternalServerError)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encoding response", "err", err)
	}
}
