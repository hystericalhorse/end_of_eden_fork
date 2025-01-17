package gameview

import (
	"fmt"
	"github.com/BigJk/end_of_eden/game"
	"github.com/BigJk/end_of_eden/system/audio"
	"github.com/BigJk/end_of_eden/ui"
	"github.com/BigJk/end_of_eden/ui/components"
	"github.com/BigJk/end_of_eden/ui/menus/carousel"
	"github.com/BigJk/end_of_eden/ui/menus/eventview"
	"github.com/BigJk/end_of_eden/ui/menus/gameover"
	"github.com/BigJk/end_of_eden/ui/menus/merchant"
	"github.com/BigJk/end_of_eden/ui/menus/overview"
	"github.com/BigJk/end_of_eden/ui/root"
	"github.com/BigJk/end_of_eden/ui/style"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/samber/lo"
	"strings"
)

const (
	ZoneCard          = "card_"
	ZoneEnemy         = "enemy_"
	ZoneEndTurn       = "end_turn"
	ZonePlayerInspect = "player_inspect"
)

type Model struct {
	ui.MenuBase

	zones               *zone.Manager
	parent              tea.Model
	selectedCard        int
	selectedOpponent    int
	inOpponentSelection bool
	inEnemyView         bool
	inPlayerView        bool
	animations          []tea.Model
	ctrlDown            bool

	lastGameState game.GameState
	lastEvent     string

	event    tea.Model
	merchant tea.Model

	Session           *game.Session
	Start             game.StateCheckpointMarker
	BeforeStateSwitch game.StateCheckpointMarker
}

func New(parent tea.Model, zones *zone.Manager, session *game.Session) Model {
	session.Log(game.LogTypeSuccess, "Game started! Good luck...")

	return Model{
		zones:    zones,
		parent:   parent,
		event:    eventview.New(zones, session),
		merchant: merchant.New(zones, session),

		Session:           session,
		Start:             session.MarkState(),
		BeforeStateSwitch: session.MarkState(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if cmd := root.CheckLuaErrors(m.zones, m.Session); cmd != nil {
		return m, cmd
	}

	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	//
	// Keyboard
	//
	case tea.KeyMsg:
		if len(m.animations) == 0 {
			switch msg.Type {
			case tea.KeyEnter:
				switch m.Session.GetGameState() {
				// Cast a card
				case game.GameStateFight:
					if m.selectedCard >= len(m.Session.GetFight().Hand) {
						m.selectedCard = 0
					}

					m = m.tryCast()
				}
			case tea.KeyEscape:
				cmds = append(cmds, root.TooltipClear())

				// Switch to menu
				if m.inOpponentSelection || m.inEnemyView || m.inPlayerView {
					m.inOpponentSelection = false
					m.inEnemyView = false
					m.inPlayerView = false
				} else {
					return overview.New(m, m.zones, m.Session), tea.Batch(cmds...)
				}
			case tea.KeyTab:
				switch m.Session.GetGameState() {
				// Select a card or opponent
				case game.GameStateFight:
					if len(m.Session.GetFight().Hand) > 0 {
						if m.inOpponentSelection {
							m.selectedOpponent = (m.selectedOpponent + 1) % m.Session.GetOpponentCount(game.PlayerActorID)
						} else {
							m.selectedCard = (m.selectedCard + 1) % len(m.Session.GetFight().Hand)
						}
					}
				}
			case tea.KeySpace:
				switch m.Session.GetGameState() {
				// End turn
				case game.GameStateFight:
					m = m.finishTurn()
				}
			case tea.KeyLeft:
				m.selectedCard = lo.Clamp(m.selectedCard-1, 0, len(m.Session.GetFight().Hand)-1)
			case tea.KeyRight:
				m.selectedCard = lo.Clamp(m.selectedCard+1, 0, len(m.Session.GetFight().Hand)-1)
			case tea.KeyCtrlDown:
				m.ctrlDown = true
			case tea.KeyCtrlU:
				m.ctrlDown = false
			}

			// Show tooltip
			if msg.String() == "x" {
				for i := 0; i < m.Session.GetOpponentCount(game.PlayerActorID); i++ {
					if m.zones.Get(fmt.Sprintf("%s%d", ZoneEnemy, i)).InBounds(m.LastMouse) {
						cmds = append(cmds, root.TooltipCreate(root.Tooltip{
							ID:      "ENEMY",
							Content: m.fightEnemyInspectTooltipView(),
							X:       m.LastMouse.X,
							Y:       m.LastMouse.Y,
						}))
					}
				}
			}
		}
	//
	// Mouse
	//
	case tea.MouseMsg:
		m.LastMouse = msg

		if len(m.animations) == 0 {
			if msg.Type == tea.MouseLeft {
				cmds = append(cmds, root.TooltipClear())

				switch m.Session.GetGameState() {
				case game.GameStateFight:
					if m.zones.Get(ZoneEndTurn).InBounds(msg) {
						m = m.finishTurn()
					} else if m.zones.Get(ZonePlayerInspect).InBounds(msg) {
						m.inPlayerView = true
					}
				}
			}

			if msg.Type == tea.MouseLeft || msg.Type == tea.MouseMotion {
				switch m.Session.GetGameState() {
				case game.GameStateFight:
					if m.inOpponentSelection {
						for i := 0; i < m.Session.GetOpponentCount(game.PlayerActorID); i++ {
							if m.zones.Get(fmt.Sprintf("%s%d", ZoneEnemy, i)).InBounds(msg) {
								if msg.Type == tea.MouseLeft && m.selectedOpponent == i {
									m = m.tryCast()
								}
							}
						}
					} else {
						onCard := false
						for i := 0; i < len(m.Session.GetFight().Hand); i++ {
							if m.zones.Get(fmt.Sprintf("%s%d", ZoneCard, i)).InBounds(msg) {
								onCard = true
								if msg.Type == tea.MouseLeft && m.selectedCard == i {
									m = m.tryCast()
								} else {
									m.selectedCard = i
								}
							}
						}

						if !onCard && msg.Type == tea.MouseMotion {
							m.selectedCard = -1
						}

						if !m.inOpponentSelection && msg.Type == tea.MouseLeft {
							for i := 0; i < m.Session.GetOpponentCount(game.PlayerActorID); i++ {
								if m.zones.Get(fmt.Sprintf("%s%d", ZoneEnemy, i)).InBounds(msg) {
									m.selectedOpponent = i
									m.inEnemyView = true
								}
							}
						}
					}

				}
			}
		}
	//
	// Window Size
	//
	case tea.WindowSizeMsg:
		m.Size = msg

		// Always pass size events
		m.event, _ = m.event.Update(msg)
		m.merchant, _ = m.merchant.Update(msg)

		for i := range m.animations {
			m.animations[i], _ = m.animations[i].Update(tea.WindowSizeMsg{Width: m.Size.Width, Height: m.fightEnemyViewHeight() + m.fightCardViewHeight() + 1})
		}
	}

	//
	// Updating
	//

	if len(m.animations) > 0 {
		d, cmd := m.animations[0].Update(msg)

		if d == nil {
			m.animations = lo.Drop(m.animations, 1)
		} else {
			m.animations[0] = d
		}

		return m, cmd
	}

	switch m.Session.GetGameState() {
	case game.GameStateFight:
	case game.GameStateMerchant:
		m.merchant, cmd = m.merchant.Update(msg)
		cmds = append(cmds, cmd)
	case game.GameStateEvent:
		m.event, cmd = m.event.Update(msg)
		cmds = append(cmds, cmd)
	case game.GameStateGameOver:
		return gameover.New(m.zones, m.Session, m.Start), nil
	}

	if m.Session.GetGameState() != m.lastGameState || m.Session.GetEventID() != m.lastEvent {
		diff := m.BeforeStateSwitch.Diff(m.Session)

		m.BeforeStateSwitch = m.Session.MarkState()
		m.lastGameState = m.Session.GetGameState()
		m.lastEvent = m.Session.GetEventID()

		if len(diff) > 0 {
			artifacts := lo.Map(lo.Filter(diff, func(item game.StateCheckpoint, index int) bool {
				added, ok := item.Events[game.StateEventArtifactAdded].(game.StateEventArtifactAddedData)
				return ok && !lo.SomeBy(diff, func(item game.StateCheckpoint) bool {
					removed, ok := item.Events[game.StateEventArtifactRemoved].(game.StateEventArtifactRemovedData)
					return ok && added.GUID == removed.GUID
				})
			}), func(item game.StateCheckpoint, index int) string {
				return components.ArtifactCard(m.Session, item.Events[game.StateEventArtifactAdded].(game.StateEventArtifactAddedData).GUID, 20, 45)
			})

			cards := lo.Map(lo.Filter(diff, func(item game.StateCheckpoint, index int) bool {
				added, ok := item.Events[game.StateEventCardAdded].(game.StateEventCardAddedData)
				return ok && !lo.SomeBy(diff, func(item game.StateCheckpoint) bool {
					removed, ok := item.Events[game.StateEventCardRemoved].(game.StateEventCardRemovedData)
					return ok && added.GUID == removed.GUID
				})
			}), func(item game.StateCheckpoint, index int) string {
				return components.HalfCard(m.Session, item.Events[game.StateEventCardAdded].(game.StateEventCardAddedData).GUID, false, 20, 20, false, 45, false)
			})

			var pushModels []tea.Model

			if len(cards) > 0 {
				c := carousel.New(nil, m.zones, fmt.Sprintf("%d New Cards", len(cards)), cards)
				c.Size = m.Size
				pushModels = append(pushModels, root.NewOnVisibleModel(c, func(model tea.Model) {
					audio.Play("new_cards")
				}))
			}

			if len(artifacts) > 0 {
				c := carousel.New(nil, m.zones, fmt.Sprintf("%d New Artifacts", len(artifacts)), artifacts)
				c.Size = m.Size
				pushModels = append(pushModels, root.NewOnVisibleModel(c, func(model tea.Model) {
					audio.Play("new_artifacts")
				}))
			}

			if len(pushModels) > 0 {
				cmds = append(cmds, root.PushAll(pushModels...))
			}
		}

		cmds = append(cmds, tea.ClearScreen)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if !m.HasSize() {
		return "..."
	}

	// Always finish death animations.
	if len(m.animations) > 0 {
		return lipgloss.JoinVertical(
			lipgloss.Top,
			m.fightStatusTop(),
			m.animations[0].View(),
			m.fightStatusBottom(),
		)
	}

	switch m.Session.GetGameState() {
	case game.GameStateFight:
		if m.inEnemyView {
			return lipgloss.JoinVertical(
				lipgloss.Top,
				m.fightStatusTop(),
				m.fightEnemyInspectView(),
				m.fightStatusBottom(),
			)
		}

		if m.inPlayerView {
			return lipgloss.JoinVertical(
				lipgloss.Top,
				m.fightStatusTop(),
				m.playerInspectView(),
				m.fightStatusBottom(),
			)
		}

		return lipgloss.JoinVertical(
			lipgloss.Top,
			m.fightStatusTop(),
			lipgloss.NewStyle().Width(m.Size.Width).Height(m.fightEnemyViewHeight()).Render(m.fightEnemyView()),
			m.fightDivider(),
			lipgloss.NewStyle().Width(m.Size.Width).Height(m.fightCardViewHeight()).Render(m.fightCardView()),
			m.fightStatusBottom(),
		)
	case game.GameStateMerchant:
		return lipgloss.JoinVertical(lipgloss.Top, m.fightStatusTop(), m.merchant.View())
	case game.GameStateEvent:
		return lipgloss.Place(m.Size.Width, m.Size.Height, lipgloss.Center, lipgloss.Center, m.event.View(), lipgloss.WithWhitespaceChars(" "))
	}

	return fmt.Sprintf("Unknown State: %s", m.Session.GetGameState())
}

//
// Actions
//

func (m Model) finishTurn() Model {
	audio.Play("btn_complete")

	before := m.Session.MarkState()
	m.Session.FinishPlayerTurn()
	damages := before.DiffEvent(m.Session, game.StateEventDamage)

	if len(damages) > 0 {
		hp := m.Session.GetPlayer().HP

		var damageActors []game.Actor
		var damageEnemies []*game.Enemy
		var damageData []game.StateEventDamageData

		for i := range damages {
			dmg := damages[i].Events[game.StateEventDamage].(game.StateEventDamageData)
			if dmg.Source == game.PlayerActorID {
				continue
			}

			src := damages[i].Session.GetActor(dmg.Source)

			damageData = append(damageData, dmg)
			damageEnemies = append(damageEnemies, damages[i].Session.GetEnemy(src.TypeID))
			damageActors = append(damageActors, src)

			hp += dmg.Damage
		}

		m.animations = append(m.animations, NewDamageAnimationModel(m.Size.Width, m.fightEnemyViewHeight()+m.fightCardViewHeight()+1, hp, damageActors, damageEnemies, damageData))
	}

	m.animations = append(m.animations, NewEndTurnAnimationModel(m.Size.Width, m.fightEnemyViewHeight()+m.fightCardViewHeight()+1, m.Session.GetFightRound()+1))

	return m
}

func (m Model) tryCast() Model {
	before := m.Session.MarkState()

	hand := m.Session.GetFight().Hand
	if len(hand) > 0 && m.selectedCard < len(hand) {
		card, _ := m.Session.GetCard(hand[m.selectedCard])
		if card.NeedTarget {
			if m.inOpponentSelection {
				m.inOpponentSelection = false

				if err := m.Session.PlayerCastHand(m.selectedCard, m.Session.GetOpponentByIndex(game.PlayerActorID, m.selectedOpponent).GUID); err == nil {
					audio.Play("damage_1")
				} else {
					audio.Play("btn_deny")
				}
			} else {
				audio.Play("btn_menu")

				m.inOpponentSelection = true
			}
		} else {
			if err := m.Session.PlayerCastHand(m.selectedCard, ""); err == nil {
				audio.Play("btn_menu")
			} else {
				audio.Play("btn_deny")
			}
		}
	}

	// Check if any death occurred in this operation, so we can trigger animations.
	diff := before.DiffEvent(m.Session, game.StateEventDeath)
	m.animations = append(m.animations, lo.Map(diff, func(item game.StateCheckpoint, index int) tea.Model {
		death := item.Events[game.StateEventDeath].(game.StateEventDeathData)
		actor := item.Session.GetActor(death.Target)
		enemy := m.Session.GetEnemy(actor.TypeID)
		return NewDeathAnimationModel(m.Size.Width, m.fightEnemyViewHeight()+m.fightCardViewHeight()+1, actor, enemy, death)
	})...)

	return m
}

//
// Fight View
//

func (m Model) fightStatusTop() string {
	fight := m.Session.GetFight()
	player := m.Session.GetPlayer()

	return components.Header(m.Size.Width, []components.HeaderValue{
		components.NewHeaderValue(fmt.Sprintf("Gold: %d", player.Gold), lipgloss.Color("#FFFF00")),
		components.NewHeaderValue(fmt.Sprintf("HP: %d / %d", player.HP, player.MaxHP), style.BaseRed),
		components.NewHeaderValue(fmt.Sprintf("%d. Stage", m.Session.GetStagesCleared()+1), style.BaseWhite),
		components.NewHeaderValue(fmt.Sprintf("%d. Round", fight.Round+1), style.BaseWhite),
	}, fight.Description)
}

func (m Model) fightDivider() string {
	if m.inOpponentSelection {
		const message = " Select a target for your card... "

		return lipgloss.Place(m.Size.Width, 1, lipgloss.Center, lipgloss.Center, style.RedText.Bold(true).Render(message), lipgloss.WithWhitespaceForeground(style.BaseGrayDarker), lipgloss.WithWhitespaceChars("─"))
	}

	return lipgloss.NewStyle().Foreground(style.BaseGrayDarker).Render(strings.Repeat("─", m.Size.Width))
}

func (m Model) fightStatusBottom() string {
	outerStyle := lipgloss.NewStyle().
		Width(m.Size.Width).
		Foreground(style.BaseWhite).
		Border(lipgloss.BlockBorder(), true, false, false, false).
		BorderForeground(style.BaseRedDarker)

	fight := m.Session.GetFight()

	return outerStyle.Render(lipgloss.JoinHorizontal(
		lipgloss.Center,
		lipgloss.Place(m.Size.Width-40, 3, lipgloss.Left, lipgloss.Center, lipgloss.JoinHorizontal(
			lipgloss.Center,
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(style.BaseWhite)).Padding(0, 4, 0, 4).Render(fmt.Sprintf("Deck: %d", len(fight.Deck))),
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFFF00")).Padding(0, 4, 0, 0).Render(fmt.Sprintf("Used: %d", len(fight.Used))),
			lipgloss.NewStyle().Bold(true).Foreground(style.BaseRed).Padding(0, 4, 0, 0).Render(fmt.Sprintf("Exhausted: %d", len(fight.Exhausted))),
			lipgloss.NewStyle().Bold(true).Foreground(style.BaseGreen).Padding(0, 4, 0, 0).Render(fmt.Sprintf("Action Points: (%d) %s", fight.CurrentPoints, strings.Repeat("• ", fight.CurrentPoints))),
			m.zones.Mark(ZonePlayerInspect, components.StatusEffects(m.Session, m.Session.GetPlayer())),
		),
		),
		lipgloss.Place(40, 3, lipgloss.Right, lipgloss.Center, lipgloss.JoinHorizontal(
			lipgloss.Center,
			m.zones.Mark(ZoneEndTurn, style.HeaderStyle.Copy().Background(lo.Ternary(m.zones.Get(ZoneEndTurn).InBounds(m.LastMouse), style.BaseRed, style.BaseRedDarker)).Margin(0, 4, 0, 0).Render("End Turn")),
			style.RedDarkerText.Render(`▀ █▌█▌▪
 ·██· 
▪▐█·█▌`))),
	))
}

func (m Model) fightEnemyViewHeight() int {
	return m.Size.Height / 3
}

func (m Model) fightCardViewHeight() int {
	return m.Size.Height - m.fightEnemyViewHeight() - 1 - 4 - 4
}

func (m Model) fightEnemyInspectTooltipView() string {
	enemy := m.Session.GetOpponents(game.PlayerActorID)[m.selectedOpponent]

	intend := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(style.BaseWhite).Render("Intend:") + "\n\n" + m.Session.GetActorIntend(enemy.GUID) + "\n\n"

	status := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(style.BaseWhite).Render("Status Effects:") + "\n\n" + strings.Join(lo.Map(enemy.StatusEffects.ToSlice(), func(guid string, index int) string {
		return components.StatusEffect(m.Session, guid) + ": " + m.Session.GetStatusEffectState(guid)
	}), "\n\n")

	return lipgloss.NewStyle().Border(lipgloss.ThickBorder(), true).Padding(1, 2).BorderForeground(style.BaseRedDarker).Render(
		lipgloss.NewStyle().Width(30).Render(intend + status),
	)
}

func (m Model) fightEnemyInspectView() string {
	enemy := m.Session.GetOpponents(game.PlayerActorID)[m.selectedOpponent]

	intend := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(style.BaseWhite).Render("Intend:") + "\n\n" + m.Session.GetActorIntend(enemy.GUID) + "\n\n"

	status := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(style.BaseWhite).Render("Status Effects:") + "\n\n" + strings.Join(lo.Map(enemy.StatusEffects.ToSlice(), func(guid string, index int) string {
		return components.StatusEffect(m.Session, guid) + ": " + m.Session.GetStatusEffectState(guid)
	}), "\n\n")

	return lipgloss.Place(m.Size.Width, m.fightEnemyViewHeight()+m.fightCardViewHeight()+1, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.ThickBorder(), true).Padding(1, 2).BorderForeground(style.BaseRedDarker).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, true, false, false).BorderForeground(style.BaseGrayDarker).Padding(0, 2, 2, 0).Render(components.Actor(m.Session, enemy, m.Session.GetEnemy(enemy.TypeID), true, true, false)),
				lipgloss.NewStyle().Margin(0, 0, 0, 3).Width(30).Render(intend+status+"\n\n"+style.GrayText.Render("press 'esc' to close")),
			),
		),
		lipgloss.WithWhitespaceChars("?"), lipgloss.WithWhitespaceForeground(style.BaseGrayDarker),
	)
}

func (m Model) playerInspectView() string {
	player := m.Session.GetPlayer()

	status := lipgloss.NewStyle().Bold(true).Underline(true).Foreground(style.BaseWhite).Render("Status Effects:") + "\n\n" + strings.Join(lo.Map(player.StatusEffects.ToSlice(), func(guid string, index int) string {
		return components.StatusEffect(m.Session, guid) + ": " + m.Session.GetStatusEffectState(guid)
	}), "\n\n")

	return lipgloss.Place(m.Size.Width, m.fightEnemyViewHeight()+m.fightCardViewHeight()+1, lipgloss.Center, lipgloss.Center,
		lipgloss.NewStyle().Border(lipgloss.ThickBorder(), true).Padding(1, 2).BorderForeground(style.BaseRedDarker).Render(
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.NewStyle().Margin(0, 0, 0, 3).Width(30).Render(
					lipgloss.NewStyle().Foreground(style.BaseWhite).Bold(true).Underline(true).Render("Player Status")+"\n\n"+status+"\n\n"+style.GrayText.Render("press 'esc' to close"),
				),
			),
		),
		lipgloss.WithWhitespaceChars("?"), lipgloss.WithWhitespaceForeground(style.BaseGrayDarker),
	)
}

func (m Model) fightEnemyView() string {
	enemyBoxes := lo.Map(m.Session.GetOpponents(game.PlayerActorID), func(actor game.Actor, i int) string {
		intend := m.Session.GetActorIntend(actor.GUID)
		if len(intend) > 0 {
			intend = "\n" + lipgloss.NewStyle().Italic(true).Foreground(style.BaseGray).Render(intend)
		}

		return components.Actor(
			m.Session,
			actor,
			m.Session.GetEnemy(actor.TypeID),
			true,
			true,
			m.inOpponentSelection && i == m.selectedOpponent || m.zones.Get(fmt.Sprintf("%s%d", ZoneEnemy, i)).InBounds(m.LastMouse),
			intend,
		)
	})

	enemyBoxes = lo.Map(enemyBoxes, func(item string, i int) string {
		return m.zones.Mark(fmt.Sprintf("%s%d", ZoneEnemy, i), item)
	})

	return lipgloss.Place(m.Size.Width, m.fightEnemyViewHeight(), lipgloss.Center, lipgloss.Center, lipgloss.JoinHorizontal(lipgloss.Center, enemyBoxes...), lipgloss.WithWhitespaceChars(" "))
}

func (m Model) fightCardView() string {
	fight := m.Session.GetFight()
	var cardBoxes = lo.Map(fight.Hand, func(guid string, index int) string {
		return components.HalfCard(m.Session, guid, index == m.selectedCard, m.fightCardViewHeight()/2, m.fightCardViewHeight()-1, len(fight.Hand)*35 >= m.Size.Width, 0, true)
	})

	cardBoxes = lo.Map(cardBoxes, func(item string, i int) string {
		return m.zones.Mark(fmt.Sprintf("%s%d", ZoneCard, i), item)
	})

	return lipgloss.Place(m.Size.Width, m.fightCardViewHeight(), lipgloss.Center, lipgloss.Bottom, lipgloss.JoinHorizontal(lipgloss.Bottom, cardBoxes...), lipgloss.WithWhitespaceChars(" "))
}
