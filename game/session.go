package game

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/BigJk/end_of_eden/internal/fs"
	"github.com/BigJk/end_of_eden/internal/lua/ludoc"
	"github.com/BigJk/end_of_eden/system/gen"
	"github.com/BigJk/end_of_eden/system/gen/faces"
	"github.com/BigJk/end_of_eden/system/localization"
	"github.com/BigJk/end_of_eden/ui"
	"github.com/samber/lo"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
	"io"
	"log"
	"math/rand"
	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	"oss.terrastruct.com/d2/lib/textmeasure"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func init() {
	gob.Register(FightState{})
	gob.Register(MerchantState{})
}

// GameState represents the current state of the game.
type GameState string

const (
	GameStateFight    = GameState("FIGHT")
	GameStateMerchant = GameState("MERCHANT")
	GameStateEvent    = GameState("EVENT")
	GameStateRandom   = GameState("RANDOM")
	GameStateGameOver = GameState("GAME_OVER")
)

const (
	// DefaultUpgradeCost is the default cost for upgrading a card.
	DefaultUpgradeCost = 65

	// DefaultRemoveCost is the default cost for removing a card.
	DefaultRemoveCost = 50

	// PointsPerRound is the amount of points the player gets per round.
	PointsPerRound = 3

	// DrawSize is the amount of cards the player draws per round.
	DrawSize = 3
)

type Hook string

const (
	HookNextFightEnd = Hook("NextFightEnd")
)

// FightState represents the current state of the fight in regard to the
// deck of the player.
type FightState struct {
	Round         int
	Description   string
	CurrentPoints int
	Deck          []string
	Hand          []string
	Used          []string
	Exhausted     []string
}

// MerchantState represents the current state of the merchant.
type MerchantState struct {
	Face      string
	Text      string
	Cards     []string
	Artifacts []string
}

// LuaError represents an error that occurred in lua.
type LuaError struct {
	File     string
	Line     int
	Callback string
	Type     string
	Err      error
}

// Session represents the state inside a game session.
type Session struct {
	log       *log.Logger
	luaState  *lua.LState
	luaDocs   *ludoc.Docs
	resources *ResourcesManager

	state         GameState
	actors        map[string]Actor
	instances     map[string]any
	stagesCleared int
	currentEvent  string
	currentFight  FightState
	merchant      MerchantState
	eventHistory  []string
	randomHistory []string
	ctxData       map[string]any
	hooks         map[Hook][]func()

	loadedMods       []string
	stateCheckpoints []StateCheckpoint
	closer           []func() error
	onLuaError       func(file string, line int, callback string, typeId string, err error)
	luaErrors        chan LuaError

	Logs []LogEntry
}

// NewSession creates a new game session.
func NewSession(options ...func(s *Session)) *Session {
	session := &Session{
		log:   log.New(io.Discard, "", 0),
		state: GameStateEvent,
		actors: map[string]Actor{
			PlayerActorID: NewActor(PlayerActorID),
		},
		instances: map[string]any{},
		ctxData:   map[string]any{},
		hooks: map[Hook][]func(){
			HookNextFightEnd: {},
		},
		stagesCleared: 0,
		onLuaError:    nil,
		luaErrors:     make(chan LuaError, 25),
		eventHistory:  []string{},
		randomHistory: []string{},
	}
	session.SetOnLuaError(nil)

	session.luaState, session.luaDocs = SessionAdapter(session)

	for i := range options {
		if options[i] == nil {
			continue
		}
		options[i](session)
	}

	session.resources = NewResourcesManager(session.luaState, session.luaDocs, session.log)
	session.resources.MarkBaseGame()
	session.loadMods(session.loadedMods)

	session.log.Println("Session started!")

	session.UpdatePlayer(func(actor *Actor) bool {
		actor.HP = 80
		actor.MaxHP = 80
		actor.Gold = 50 + rand.Intn(50)
		return true
	})

	session.SetEvent("START")

	return session
}

// WithDebugEnabled enables the lua debugging. With lua debugging a server will be started
// on the given bind port. This exposes the /ws route to connect over websocket to. In essence,
// it exposes REPL access to the internal lua state which is helpful to debug problems. You can use
// the debug_r function to send data back to the websocket.
//
// Tip: Use https://github.com/websockets/wscat to connect and talk with it.
func WithDebugEnabled(port int) func(s *Session) {
	return func(s *Session) {
		s.closer = append(s.closer, ExposeDebug(port, s, s.luaState, s.log))
	}
}

// WithLogging sets the internal logger.
func WithLogging(logger *log.Logger) func(s *Session) {
	return func(s *Session) {
		s.log = logger
	}
}

// WithMods sets the mods that should be loaded.
func WithMods(mods []string) func(s *Session) {
	return func(s *Session) {
		s.loadedMods = mods
	}
}

// WithOnLuaError sets the function that will be called when a lua error happens.
func WithOnLuaError(fn func(file string, line int, callback string, typeId string, err error)) func(s *Session) {
	return func(s *Session) {
		s.onLuaError = fn
	}
}

// SetOnLuaError sets the function that will be called when a lua error happens.
func (s *Session) SetOnLuaError(fn func(file string, line int, callback string, typeId string, err error)) {
	if fn == nil {
		s.onLuaError = func(file string, line int, callback string, typeId string, err error) {}
	} else {
		s.onLuaError = fn
	}
}

// LuaDocs returns the documentation of the lua state.
func (s *Session) LuaDocs() *ludoc.Docs {
	return s.luaDocs
}

// Close closes the internal lua state and everything else.
func (s *Session) Close() {
	for i := range s.closer {
		if err := s.closer[i](); err != nil {
			s.log.Println("Close error:", err)
		}
	}
	s.luaState.Close()
}

// LuaErrors returns a channel that will receive all lua errors that happen during the session.
// Only a single channel is used for all errors, so be wary when using this in multiple goroutines.
func (s *Session) LuaErrors() chan LuaError {
	return s.luaErrors
}

// ToSavedState creates a saved state of the session that can be serialized with Gob.
func (s *Session) ToSavedState() SavedState {
	return SavedState{
		State:            s.state,
		Actors:           s.actors,
		Instances:        s.instances,
		StagesCleared:    s.stagesCleared,
		CurrentEvent:     s.currentEvent,
		CurrentFight:     s.currentFight,
		Merchant:         s.merchant,
		EventHistory:     s.eventHistory,
		StateCheckpoints: s.stateCheckpoints,
		CtxData:          s.ctxData,
		LoadedMods:       s.loadedMods,
	}
}

// LoadSavedState applies a saved state to the session. This will overwrite all game related data, but
// not the lua state, logging etc. This also means that for a save file to work the same lua scripts
// should be loaded or the state could be corrupted.
func (s *Session) LoadSavedState(save SavedState) {
	s.state = save.State
	s.actors = lo.MapValues(save.Actors, func(item Actor, key string) Actor {
		return item.Sanitize()
	})
	s.instances = save.Instances
	s.stagesCleared = save.StagesCleared
	s.currentEvent = save.CurrentEvent
	s.currentFight = save.CurrentFight
	s.merchant = save.Merchant
	s.eventHistory = save.EventHistory
	s.stateCheckpoints = lo.Map(save.StateCheckpoints, func(item StateCheckpoint, index int) StateCheckpoint {
		item.Session = s
		return item
	})
	s.ctxData = save.CtxData
	s.loadedMods = save.LoadedMods

	// Don't load mods from settings but from the saved list!
	s.loadMods(s.loadedMods)
}

func (s *Session) GobEncode() ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := gob.NewEncoder(buf)
	err := enc.Encode(s.ToSavedState())
	return buf.Bytes(), err
}

func (s *Session) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	var saved SavedState
	if err := dec.Decode(&saved); err != nil {
		return err
	}
	s.LoadSavedState(saved)
	return nil
}

// GetResources returns the resources manager.
func (s *Session) GetResources() *ResourcesManager {
	return s.resources
}

// GetLoadedMods returns the list of loaded mods.
func (s *Session) GetLoadedMods() []string {
	return s.loadedMods
}

//
// Internal
//

func (s *Session) logLuaError(callback string, typeId string, err error) {
	_, file, no, ok := runtime.Caller(1)
	if ok {
		s.log.Printf("%s:%d Error from Lua:%s type=%s %s\n", file, no, callback, typeId, err.Error())
		s.onLuaError(file, no, callback, typeId, err)
		s.luaErrors <- LuaError{
			File:     file,
			Line:     no,
			Callback: callback,
			Type:     typeId,
			Err:      err,
		}
	} else {
		s.log.Printf("Error from Lua:%s type=%s %s\n", callback, typeId, err.Error())
		s.onLuaError("", 0, callback, typeId, err)
		s.luaErrors <- LuaError{
			File:     "",
			Line:     0,
			Callback: callback,
			Type:     typeId,
			Err:      err,
		}
	}
}

func (s *Session) loadMods(mods []string) {
	for i := range mods {
		mod, err := ModDescription(filepath.Join("./mods", mods[i]))
		if err != nil {
			log.Println("Error loading mod:", err)
		} else {
			log.Println("Loading mod:", mod.Name)
		}

		_ = fs.Walk(filepath.Join("./mods", mods[i]), func(path string, isDir bool) error {
			if strings.Contains(path, "__") {
				return nil
			}

			// If we find a locals folder we add it to the localization
			if isDir && filepath.Base(path) == "locals" {
				_ = localization.Global.AddFolder(path)
			}

			if !isDir && strings.HasSuffix(path, ".lua") {
				luaBytes, err := fs.ReadFile(path)
				if err != nil {
					// TODO: error handling
					panic(err)
				}

				if strings.HasPrefix(string(luaBytes), "---@meta") {
					return nil
				}

				if err := s.luaState.DoString(string(luaBytes)); err != nil {
					s.logLuaError("ModLoader", "", err)
				}
			}

			return nil
		})
	}
}

//
// Checkpoints
//

// MarkState creates a checkpoint of the session state that can be used to diff and see what happened
// between two points in time.
func (s *Session) MarkState() StateCheckpointMarker {
	return StateCheckpointMarker{checkpoints: s.stateCheckpoints}
}

// PushState pushes a new state to the session. New states are relevant information like damage done,
// money received, actor death etc.
func (s *Session) PushState(events map[StateEvent]any) {
	savedState := *s

	// Only have the current session have the state checkpoints
	savedState.stateCheckpoints = make([]StateCheckpoint, 0)
	savedState.actors = lo.MapValues(CopyMap(savedState.actors), func(actor Actor, key string) Actor {
		return actor.Clone()
	})
	savedState.instances = CopyMap(savedState.instances)

	s.stateCheckpoints = append(s.stateCheckpoints, StateCheckpoint{
		Session: &savedState,
		Events:  events,
	})
}

// GetFormerState iterates backwards over the states, so index == -1 means the last state and so on.
func (s *Session) GetFormerState(index int) *Session {
	if index == 0 {
		return s
	}

	index = len(s.stateCheckpoints) + index
	if index >= len(s.stateCheckpoints) {
		return nil
	}

	return s.stateCheckpoints[index].Session
}

//
// Game State Functions
//

// GetGameState returns the current game state.
func (s *Session) GetGameState() GameState {
	return s.state
}

// SetGameState sets the game state and applies all needed setups for the new state to be valid.
func (s *Session) SetGameState(state GameState) {
	s.state = state

	switch s.state {
	case GameStateFight:
		s.SetupFight()
	case GameStateRandom:
		s.LetTellerDecide()
	case GameStateMerchant:
		s.SetupMerchant()
	}
}

// SetEvent changes the active event, but won't set the game state to EVENT. So this can be used
// to set the next event even before a fight or merchant interaction is over.
func (s *Session) SetEvent(id string) {
	s.currentEvent = id
	if _, ok := s.resources.Events[id]; ok {
		s.eventHistory = append(s.eventHistory, id)
		_, _ = s.resources.Events[id].OnEnter.Call(CreateContext("type_id", id))
	}
}

// GetEventID returns the id of the current event.
func (s *Session) GetEventID() string {
	return s.currentEvent
}

// GetEvent returns the event definition of the current event. Will be nil if no event is present.
// It is not allowed to change the Event data, as this points to the event data created in lua!
func (s *Session) GetEvent() *Event {
	if len(s.currentEvent) == 0 {
		return nil
	}
	return s.resources.Events[s.currentEvent]
}

// CleanUpFight resets the fight state.
func (s *Session) CleanUpFight() {
	s.currentFight.CurrentPoints = PointsPerRound
	s.currentFight.Deck = lo.Shuffle(s.GetPlayer().Cards.ToSlice())
	s.currentFight.Hand = []string{}
	s.currentFight.Exhausted = []string{}
	s.currentFight.Used = []string{}
	s.currentFight.Round = 0
}

// SetupFight setups the fight state, which means removing all leftover status effects, cleaning the state
// drawing the initial hand size and trigger the first wave of OnPlayerTurn callbacks.
//
// Additionally, this will create a save file as this is a clean state to save.
func (s *Session) SetupFight() {
	s.RemoveAllStatusEffects()
	s.CleanUpFight()
	s.PlayerDrawCard(DrawSize)

	// Trigger OnPlayerTurn callbacks
	TriggerCallbackSimple(s, CallbackOnPlayerTurn, TriggerAll, nil)

	// Save after each fight
	{
		save, err := s.GobEncode()
		if err != nil {
			s.log.Println("Error saving file:", save)
		} else {
			if err := fs.WriteFile("./session.save", save); err != nil {
				s.log.Println("Error saving file:", save)
			}
		}
	}
}

// GetFight returns the fight state. This will return a fight state even if no fight is active at the moment.
func (s *Session) GetFight() FightState {
	return s.currentFight
}

// GetStagesCleared returns the amount of stages cleared so far. Each fight represent a stage.
func (s *Session) GetStagesCleared() int {
	return s.stagesCleared
}

// FinishPlayerTurn signals that the player is done with its turn. All enemies act now, status effects are
// evaluated, if the fight is over is checked and if not this will advance to the next round and draw cards
// for the player.
func (s *Session) FinishPlayerTurn() {
	// Enemies are allowed to act.
	s.EnemyTurn()

	// Turn over so we remove all dead status effects.
	var removeStatus []string

	instanceKeys := lo.Keys(s.instances)
	for _, guid := range instanceKeys {
		switch instance := s.instances[guid].(type) {
		case StatusEffectInstance:
			// TODO: investigate why this was here
			// if instance.Owner == PlayerActorID && instance.RoundEntered == s.currentFight.Round {
			// 	continue
			// }

			se := s.resources.StatusEffects[instance.TypeID]

			// If it can decay we reduce rounds.
			if se.Decay != DecayNone {
				instance.RoundsLeft -= 1
				s.instances[guid] = instance
			}

			// Enemy StatusEffect OnTurn were already done in EnemyTurn(). We only let
			// the player owned ones turn now.
			if instance.Owner == PlayerActorID {
				if _, err := s.GetStatusEffect(guid).Callbacks[CallbackOnTurn].Call(CreateContext("type_id", instance.TypeID, "guid", guid, "owner", instance.Owner, "round", s.currentFight.Round, "stacks", instance.Stacks)); err != nil {
					s.logLuaError(CallbackOnTurn, instance.TypeID, err)
				}
			}

			switch se.Decay {
			// Decay stacks by one and re-set rounds if stacks left.
			case DecayOne:
				if instance.RoundsLeft <= 0 {
					instance.Stacks -= 1
					instance.RoundsLeft = se.Rounds
					s.instances[guid] = instance

					if instance.Stacks <= 0 {
						removeStatus = append(removeStatus, guid)
					}
				}
			// Remove all.
			case DecayAll:
				if instance.RoundsLeft <= 0 {
					removeStatus = append(removeStatus, guid)
				}
			}
		}
	}

	for i := range removeStatus {
		s.RemoveStatusEffect(removeStatus[i])
	}

	if s.FinishFight() {
		return
	}

	// Advance to new Round
	s.currentFight.CurrentPoints = PointsPerRound
	s.currentFight.Round += 1
	s.currentFight.Used = append(s.currentFight.Used, s.currentFight.Hand...)
	s.currentFight.Hand = []string{}

	s.PlayerDrawCard(DrawSize)

	// Trigger OnPlayerTurn callbacks
	TriggerCallbackSimple(s, CallbackOnPlayerTurn, TriggerAll, nil)
}

// EnemyTurn lets all enemies act. This will also trigger the OnTurn callbacks of all status effects and
// artifacts. If a status effect or artifact returns true, the enemy turn will be skipped. This is used
// for example by the "FEAR" status effect.
func (s *Session) EnemyTurn() {
	for k, v := range s.actors {
		if k == PlayerActorID || v.IsNone() {
			continue
		}

		if enemy, ok := s.resources.Enemies[v.TypeID]; ok {
			skipTurn := false
			s.TraverseArtifactsStatus(append(v.Artifacts.ToSlice(), v.StatusEffects.ToSlice()...),
				func(instance ArtifactInstance, artifact *Artifact) {
					res, err := artifact.Callbacks[CallbackOnTurn].Call(CreateContext("type_id", artifact.ID, "guid", instance.GUID, "owner", instance.Owner, "round", s.GetFightRound()))
					if err != nil {
						s.logLuaError(CallbackOnTurn, artifact.ID, err)
					} else if skip, ok := res.(bool); ok && skip {
						skipTurn = true
					}
				},
				func(instance StatusEffectInstance, statusEffect *StatusEffect) {
					res, err := statusEffect.Callbacks[CallbackOnTurn].Call(CreateContext("type_id", statusEffect.ID, "guid", instance.GUID, "owner", instance.Owner, "round", s.GetFightRound(), "stacks", instance.Stacks))
					if err != nil {
						s.logLuaError(CallbackOnTurn, statusEffect.ID, err)
					} else if skip, ok := res.(bool); ok && skip {
						skipTurn = true
					}
				},
			)

			// An effect like FEAR aborted the turn for this actor.
			if skipTurn {
				continue
			}

			if _, err := enemy.Callbacks[CallbackOnTurn].Call(CreateContext("type_id", v.TypeID, "guid", k, "round", s.currentFight.Round)); err != nil {
				s.logLuaError(CallbackOnTurn, v.TypeID, err)
			}
		}
	}
}

// FinishFight tries to finish the fight. This will return true if the fight is really over.
func (s *Session) FinishFight() bool {
	if s.GetOpponentCount(PlayerActorID) == 0 {
		s.currentFight.Description = ""
		s.stagesCleared += 1
		s.CleanUpFight()
		s.RemoveAllStatusEffects()

		// If an event is already set we switch to it
		if len(s.currentEvent) > 0 {
			s.SetGameState(GameStateEvent)
		} else if s.stagesCleared%10 == 0 {
			s.SetEvent("MERCHANT")
		} else {
			s.SetGameState(GameStateRandom)
		}

		// Trigger HookNextFightEnd
		s.TriggerHooks(HookNextFightEnd)
	}
	return false
}

// FinishEvent finishes an event with the given choice. If the game state is not in the EVENT state this
// does nothing.
func (s *Session) FinishEvent(choice int) {
	if len(s.currentEvent) == 0 || s.state != GameStateEvent {
		return
	}

	s.RemoveNonPlayer()

	event := s.resources.Events[s.currentEvent]
	s.currentEvent = ""

	// If choice was selected and valid we try to use the next game state from the choice.
	if choice >= 0 && choice < len(event.Choices) {
		nextState, _ := event.Choices[choice].Callback(CreateContext("type_id", event.ID, "choice", choice+1))

		// If the choice dictates a new state we take that
		if nextState != nil {
			if len(nextState.(string)) > 0 {
				s.SetGameState(GameState(nextState.(string)))
			} else {
				s.SetGameState(GameStateRandom)
			}
			_, _ = event.OnEnd.Call(CreateContext("type_id", event.ID, "choice", choice+1))
			return
		}

		// Otherwise we allow OnEnd to dictate the new state
		nextState, _ = event.OnEnd.Call(CreateContext("type_id", event.ID, "choice", choice+1))
		if nextState != nil && len(nextState.(string)) > 0 {
			s.SetGameState(GameState(nextState.(string)))
		} else {
			s.SetGameState(GameStateRandom)
		}
		return
	}

	nextState, _ := event.OnEnd.Call(CreateContext("type_id", event.ID, "choice", nil))
	if nextState != nil && len(nextState.(string)) > 0 {
		s.SetGameState(GameState(nextState.(string)))
	} else {
		s.SetGameState(GameStateRandom)
	}
}

// SetFightDescription sets the description of the fight.
func (s *Session) SetFightDescription(description string) {
	s.currentFight.Description = description
}

// GetFightRound returns the current round of the fight.
func (s *Session) GetFightRound() int {
	return s.currentFight.Round
}

// HadEvent checks if the given event already happened in this run.
func (s *Session) HadEvent(id string) bool {
	return lo.Contains(s.eventHistory, id)
}

// HadEvents checks if the given events already happened in this run.
func (s *Session) HadEvents(ids []string) bool {
	return lo.Every(s.eventHistory, ids)
}

// HadEventsAny checks if at least one of the given events already happened in this run.
func (s *Session) HadEventsAny(ids []string) bool {
	return lo.Some(s.eventHistory, ids)
}

// GetEventHistory returns the ordered list of all events encountered so far.
func (s *Session) GetEventHistory() []string {
	return s.eventHistory
}

func (s *Session) GetEventChoiceDescription(i int) string {
	event := s.GetEvent()
	if event == nil || i < 0 || i >= len(event.Choices) {
		return ""
	}

	if event.Choices[i].DescriptionFn == nil {
		return event.Choices[i].Description
	}

	res, err := event.Choices[i].DescriptionFn.Call(CreateContext("type_id", event.ID, "choice", i))
	if err != nil {
		s.logLuaError("DescriptionFn", event.ID, err)
		return event.Choices[i].Description
	}

	if res, ok := res.(string); ok {
		return res
	}

	s.logLuaError("DescriptionFn", event.ID, errors.New("didn't return a string"))
	return event.Choices[i].Description
}

//
// Merchant
//

// SetupMerchant sets up the merchant, which means generating a new face, text and initial wares.
func (s *Session) SetupMerchant() {
	s.merchant.Artifacts = nil
	s.merchant.Cards = nil
	s.merchant.Face = faces.Global.GenRand()
	s.merchant.Text = gen.GetRandom("merchant_lines")

	for i := 0; i < 3; i++ {
		s.AddMerchantArtifact()
		s.AddMerchantCard()
	}
}

// LeaveMerchant finishes the merchant state and lets the storyteller decide what to do next.
func (s *Session) LeaveMerchant() {
	s.SetGameState(GameStateRandom)
}

// GetMerchant return the merchant state.
func (s *Session) GetMerchant() MerchantState {
	return s.merchant
}

// GetMerchantGoldMax returns what the max cost of a artifact or card is that the merchant might offer.
func (s *Session) GetMerchantGoldMax() int {
	return 150 + s.stagesCleared*30
}

func (s *Session) PushRandomHistory(id string) {
	s.randomHistory = append([]string{id}, s.randomHistory...)
	if len(s.randomHistory) > 10 {
		s.randomHistory = s.randomHistory[:10]
	}
}

// GetRandomArtifact returns the type id of a random artifact with a price lower than the given value.
func (s *Session) GetRandomArtifact(maxGold int) string {
	possible := lo.Filter(lo.Values(s.resources.Artifacts), func(item *Artifact, index int) bool {
		return item.Price >= 0 && item.Price < maxGold
	})

	possibleNoDupes := lo.Filter(possible, func(item *Artifact, index int) bool {
		return !lo.Contains(s.randomHistory, item.ID)
	})

	if len(possible) > 0 {
		var chosen string
		if len(possibleNoDupes) > 0 {
			chosen = lo.Shuffle(possibleNoDupes)[0].ID
		} else {
			chosen = lo.Shuffle(possible)[0].ID
		}
		s.PushRandomHistory(chosen)
		return chosen
	}

	return ""
}

// GetRandomCard returns the type id of a random card with a price lower than the given value.
func (s *Session) GetRandomCard(maxGold int) string {
	possible := lo.Filter(lo.Values(s.resources.Cards), func(item *Card, index int) bool {
		return item.Price >= 0 && item.Price < maxGold
	})

	possibleNoDupes := lo.Filter(possible, func(item *Card, index int) bool {
		return !lo.Contains(s.randomHistory, item.ID)
	})

	if len(possible) > 0 {
		var chosen string
		if len(possibleNoDupes) > 0 {
			chosen = lo.Shuffle(possibleNoDupes)[0].ID
		} else {
			chosen = lo.Shuffle(possible)[0].ID
		}
		s.PushRandomHistory(chosen)
		return chosen
	}

	return ""
}

// AddMerchantArtifact adds another artifact to the wares of the merchant.
func (s *Session) AddMerchantArtifact() {
	if val := s.GetRandomArtifact(s.GetMerchantGoldMax()); len(val) > 0 {
		s.merchant.Artifacts = append(s.merchant.Artifacts, val)
	}
}

// AddMerchantCard adds another card to the wares of the merchant.
func (s *Session) AddMerchantCard() {
	if val := s.GetRandomCard(s.GetMerchantGoldMax()); len(val) > 0 {
		s.merchant.Cards = append(s.merchant.Cards, val)
	}
}

// PlayerBuyCard buys the card with the given type id. The card needs to be in the wares of the merchant.
func (s *Session) PlayerBuyCard(t string) bool {
	if !lo.Contains(s.merchant.Cards, t) {
		return false
	}

	card, _ := s.GetCard(t)

	if s.GetPlayer().Gold < card.Price {
		return false
	}

	s.UpdatePlayer(func(actor *Actor) bool {
		actor.Gold -= card.Price
		return true
	})

	firstFound := false
	s.merchant.Cards = lo.Filter(s.merchant.Cards, func(item string, index int) bool {
		if firstFound {
			return true
		}

		isType := item == t
		if isType {
			firstFound = true
			return false
		}

		return true
	})
	s.GiveCard(card.ID, PlayerActorID)
	return true
}

// PlayerBuyArtifact buys the artifact with the given type id. The artifact needs to be in the wares of the merchant.
func (s *Session) PlayerBuyArtifact(t string) bool {
	if !lo.Contains(s.merchant.Artifacts, t) {
		return false
	}

	art, _ := s.GetArtifact(t)

	if s.GetPlayer().Gold < art.Price {
		return false
	}

	s.UpdatePlayer(func(actor *Actor) bool {
		actor.Gold -= art.Price
		return true
	})

	firstFound := false
	s.merchant.Artifacts = lo.Filter(s.merchant.Artifacts, func(item string, index int) bool {
		if firstFound {
			return true
		}

		isType := item == t
		if isType {
			firstFound = true
			return false
		}

		return true
	})
	s.GiveArtifact(art.ID, PlayerActorID)
	return true
}

//
// StoryTeller
//

// ActiveTeller returns the active storyteller. The storyteller is responsible for deciding what enemies or events
// the player will encounter next.
func (s *Session) ActiveTeller() *StoryTeller {
	teller := lo.Filter(lo.Values(s.resources.StoryTeller), func(teller *StoryTeller, index int) bool {
		res, err := teller.Active(CreateContext("type_id", teller.ID))
		if err != nil {
			s.logLuaError("Active", teller.ID, err)
			return false
		}
		if val, ok := res.(float64); ok {
			return val > 0
		}
		return false
	})

	if len(teller) == 0 {
		s.log.Printf("No active teller found!")
		return nil
	}

	slices.SortFunc(teller, func(a, b *StoryTeller) bool {
		aOrder, _ := a.Active(CreateContext("type_id", a.ID))
		bOrder, _ := b.Active(CreateContext("type_id", b.ID))

		return aOrder.(float64) > bOrder.(float64)
	})

	return teller[0]
}

// LetTellerDecide lets the currently active storyteller decide what the next game state will be.
func (s *Session) LetTellerDecide() {
	active := s.ActiveTeller()

	if active == nil {
		s.log.Printf("No active teller found! Can't decide")
		return
	}

	res, err := active.Decide(CreateContext("type_id", active.ID))
	if err != nil {
		s.logLuaError("Decide", active.ID, err)
		return
	}

	if val, ok := res.(string); ok {
		s.SetGameState(GameState(val))
	} else {
		s.logLuaError("Decide", active.ID, errors.New("return wasn't a game state"))
	}
}

//
// Instances
//

// GetInstances returns all instances in the session.
func (s *Session) GetInstances() []string {
	return lo.Keys(s.instances)
}

// GetInstance returns an instance by guid. An instance is a CardInstance or ArtifactInstance.
func (s *Session) GetInstance(guid string) any {
	return s.instances[guid]
}

// TraverseArtifactsStatus traverses all artifacts and status effects in the session and calls the given functions
// for each instance. The instances are sorted by their order value. The order value is based on the order that is
// specified in the game data. This allows the game data to control the order of effects.
func (s *Session) TraverseArtifactsStatus(guids []string, artifact func(instance ArtifactInstance, artifact *Artifact), status func(instance StatusEffectInstance, statusEffect *StatusEffect)) {
	sort.SliceStable(guids, func(i, j int) bool {
		oa := ui.Max(s.GetArtifactOrder(guids[i]), s.GetStatusEffectOrder(guids[i]))
		ob := ui.Max(s.GetArtifactOrder(guids[j]), s.GetStatusEffectOrder(guids[j]))
		return oa > ob
	})

	for _, id := range guids {
		instance, ok := s.instances[id]
		if !ok {
			continue
		}

		switch instance := instance.(type) {
		case ArtifactInstance:
			// Fetch the backing definition of the type
			art, ok := s.resources.Artifacts[instance.TypeID]
			if !ok {
				continue
			}

			artifact(instance, art)
		case StatusEffectInstance:
			// Fetch the backing definition of the type
			se, ok := s.resources.StatusEffects[instance.TypeID]
			if !ok {
				continue
			}

			status(instance, se)
		}
	}
}

//
// Status Effect Functions
//

// GetStatusEffectOrder returns the order value of a status effect by guid.
func (s *Session) GetStatusEffectOrder(guid string) int {
	// Try as type id
	if e, ok := s.resources.StatusEffects[guid]; ok {
		return e.Order
	}

	instance, ok := s.instances[guid]
	if !ok {
		return 0
	}
	switch instance := instance.(type) {
	case StatusEffectInstance:
		if e, ok := s.resources.StatusEffects[instance.TypeID]; ok {
			return e.Order
		}
	}
	return 0
}

// GetStatusEffect returns status effect by guid.
func (s *Session) GetStatusEffect(guid string) *StatusEffect {
	// Try as type id
	if e, ok := s.resources.StatusEffects[guid]; ok {
		return e
	}

	instance, ok := s.instances[guid]
	if !ok {
		return nil
	}
	switch instance := instance.(type) {
	case StatusEffectInstance:
		if e, ok := s.resources.StatusEffects[instance.TypeID]; ok {
			return e
		}
	}
	return nil
}

// GetStatusEffectState returns the rendered state of the status effect.
func (s *Session) GetStatusEffectState(guid string) string {
	status := s.GetStatusEffect(guid)
	if status == nil {
		return ""
	}
	instance := s.GetStatusEffectInstance(guid)

	if status.State == nil {
		return status.Description
	}

	res, err := status.State.Call(CreateContext("type_id", status.ID, "guid", guid, "stacks", instance.Stacks, "owner", instance.Owner))
	if err != nil {
		s.logLuaError("State", instance.TypeID, err)
	}

	if res == nil {
		return status.Description
	}
	return res.(string)
}

// GetStatusEffectInstance returns status effect instance by guid.
func (s *Session) GetStatusEffectInstance(guid string) StatusEffectInstance {
	if val, ok := s.instances[guid].(StatusEffectInstance); ok {
		return val
	}
	return StatusEffectInstance{}
}

// RemoveAllStatusEffects clears all present status effects.
func (s *Session) RemoveAllStatusEffects() {
	var clean []string
	for guid, v := range s.instances {
		if _, ok := v.(StatusEffectInstance); ok {
			clean = append(clean, guid)
		}
	}

	for i := range clean {
		delete(s.instances, clean[i])
	}

	for guid, actor := range s.actors {
		actor.StatusEffects = NewStringSet()
		s.actors[guid] = actor
	}
}

// GiveStatusEffect gives the owner a status effect of a certain type. Status effects are singleton per actor,
// so if the actor already has the status effect the stacks will be increased.
func (s *Session) GiveStatusEffect(typeId string, owner string, stacks int) string {
	if len(owner) == 0 {
		s.log.Println("Error: trying to give status effect without owner!")
		return ""
	}

	status := s.resources.StatusEffects[typeId]
	if status == nil {
		return ""
	}

	if _, ok := s.actors[owner]; !ok {
		return ""
	}

	// TODO: This should always be either 0 or 1 len, so the logic down below is a bit meh.
	same := lo.Filter(s.actors[owner].StatusEffects.ToSlice(), func(guid string, index int) bool {
		instance, ok := s.instances[guid].(StatusEffectInstance)
		if !ok {
			return false
		}

		return instance.TypeID == typeId
	})

	if len(same) > 1 {
		panic("Error: status effect duplicate!")
	}

	// If it can't stack we delete all existing instances
	if !status.CanStack {
		for i := range same {
			s.RemoveStatusEffect(same[i])
		}
	} else if len(same) > 0 {
		// Increase stack and re-set rounds left
		instance := s.instances[same[0]].(StatusEffectInstance)
		instance.Stacks += stacks
		instance.RoundsLeft = status.Rounds
		s.instances[same[0]] = instance

		if _, err := status.Callbacks[CallbackOnStatusStack].Call(CreateContext("type_id", typeId, "guid", same[0], "owner", owner, "stacks", instance.Stacks)); err != nil {
			s.logLuaError(CallbackOnStatusStack, instance.TypeID, err)
		}

		return instance.GUID
	}

	instance := StatusEffectInstance{
		TypeID:       typeId,
		GUID:         NewGuid("STATUS"),
		Owner:        owner,
		RoundsLeft:   status.Rounds,
		Stacks:       stacks,
		RoundEntered: s.currentFight.Round,
	}
	s.instances[instance.GUID] = instance
	s.actors[owner].StatusEffects.Add(instance.GUID)

	// Call OnStatusAdd callback for the new instance
	_, _ = status.Callbacks[CallbackOnStatusAdd].Call(CreateContext("type_id", typeId, "guid", instance.GUID))

	return instance.GUID
}

// RemoveStatusEffect removes a status effect by guid.
func (s *Session) RemoveStatusEffect(guid string) {
	instance, ok := s.instances[guid].(StatusEffectInstance)
	if !ok {
		return
	}

	if _, err := s.resources.StatusEffects[instance.TypeID].Callbacks[CallbackOnStatusRemove].Call(CreateContext("type_id", instance.TypeID, "guid", guid, "owner", instance.Owner)); err != nil {
		s.logLuaError(CallbackOnStatusRemove, instance.TypeID, err)
	}
	if actor, ok := s.actors[instance.Owner]; ok {
		actor.StatusEffects.Remove(instance.GUID)
	}
	delete(s.instances, guid)
}

// GetActorStatusEffects returns the guids of all the status effects a certain actor owns.
func (s *Session) GetActorStatusEffects(guid string) []string {
	if actor, ok := s.actors[guid]; ok {
		return actor.StatusEffects.ToSlice()
	}

	return []string{}
}

// AddStatusEffectStacks increases the stacks of a certain status effect by guid.
func (s *Session) AddStatusEffectStacks(guid string, stacks int) {
	instance, ok := s.instances[guid].(StatusEffectInstance)
	if !ok {
		return
	}

	instance.Stacks += stacks
	if instance.Stacks <= 0 {
		s.RemoveStatusEffect(guid)
	} else {
		s.instances[guid] = instance
	}
}

// SetStatusEffectStacks sets the stacks of a certain status effect by guid.
func (s *Session) SetStatusEffectStacks(guid string, stacks int) {
	instance, ok := s.instances[guid].(StatusEffectInstance)
	if !ok {
		return
	}

	instance.Stacks = stacks
	if instance.Stacks <= 0 {
		s.RemoveStatusEffect(guid)
	} else {
		s.instances[guid] = instance
	}
}

//
// Artifact Functions
//

// GetArtifactOrder returns the order value of a certain artifact by guid.
func (s *Session) GetArtifactOrder(guid string) int {
	artInstance, ok := s.instances[guid]
	if !ok {
		return 0
	}
	switch artInstance := artInstance.(type) {
	case ArtifactInstance:
		if art, ok := s.resources.Artifacts[artInstance.TypeID]; ok {
			return art.Order
		}
	}
	return 0
}

// GetArtifacts returns all artifacts owned by a actor.
func (s *Session) GetArtifacts(owner string) []string {
	guids := s.actors[owner].Artifacts.ToSlice()
	sort.Strings(guids)
	return guids
}

// GetArtifact returns an artifact, and instance by guid or type id. If a type id is given
// only the Artifact will be returned.
func (s *Session) GetArtifact(guid string) (*Artifact, ArtifactInstance) {
	// check if guid is actually typeId
	if val, ok := s.resources.Artifacts[guid]; ok {
		return val, ArtifactInstance{}
	}

	artInstance, ok := s.instances[guid]
	if !ok {
		return nil, ArtifactInstance{}
	}
	switch artInstance := artInstance.(type) {
	case ArtifactInstance:
		if art, ok := s.resources.Artifacts[artInstance.TypeID]; ok {
			return art, artInstance
		}
	}
	return nil, ArtifactInstance{}
}

// GiveArtifact gives an artifact to an actor.
func (s *Session) GiveArtifact(typeId string, owner string) string {
	if _, ok := s.resources.Artifacts[typeId]; !ok {
		return ""
	}

	instance := ArtifactInstance{
		TypeID: typeId,
		GUID:   NewGuid("ARTIFACT"),
		Owner:  owner,
	}
	s.instances[instance.GUID] = instance
	s.actors[owner].Artifacts.Add(instance.GUID)

	// Call OnPickUp callback for the new instance
	if _, err := s.resources.Artifacts[typeId].Callbacks[CallbackOnPickUp].Call(CreateContext("type_id", typeId, "guid", instance.GUID, "owner", owner)); err != nil {
		s.logLuaError(CallbackOnPickUp, instance.TypeID, err)
	}

	s.PushState(map[StateEvent]any{
		StateEventArtifactAdded: StateEventArtifactAddedData{
			Owner:  owner,
			TypeID: typeId,
			GUID:   instance.GUID,
		},
	})

	return instance.GUID
}

// RemoveArtifact removes an artifact by guid.
func (s *Session) RemoveArtifact(guid string) {
	instance := s.instances[guid].(ArtifactInstance)
	if _, err := s.resources.Artifacts[instance.TypeID].Callbacks[CallbackOnRemove].Call(CreateContext("type_id", instance.TypeID, "guid", guid, "owner", instance.Owner)); err != nil {
		s.logLuaError(CallbackOnRemove, instance.TypeID, err)
	}
	s.actors[instance.Owner].Artifacts.Remove(instance.GUID)
	delete(s.instances, guid)

	s.PushState(map[StateEvent]any{
		StateEventArtifactRemoved: StateEventArtifactRemovedData{
			Owner:  instance.Owner,
			TypeID: instance.TypeID,
			GUID:   instance.GUID,
		},
	})
}

//
// Card Functions
//

// GetCard returns a card, and instance by guid or type id. If a type id is given
// only the Card will be returned. If the card is not found, nil is returned.
func (s *Session) GetCard(guid string) (*Card, CardInstance) {
	// check if guid is actually typeId
	if val, ok := s.resources.Cards[guid]; ok {
		return val, CardInstance{}
	}

	cardInstance, ok := s.instances[guid]
	if !ok {
		return nil, CardInstance{}
	}
	switch cardInstance := cardInstance.(type) {
	case CardInstance:
		if card, ok := s.resources.Cards[cardInstance.TypeID]; ok {
			return card, cardInstance
		}
	}
	return nil, CardInstance{}
}

// GiveCard gives a card to an actor. Returns the guid of the new card.
func (s *Session) GiveCard(typeId string, owner string) string {
	if _, ok := s.resources.Cards[typeId]; !ok {
		return ""
	}

	instance := CardInstance{
		TypeID: typeId,
		GUID:   NewGuid("CARD"),
		Owner:  owner,
	}
	s.instances[instance.GUID] = instance
	s.actors[owner].Cards.Add(instance.GUID)

	s.PushState(map[StateEvent]any{
		StateEventCardAdded: StateEventCardAddedData{
			Owner:  owner,
			TypeID: typeId,
			GUID:   instance.GUID,
		},
	})

	return instance.GUID
}

// RemoveCard removes a card by guid.
func (s *Session) RemoveCard(guid string) {
	instance := s.instances[guid].(CardInstance)
	s.actors[instance.Owner].Cards.Remove(instance.GUID)
	delete(s.instances, guid)

	s.PushState(map[StateEvent]any{
		StateEventCardRemoved: StateEventCardRemovedData{
			Owner:  instance.Owner,
			TypeID: instance.TypeID,
			GUID:   instance.GUID,
		},
	})
}

// CastCard calls the OnCast callback for a card, casting it.
func (s *Session) CastCard(guid string, target string) bool {
	if card, instance := s.GetCard(guid); card != nil {
		res, err := card.Callbacks[CallbackOnCast].Call(CreateContext("type_id", card.ID, "guid", guid, "caster", instance.Owner, "target", target, "level", instance.Level))
		if err != nil {
			s.logLuaError(CallbackOnCast, instance.TypeID, err)
		}
		if val, ok := res.(bool); ok {
			if !val {
				return false
			}

			TriggerCallbackSimple(s, CallbackOnActorDidCast, TriggerAll, EmptyContext, CreateContext("type_id", card.ID, "guid", guid, "caster", instance.Owner, "target", target, "level", instance.Level, "tags", card.Tags))
			return true
		}
	}
	return true
}

// GetCards returns all cards owned by a actor.
func (s *Session) GetCards(owner string) []string {
	guids := s.actors[owner].Cards.ToSlice()
	sort.Strings(guids)
	return guids
}

// GetCardState returns the state of a card.
func (s *Session) GetCardState(guid string) string {
	card, instance := s.GetCard(guid)
	if card == nil {
		return ""
	}

	if card.State == nil {
		return card.Description
	}

	res, err := card.State.Call(CreateContext("type_id", card.ID, "guid", guid, "level", instance.Level, "owner", instance.Owner))
	if err != nil {
		s.logLuaError("State", instance.TypeID, err)
	}

	if res == nil {
		return card.Description
	}
	return res.(string)
}

// PlayerCastHand casts a card from the players hand.
func (s *Session) PlayerCastHand(i int, target string) error {
	if i >= len(s.currentFight.Hand) {
		return errors.New("hand empty")
	}

	cardId := s.currentFight.Hand[i]

	// Only cast a card if castable and points are available and subtract them.
	card, _ := s.GetCard(cardId)
	if card != nil {
		if !card.Callbacks[CallbackOnCast].Present() {
			return errors.New("card is not castable")
		}

		if s.currentFight.CurrentPoints < card.PointCost {
			return errors.New("not enough points")
		}

		s.currentFight.CurrentPoints -= card.PointCost
	} else {
		return errors.New("card not exists")
	}

	// Remove from hand.
	s.currentFight.Hand = lo.Reject(s.currentFight.Hand, func(item string, index int) bool {
		return index == i
	})

	// Cast and exhaust if needed.
	didCast := s.CastCard(cardId, target)
	if didCast {
		if card.DoesExhaust {
			s.currentFight.Exhausted = append(s.currentFight.Exhausted, cardId)
		} else if card.DoesConsume {
			s.RemoveCard(cardId)
		} else {
			s.currentFight.Used = append(s.currentFight.Used, cardId)
		}
	}

	s.FinishFight()

	return nil
}

// PlayerDrawCard draws a card from the deck.
func (s *Session) PlayerDrawCard(amount int) {
	for i := 0; i < amount; i++ {
		// Shuffle used back in
		if len(s.currentFight.Deck) == 0 && len(s.currentFight.Used) > 0 {
			s.currentFight.Deck = lo.Shuffle(s.currentFight.Used)
			s.currentFight.Used = []string{}
		}

		// If nothing left don't draw
		if len(s.currentFight.Deck) == 0 {
			break
		}

		s.currentFight.Hand = append(s.currentFight.Hand, s.currentFight.Deck[0])
		s.currentFight.Deck = lo.Drop(s.currentFight.Deck, 1)
	}
}

// PlayerGiveActionPoints gives the player action points.
func (s *Session) PlayerGiveActionPoints(amount int) {
	s.currentFight.CurrentPoints += amount
}

// BuyUpgradeCard upgrades a card by its GUID.
func (s *Session) BuyUpgradeCard(guid string) bool {
	card, instance := s.GetCard(guid)
	if instance.IsNone() || card.MaxLevel == 0 || instance.Level == card.MaxLevel {
		return false
	}

	if s.GetPlayer().Gold < DefaultUpgradeCost {
		return false
	}
	s.UpdatePlayer(func(actor *Actor) bool {
		actor.Gold -= DefaultUpgradeCost
		return true
	})

	instance.Level += 1
	s.instances[guid] = instance
	return true
}

// BuyRemoveCard removes a card by its GUID.
func (s *Session) BuyRemoveCard(guid string) bool {
	_, instance := s.GetCard(guid)
	if instance.IsNone() {
		return false
	}

	if s.GetPlayer().Gold < DefaultRemoveCost {
		return false
	}
	s.UpdatePlayer(func(actor *Actor) bool {
		actor.Gold -= DefaultUpgradeCost
		return true
	})

	s.RemoveCard(guid)
	return true
}

// UpgradeCard upgrades a card by its GUID.
func (s *Session) UpgradeCard(guid string) bool {
	card, instance := s.GetCard(guid)
	if instance.IsNone() || card.MaxLevel == 0 || instance.Level == card.MaxLevel {
		return false
	}

	instance.Level += 1
	s.instances[guid] = instance
	return true
}

// UpgradeRandomCard upgrades a random card of the given owner.
func (s *Session) UpgradeRandomCard(owner string) bool {
	upgradeable := lo.Filter(s.GetActor(owner).Cards.ToSlice(), func(item string, index int) bool {
		card, instance := s.GetCard(item)
		if instance.IsNone() {
			return false
		}
		return instance.Level != card.MaxLevel
	})

	if len(upgradeable) == 0 {
		return false
	}

	return s.UpgradeCard(lo.Shuffle(upgradeable)[0])
}

//
// Damage & Heal Function
//

// DealDamage deals damage to a target. If flat is true it will not trigger any callbacks which modify the damage.
func (s *Session) DealDamage(source string, target string, damage int, flat bool) int {
	if _, ok := s.actors[source]; !ok {
		return 0
	}

	val, ok := s.actors[target]
	if !ok {
		return 0
	}

	// If not flat we will modify the damage based on the OnDamageCalc callbacks.
	if !flat {
		reducer := func(cur float64, val float64) float64 {
			return val
		}
		damage = int(TriggerCallbackReduce[float64](
			s,
			CallbackOnDamageCalc,
			TriggerAll,
			reducer,
			float64(damage),
			"damage",
			CreateContext("source", source, "target", target, "damage", damage)),
		)
	}

	if source == PlayerActorID {
		s.Log(LogTypeSuccess, fmt.Sprintf("You hit the enemy for %d damage", damage))
	} else if target == PlayerActorID {
		s.Log(LogTypeDanger, fmt.Sprintf("You took %d damage", damage))
	} else {
		s.Log(LogTypeSuccess, fmt.Sprintf("%s took %d damage", val.Name, damage))
	}

	// Negative damage aka heal is not allowed!
	if damage < 0 {
		return 0
	}

	// Trigger OnDamage callbacks
	TriggerCallbackSimple(s, CallbackOnDamage, TriggerAll, CreateContext("source", source, "target", target, "damage", damage))

	// Re-fetch actor in case the OnDamage callback triggered some kind of damage or healing.
	val = s.actors[target]

	hpLeft := lo.Clamp(val.HP-damage, 0, val.MaxHP)

	// Remove dead non-player actor
	if target != PlayerActorID && hpLeft == 0 {
		s.PushState(map[StateEvent]any{
			StateEventDeath: StateEventDeathData{
				Source: source,
				Target: target,
				Damage: damage,
			},
		})
		s.Log(LogTypeSuccess, fmt.Sprintf("%s died and dropped %d gold!", val.Name, val.Gold))
		s.GivePlayerGold(val.Gold)

		// Trigger OnActorDie callbacks
		TriggerCallbackSimple(s, CallbackOnActorDie, TriggerAll, CreateContext("source", source, "target", target, "damage", damage))

		s.RemoveActor(target)
	} else {
		s.PushState(map[StateEvent]any{
			StateEventDamage: StateEventDamageData{
				Source: source,
				Target: target,
				Damage: damage,
			},
		})
		s.UpdateActor(target, func(actor *Actor) bool {
			actor.HP = hpLeft
			return true
		})
		if target == PlayerActorID && s.GetPlayer().HP == 0 {
			s.SetGameState(GameStateGameOver)
		}
	}

	return damage
}

// SimulateDealDamage will simulate damage to a target. If flat is true it will not trigger any callbacks which modify the damage.
func (s *Session) SimulateDealDamage(source string, target string, damage int, flat bool) int {
	if _, ok := s.actors[source]; !ok {
		return 0
	}

	_, ok := s.actors[target]
	if !ok {
		return 0
	}

	// If not flat we will modify the damage based on the OnDamageCalc callbacks.
	if !flat {
		reducer := func(cur float64, val float64) float64 {
			return val
		}
		damage = int(TriggerCallbackReduce[float64](
			s,
			CallbackOnDamageCalc,
			TriggerAll,
			reducer,
			float64(damage),
			"damage",
			CreateContext("source", source, "target", target, "damage", damage, "simulated", true)),
		)
	}

	// Negative damage aka heal is not allowed!
	if damage < 0 {
		return 0
	}

	return damage
}

// DealDamageMulti will deal damage to multiple targets and return the amount of damage dealt to each target.
// If flat is true it will not trigger any OnDamageCalc callbacks which modify the damage.
func (s *Session) DealDamageMulti(source string, targets []string, damage int, flat bool) []int {
	return lo.Map(targets, func(guid string, index int) int {
		return s.DealDamage(source, guid, damage, flat)
	})
}

// Heal will heal the target for the given amount from source to target and return the amount healed.
// If flat is true it will not trigger any OnHealCalc callbacks which modify the heal.
func (s *Session) Heal(source string, target string, heal int, flat bool) int {
	if val, ok := s.actors[target]; ok {
		if !flat {
			s.TraverseArtifactsStatus(lo.Flatten([][]string{
				s.GetActor(source).Artifacts.ToSlice(),
				s.GetActor(target).StatusEffects.ToSlice(),
				s.GetActor(source).StatusEffects.ToSlice(),
			}),
				func(instance ArtifactInstance, art *Artifact) {
					res, err := art.Callbacks[CallbackOnHealCalc].Call(CreateContext("type_id", art.ID, "guid", instance.GUID, "source", source, "target", target, "owner", instance.Owner, "heal", heal))
					if err != nil {
						s.logLuaError(CallbackOnHealCalc, instance.TypeID, err)
					} else if res != nil {
						if newHeal, ok := res.(float64); ok {
							heal = int(newHeal)
						}
					}
				},
				func(instance StatusEffectInstance, se *StatusEffect) {
					res, err := se.Callbacks[CallbackOnHealCalc].Call(CreateContext("type_id", se.ID, "guid", instance.GUID, "source", source, "target", target, "owner", instance.Owner, "stacks", instance.Stacks, "heal", heal))
					if err != nil {
						s.logLuaError(CallbackOnHealCalc, instance.TypeID, err)
					} else if res != nil {
						if newHeal, ok := res.(float64); ok {
							heal = int(newHeal)
						}
					}
				},
			)
		}

		if target == PlayerActorID {
			s.Log(LogTypeSuccess, fmt.Sprintf("You healed %d damage", heal))
		} else {
			s.Log(LogTypeDanger, fmt.Sprintf("%s healed %d damage", val.Name, heal))
		}

		// Negative heal aka damage is not allowed!
		if heal < 0 {
			heal = 0
		}

		s.UpdateActor(target, func(actor *Actor) bool {
			actor.HP = lo.Clamp(val.HP+heal, 0, val.MaxHP)
			return true
		})

		return heal
	}
	return 0
}

//
// Actor Functions
//

// GetPlayer returns the player.
func (s *Session) GetPlayer() Actor {
	return s.actors[PlayerActorID]
}

// UpdatePlayer updates the player using a update function.
func (s *Session) UpdatePlayer(update func(actor *Actor) bool) {
	s.UpdateActor(PlayerActorID, update)
}

// GetActors returns all actors.
func (s *Session) GetActors() []string {
	return lo.Keys(s.actors)
}

// GetActor returns an actor.
func (s *Session) GetActor(id string) Actor {
	if val, ok := s.actors[id]; ok {
		return val
	}
	return NewActor("")
}

// UpdateActor updates an actor. If the update function returns true the actor will be updated.
func (s *Session) UpdateActor(id string, update func(actor *Actor) bool) {
	actor := s.GetActor(id)
	if update(&actor) {
		s.actors[id] = actor
	}
}

// GetActorIntend returns the intend of an actor.
func (s *Session) GetActorIntend(guid string) string {
	if enemy := s.GetEnemy(s.actors[guid].TypeID); enemy != nil {
		res, err := enemy.Intend.Call(CreateContext("type_id", enemy.ID, "guid", guid, "round", s.currentFight.Round))
		if err != nil {
			s.logLuaError("Intend", enemy.ID, err)
		} else if res, ok := res.(string); ok {
			return res
		}
	}
	return ""
}

// ActorAddMaxHP adds max hp to an actor.
func (s *Session) ActorAddMaxHP(id string, val int) {
	s.UpdateActor(id, func(actor *Actor) bool {
		actor.MaxHP += val
		return true
	})
}

// ActorAddHP adds HP to an actor.
func (s *Session) ActorAddHP(id string, val int) {
	s.UpdateActor(id, func(actor *Actor) bool {
		actor.HP += val
		return true
	})
}

// AddActor adds an actor to the session.
func (s *Session) AddActor(actor Actor) {
	s.actors[actor.GUID] = actor
}

// AddActorFromEnemy adds an actor to the session from an enemy base.
func (s *Session) AddActorFromEnemy(id string) string {
	if base, ok := s.resources.Enemies[id]; ok {
		actor := NewActor(NewGuid(id))

		actor.TypeID = id
		actor.Name = base.Name
		actor.Description = base.Description
		actor.HP = base.InitialHP
		actor.MaxHP = base.MaxHP

		// Its important we add the actor before any callbacks so that it's instance is available
		// to add cards etc. to!
		s.AddActor(actor)

		if _, err := base.Callbacks[CallbackOnInit].Call(CreateContext("type_id", id, "guid", actor.GUID)); err != nil {
			s.logLuaError(CallbackOnInit, actor.TypeID, err)
		}

		return actor.GUID
	}

	return ""
}

// RemoveActor removes an actor from the session.
func (s *Session) RemoveActor(id string) {
	var deleteInstances []string

	for _, val := range s.instances {
		switch val := val.(type) {
		case CardInstance:
			if val.Owner == id {
				deleteInstances = append(deleteInstances, id)
			}
		case ArtifactInstance:
			if val.Owner == id {
				deleteInstances = append(deleteInstances, id)
			}
		}
	}

	// Clear actor owned items
	for _, k := range deleteInstances {
		delete(s.instances, k)
	}

	delete(s.actors, id)
}

// RemoveNonPlayer removes all actors that are not the player.
func (s *Session) RemoveNonPlayer() {
	var deleteActors []string
	for _, a := range s.actors {
		if a.GUID != PlayerActorID {
			deleteActors = append(deleteActors, a.GUID)
		}
	}

	for _, k := range deleteActors {
		delete(s.actors, k)
	}
}

// GetOpponentCount returns the number of opponents from the given viewpoint.
func (s *Session) GetOpponentCount(viewpoint string) int {
	switch viewpoint {
	// From the viewpoint of the player we can have multiple enemies.
	case PlayerActorID:
		return len(lo.Filter(lo.Keys(s.actors), func(item string, index int) bool {
			return item != PlayerActorID
		}))
	// From the viewpoint of an enemy we only have the player as enemy.
	default:
		return 1
	}
}

// GetOpponentByIndex returns the opponent at the given index from the given viewpoint.
func (s *Session) GetOpponentByIndex(viewpoint string, i int) Actor {
	switch viewpoint {
	// From the viewpoint of the player we can have multiple enemies.
	case PlayerActorID:
		if len(s.actors) <= 1 {
			return Actor{}
		}

		ids := lo.Filter(lo.Keys(s.actors), func(guid string, index int) bool {
			return guid != PlayerActorID
		})
		sort.Strings(ids)
		if i < 0 || i >= len(ids) {
			return Actor{}
		}

		return s.actors[ids[i]]
	// From the viewpoint of an enemy we only have the player as enemy.
	default:
		return s.actors[PlayerActorID]
	}
}

// GetOpponents returns the opponents from the given viewpoint.
func (s *Session) GetOpponents(viewpoint string) []Actor {
	return lo.Map(s.GetOpponentGUIDs(viewpoint), func(guid string, index int) Actor {
		return s.actors[guid]
	})
}

// GetOpponentGUIDs returns the guids of the opponents from the given viewpoint.
func (s *Session) GetOpponentGUIDs(viewpoint string) []string {
	switch viewpoint {
	// From the viewpoint of the player we can have multiple enemies.
	case PlayerActorID:
		guids := lo.Filter(lo.Keys(s.actors), func(guid string, index int) bool {
			return guid != PlayerActorID
		})
		sort.Strings(guids)
		return guids
	// From the viewpoint of an enemy we only have the player as enemy.
	default:
		return []string{PlayerActorID}
	}
}

// GetEnemy returns the enemy with the given type id.
func (s *Session) GetEnemy(typeId string) *Enemy {
	return s.resources.Enemies[typeId]
}

//
// Gold
//

// GivePlayerGold gives the player the given amount of gold.
func (s *Session) GivePlayerGold(amount int) {
	if amount <= 0 {
		return
	}

	s.UpdatePlayer(func(actor *Actor) bool {
		actor.Gold += amount
		s.PushState(map[StateEvent]any{
			StateEventMoney: StateEventMoneyData{
				Target: PlayerActorID,
				Money:  amount,
			},
		})
		return true
	})
}

//
// Hooks
//

// AddHook adds a hook to the session.
func (s *Session) AddHook(hook Hook, callback func()) {
	s.hooks[hook] = append(s.hooks[hook], callback)
}

// TriggerHooks triggers all hooks of a certain type.
func (s *Session) TriggerHooks(hook Hook) {
	for _, callback := range s.hooks[hook] {
		callback()
	}
	s.hooks[hook] = []func(){}
}

//
// Misc Functions
//

// Log adds a log entry to the session.
func (s *Session) Log(t LogType, msg string) {
	s.Logs = append(s.Logs, LogEntry{
		Time:    time.Now(),
		Type:    t,
		Message: msg,
	})
}

// Fetch retrieves a value from the session context.
func (s *Session) Fetch(key string) any {
	return s.ctxData[key]
}

// Store stores a value in the session context.
func (s *Session) Store(key string, value any) {
	s.ctxData[key] = value
}

// ToSVG creates an SVG representation from the internal state. The returned string is the d2
// representation of the SVG (https://d2lang.com/).
func (s *Session) ToSVG() ([]byte, string, error) {
	diag := `
direction: right

resources: Lua Defined Resources {
  cards: Cards
  artifacts: Artifacts
  status: Status Effects
}

instances: Instances {
}

actors: Actors {
}

`
	for k, v := range s.actors {
		diag += fmt.Sprintf(`
actors.%s: {
  text: ||txt
%s
||
}

`, k, fmt.Sprintf("NAME = %s\nHP = %d / %d", v.Name, v.HP, v.MaxHP))

		lo.ForEach(v.Cards.ToSlice(), func(item string, index int) {
			diag += fmt.Sprintf("actors.%s -> instances.%s\n", k, item)
		})
		lo.ForEach(v.Artifacts.ToSlice(), func(item string, index int) {
			diag += fmt.Sprintf("actors.%s -> instances.%s\n", k, item)
		})
		lo.ForEach(v.StatusEffects.ToSlice(), func(item string, index int) {
			diag += fmt.Sprintf("actors.%s -> instances.%s\n", k, item)
		})
	}

	for i := range s.instances {
		switch inst := s.instances[i].(type) {
		case ArtifactInstance:
			diag += fmt.Sprintf("instances.%s -> %s\n", inst.GUID, "resources.artifacts."+inst.TypeID+": TypeId {style.animated: true}")
		case CardInstance:
			diag += fmt.Sprintf("instances.%s { \ntext: ||txt\n%s\n||\n}\n", inst.GUID, fmt.Sprintf("Level = %d", inst.Level))
			diag += fmt.Sprintf("instances.%s -> %s\n", inst.GUID, "resources.cards."+inst.TypeID+": TypeId {style.animated: true}")
		case StatusEffectInstance:
			diag += fmt.Sprintf("instances.%s { \ntext: ||txt\n%s\n||\n}\n", inst.GUID, fmt.Sprintf("Stacks = %d\nRounds Left = %d", inst.Stacks, inst.RoundsLeft))
			diag += fmt.Sprintf("instances.%s -> %s\n", inst.GUID, "resources.status."+inst.TypeID+": TypeId {style.animated: true}")
		}
	}

	ruler, _ := textmeasure.NewRuler()
	defaultLayout := func(ctx context.Context, g *d2graph.Graph) error {
		return d2dagrelayout.Layout(ctx, g, nil)
		//return d2elklayout.Layout(ctx, g, nil)
	}
	diagram, _, err := d2lib.Compile(context.Background(), diag, &d2lib.CompileOptions{
		Layout: defaultLayout,
		Ruler:  ruler,
	})
	if err != nil {
		return nil, diag, err
	}

	out, err := d2svg.Render(diagram, &d2svg.RenderOpts{
		Pad:     d2svg.DEFAULT_PADDING * 2,
		Center:  true,
		ThemeID: d2themescatalog.TerminalGrayscale.ID,
	})
	if err != nil {
		return nil, diag, err
	}

	return out, diag, nil
}
