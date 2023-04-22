package gameview

import (
	"fmt"
	"github.com/BigJk/project_gonzo/audio"
	"github.com/BigJk/project_gonzo/game"
	"github.com/BigJk/project_gonzo/ui"
	"github.com/BigJk/project_gonzo/ui/components"
	"github.com/BigJk/project_gonzo/ui/gameover"
	"github.com/BigJk/project_gonzo/ui/style"
	"github.com/BigJk/project_gonzo/util"
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/muesli/reflow/wordwrap"
	"github.com/samber/lo"
	"strconv"
	"strings"
)

const (
	ZoneCard          = "card_"
	ZoneEnemy         = "enemy_"
	ZoneEventChoice   = "event_choice_"
	ZoneEndTurn       = "end_turn"
	ZoneBuyItem       = "buy_item"
	ZoneLeaveMerchant = "leave_merchant"
)

type Model struct {
	ui.MenuBase

	zones               *zone.Manager
	parent              tea.Model
	viewport            viewport.Model
	selectedChoice      int
	selectedCard        int
	selectedOpponent    int
	inOpponentSelection bool
	animations          []tea.Model

	lastMouse tea.MouseMsg

	merchantSellTable table.Model

	Session *game.Session
	Start   game.StateCheckpointMarker
}

func New(parent tea.Model, zones *zone.Manager, session *game.Session) Model {
	session.Log(game.LogTypeSuccess, "Game started! Good luck...")

	return Model{
		zones:   zones,
		parent:  parent,
		Session: session,
		Start:   session.MarkState(),

		merchantSellTable: table.New(table.WithStyles(tableStyle), table.WithColumns([]table.Column{
			{Title: "Type", Width: 10},
			{Title: "Name", Width: 10},
			{Title: "Price", Width: 10},
		})),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	//
	// Keyboard
	//
	case tea.KeyMsg:
		if val, err := strconv.Atoi(msg.String()); err == nil {
			m.selectedChoice = val - 1
		}

		switch msg.Type {
		case tea.KeyEnter:
			switch m.Session.GetGameState() {
			// If we are in an event commit the choice. Only commit if choice is in range.
			case game.GameStateEvent:
				m = m.tryFinishEvent()
			// Cast a card
			case game.GameStateFight:
				if m.selectedCard >= len(m.Session.GetFight().Hand) {
					m.selectedCard = 0
				}

				m = m.tryCast()
			// Buy selected item
			case game.GameStateMerchant:
				m.merchantBuy()
			}
		case tea.KeyEscape:
			// Switch to menu
			if m.inOpponentSelection {
				m.inOpponentSelection = false
			} else {
				return NewMenuModel(m, m.zones, m.Session), nil
			}
		case tea.KeyTab:
			switch m.Session.GetGameState() {
			// Select a choice
			case game.GameStateEvent:
				if len(m.Session.GetEvent().Choices) > 0 {
					m.selectedChoice = (m.selectedChoice + 1) % len(m.Session.GetEvent().Choices)
				}
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
		case tea.KeyRight:
			// TODO: right / left movement
		}
	//
	// Mouse
	//
	case tea.MouseMsg:
		m.lastMouse = msg

		if msg.Type == tea.MouseLeft {
			switch m.Session.GetGameState() {
			case game.GameStateEvent:
			case game.GameStateFight:
				if m.zones.Get(ZoneEndTurn).InBounds(msg) {
					m = m.finishTurn()
				}
			case game.GameStateMerchant:
				if m.zones.Get(ZoneBuyItem).InBounds(msg) {
					m.merchantBuy()
				} else if m.zones.Get(ZoneLeaveMerchant).InBounds(msg) {
					m.Session.LeaveMerchant()
				}
			}
		}

		if msg.Type == tea.MouseLeft || msg.Type == tea.MouseMotion {
			switch m.Session.GetGameState() {
			case game.GameStateEvent:
				if m.Session.GetEvent() != nil {
					for i := 0; i < len(m.Session.GetEvent().Choices); i++ {
						if choiceZone := m.zones.Get(fmt.Sprintf("%s%d", ZoneEventChoice, i)); choiceZone.InBounds(msg) {
							if msg.Type == tea.MouseLeft && m.selectedChoice == i {
								audio.Play("button")

								m = m.tryFinishEvent()
								break
							} else {
								m.selectedChoice = i
							}
						}
					}
				}
			case game.GameStateFight:
				if m.inOpponentSelection {
					for i := 0; i < m.Session.GetOpponentCount(game.PlayerActorID); i++ {
						if cardZone := m.zones.Get(fmt.Sprintf("%s%d", ZoneEnemy, i)); cardZone.InBounds(msg) {
							if msg.Type == tea.MouseLeft && m.selectedOpponent == i {
								m = m.tryCast()
							} else {
								m.selectedOpponent = i
							}
						}
					}
				} else {
					onCard := false
					for i := 0; i < len(m.Session.GetFight().Hand); i++ {
						if cardZone := m.zones.Get(fmt.Sprintf("%s%d", ZoneCard, i)); cardZone.InBounds(msg) {
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
				}
			}
		}
	//
	// Window Size
	//
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.eventHeaderView())
		footerHeight := lipgloss.Height(m.eventFooterView())
		verticalMarginHeight := headerHeight + footerHeight + m.eventChoiceHeight()

		if !m.HasSize() {
			m.viewport = viewport.New(util.Min(msg.Width, 100), msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = false

			m = m.eventUpdateContent()
		} else {
			m.viewport.Width = util.Min(msg.Width, 100)
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		m.Size = msg

		for i := range m.animations {
			m.animations[i], _ = m.animations[i].Update(ui.SizeMsg{Width: m.Size.Width, Height: m.fightEnemyViewHeight() + m.fightCardViewHeight() + 1})
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
		merchant := m.Session.GetMerchant()

		m.merchantSellTable.SetRows(lo.Flatten([][]table.Row{
			lo.Map(merchant.Artifacts, func(guid string, index int) table.Row {
				artifact, _ := m.Session.GetArtifact(guid)
				return table.Row{"Artifact", artifact.Name, fmt.Sprintf("%d$", artifact.Price)}
			}),
			lo.Map(merchant.Cards, func(guid string, index int) table.Row {
				card, _ := m.Session.GetCard(guid)
				return table.Row{"Card", card.Name, fmt.Sprintf("%d$", card.Price)}
			}),
		}))

		m.merchantSellTable.Focus()
		m.merchantSellTable, cmd = m.merchantSellTable.Update(msg)
		cmds = append(cmds, cmd)
	case game.GameStateEvent:
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	case game.GameStateGameOver:
		return gameover.New(m.zones, m.Session, m.Start), nil
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
		return lipgloss.JoinVertical(
			lipgloss.Top,
			m.fightStatusTop(),
			lipgloss.NewStyle().Width(m.Size.Width).Height(m.fightEnemyViewHeight()).Render(m.fightEnemyView()),
			m.fightDivider(),
			lipgloss.NewStyle().Width(m.Size.Width).Height(m.fightCardViewHeight()).Render(m.fightCardView()),
			m.fightStatusBottom(),
		)
	case game.GameStateMerchant:
		return lipgloss.JoinVertical(
			lipgloss.Top,
			m.fightStatusTop(),
			m.merchantView(),
		)
	case game.GameStateEvent:
		return lipgloss.Place(
			m.Size.Width,
			m.Size.Height,
			lipgloss.Center,
			lipgloss.Center,
			fmt.Sprintf("%s\n%s\n%s\n%s", m.eventHeaderView(), m.viewport.View(), m.eventFooterView(), strings.Join(m.eventChoices(), "\n")),
			lipgloss.WithWhitespaceChars(" "),
		)
	}

	return fmt.Sprintf("Unknown State: %s", m.Session.GetGameState())
}

//
// Actions
//

func (m Model) finishTurn() Model {
	audio.Play("button")

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

	return m
}

func (m Model) tryCast() Model {
	before := m.Session.MarkState()

	if len(m.Session.GetFight().Hand) > 0 {
		card, _ := m.Session.GetCard(m.Session.GetFight().Hand[m.selectedCard])
		if card.NeedTarget {
			if m.inOpponentSelection {
				m.inOpponentSelection = false

				if err := m.Session.PlayerCastHand(m.selectedCard, m.Session.GetOpponentByIndex(game.PlayerActorID, m.selectedOpponent).GUID); err == nil {
					audio.Play("damage_1")
				} else {
					audio.Play("button_deny")
				}
			} else {
				audio.Play("button")

				m.inOpponentSelection = true
			}
		} else {
			if err := m.Session.PlayerCastHand(m.selectedCard, ""); err == nil {
				audio.Play("button")
			} else {
				audio.Play("button_deny")
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

func (m Model) tryFinishEvent() Model {
	if len(m.Session.GetEvent().Choices) == 0 || m.selectedChoice < len(m.Session.GetEvent().Choices) {
		m.Session.FinishEvent(m.selectedChoice)
		return m.eventUpdateContent()
	}
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
			lipgloss.NewStyle().Bold(true).Foreground(style.BaseGreen).Padding(0, 4, 0, 0).Render(fmt.Sprintf("Action Points: %d / %d", fight.CurrentPoints, game.PointsPerRound)),
			components.StatusEffects(m.Session, m.Session.GetPlayer()),
		),
		),
		lipgloss.Place(40, 3, lipgloss.Right, lipgloss.Center, lipgloss.JoinHorizontal(
			lipgloss.Center,
			m.zones.Mark(ZoneEndTurn, style.HeaderStyle.Copy().Background(lo.Ternary(m.zones.Get(ZoneEndTurn).InBounds(m.lastMouse), style.BaseRed, style.BaseRedDarker)).Margin(0, 4, 0, 0).Render("End Turn")),
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

var faceStyle = lipgloss.NewStyle().Border(lipgloss.OuterHalfBlockBorder()).Padding(0, 1).Margin(0, 0, 1, 0).BorderForeground(style.BaseGrayDarker).Foreground(style.BaseRed)

func (m Model) fightEnemyView() string {
	enemyBoxes := lo.Map(m.Session.GetOpponents(game.PlayerActorID), func(actor game.Actor, i int) string {
		return components.Actor(m.Session, actor, m.Session.GetEnemy(actor.TypeID), true, true, m.inOpponentSelection && i == m.selectedOpponent)
	})

	enemyBoxes = lo.Map(enemyBoxes, func(item string, i int) string {
		return m.zones.Mark(fmt.Sprintf("%s%d", ZoneEnemy, i), item)
	})

	return lipgloss.Place(m.Size.Width, m.fightEnemyViewHeight(), lipgloss.Center, lipgloss.Center, lipgloss.JoinHorizontal(lipgloss.Center, enemyBoxes...), lipgloss.WithWhitespaceChars(" "))
}

func (m Model) fightCardView() string {
	fight := m.Session.GetFight()
	var cardBoxes = lo.Map(fight.Hand, func(guid string, index int) string {
		return components.HalfCard(m.Session, guid, index == m.selectedCard, m.fightCardViewHeight()/2, m.fightCardViewHeight()-1, len(fight.Hand)*40 >= m.Size.Width)
	})

	cardBoxes = lo.Map(cardBoxes, func(item string, i int) string {
		return m.zones.Mark(fmt.Sprintf("%s%d", ZoneCard, i), item)
	})

	return lipgloss.Place(m.Size.Width, m.fightCardViewHeight(), lipgloss.Center, lipgloss.Bottom, lipgloss.JoinHorizontal(lipgloss.Bottom, cardBoxes...), lipgloss.WithWhitespaceChars(" "))
}

//
// Merchant View
//

func (m Model) merchantGetSelected() any {
	merchant := m.Session.GetMerchant()
	items := lo.Flatten([][]any{
		lo.Map(merchant.Artifacts, func(guid string, index int) any {
			artifact, _ := m.Session.GetArtifact(guid)
			return artifact
		}),
		lo.Map(merchant.Cards, func(guid string, index int) any {
			card, _ := m.Session.GetCard(guid)
			return card
		}),
	})

	if len(items) < m.merchantSellTable.Cursor() {
		return nil
	}

	return items[m.merchantSellTable.Cursor()]
}

func (m Model) merchantBuy() {
	item := m.merchantGetSelected()

	switch item := item.(type) {
	case *game.Artifact:
		if m.Session.PlayerBuyArtifact(item.ID) {
			m.merchantSellTable.SetCursor(util.Max(0, m.merchantSellTable.Cursor()-1))
		}
	case *game.Card:
		if m.Session.PlayerBuyCard(item.ID) {
			m.merchantSellTable.SetCursor(util.Max(0, m.merchantSellTable.Cursor()-1))
		}
	}
}

func (m Model) merchantView() string {
	// Face
	merchant := m.Session.GetMerchant()
	merchantWidth := util.Max(lipgloss.Width(merchant.Face), 30)

	faceSection := lipgloss.JoinVertical(
		lipgloss.Top,
		lipgloss.NewStyle().Margin(0, 2, 0, 2).Padding(1).Border(lipgloss.InnerHalfBlockBorder()).BorderForeground(style.BaseGray).Render(
			lipgloss.Place(merchantWidth, lipgloss.Height(merchant.Face), lipgloss.Center, lipgloss.Center, lipgloss.NewStyle().Bold(true).Foreground(style.BaseGray).Render(merchant.Face)),
		),
		lipgloss.NewStyle().
			Margin(1, 2, 2, 2).
			Padding(0, 2).
			Bold(true).Italic(true).
			Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(style.BaseGray).
			Width(merchantWidth).Render(merchant.Text),
		style.HeaderStyle.Copy().Background(lo.Ternary(m.zones.Get(ZoneLeaveMerchant).InBounds(m.lastMouse), style.BaseRed, style.BaseRedDarker)).Margin(0, 2).Render(m.zones.Mark(ZoneLeaveMerchant, "Leave Merchant")),
	)
	faceSectionWidth := lipgloss.Width(faceSection)

	// Wares

	m.merchantSellTable.SetColumns([]table.Column{
		{Title: "Type", Width: 15},
		{Title: "Name", Width: m.Size.Width - faceSectionWidth - 40 - 15 - 10},
		{Title: "Price", Width: 10},
	})
	m.merchantSellTable.SetWidth(m.Size.Width - faceSectionWidth - 40)
	m.merchantSellTable.SetHeight(util.Min(m.Size.Height-4-10, len(m.merchantSellTable.Rows())+1))

	canBuy := false
	selectedItem := m.merchantGetSelected()
	var selectedItemLook string
	switch item := selectedItem.(type) {
	case *game.Artifact:
		selectedItemLook = components.ArtifactCard(m.Session, item.ID, 20, 20)
		canBuy = m.Session.GetPlayer().Gold >= item.Price
	case *game.Card:
		selectedItemLook = components.HalfCard(m.Session, item.ID, false, 20, 20, false)
		canBuy = m.Session.GetPlayer().Gold >= item.Price
	}

	help := help.New()
	help.Width = m.Size.Width - faceSectionWidth - 40 - 15 - 10
	helpText := help.ShortHelpView([]key.Binding{
		key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "move up")),
		key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "move down")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "buy")),
	})

	shopSection := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Left,
			lipgloss.JoinVertical(lipgloss.Top, m.merchantSellTable.View(), helpText),
			lipgloss.JoinVertical(lipgloss.Top,
				selectedItemLook,
				style.HeaderStyle.Copy().Background(
					lo.Ternary(canBuy, lo.Ternary(m.zones.Get(ZoneBuyItem).InBounds(m.lastMouse), style.BaseRed, style.BaseRedDarker), style.BaseGrayDarker),
				).Margin(1, 2).Render(m.zones.Mark(ZoneBuyItem, "Buy Item")),
			),
		),
	)

	return lipgloss.JoinVertical(
		lipgloss.Top,
		style.HeaderStyle.Render("Merchant Wares"),
		lipgloss.JoinHorizontal(lipgloss.Left, faceSection, shopSection),
	)
}

//
// Event View
//

func (m Model) eventUpdateContent() Model {
	if m.Session.GetEvent() == nil {
		m.viewport.SetContent("")
		return m
	}

	r, _ := glamour.NewTermRenderer(
		glamour.WithStyles(glamour.DarkStyleConfig),
		glamour.WithWordWrap(m.viewport.Width),
	)
	res, _ := r.Render(m.Session.GetEvent().Description)

	m.viewport.SetContent(res)
	return m
}

var titleStyle = lipgloss.NewStyle().BorderStyle(lipgloss.ThickBorder()).BorderForeground(style.BaseRedDarker).Foreground(style.BaseWhite).Padding(0, 1)
var infoStyle = lipgloss.NewStyle().BorderStyle(lipgloss.ThickBorder()).BorderForeground(style.BaseRedDarker).Foreground(style.BaseWhite).Padding(0, 1)

func (m Model) eventHeaderView() string {
	if m.Session.GetEvent() == nil {
		return ""
	}

	title := titleStyle.Render(m.Session.GetEvent().Name)
	line := style.GrayTextDarker.Render(strings.Repeat("━", util.Max(0, m.viewport.Width-lipgloss.Width(title))))
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m Model) eventFooterView() string {
	if m.Session.GetEvent() == nil {
		return ""
	}

	info := infoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := style.GrayTextDarker.Render(strings.Repeat("━", util.Max(0, m.viewport.Width-lipgloss.Width(info))))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

var choiceStyle = lipgloss.NewStyle().Padding(0, 1).Border(lipgloss.ThickBorder(), true).BorderForeground(style.BaseGrayDarker).Foreground(style.BaseWhite)
var choiceSelectedStyle = choiceStyle.Copy().BorderForeground(style.BaseRed).Foreground(style.BaseWhite)

func (m Model) eventChoices() []string {
	if m.Session.GetEvent() == nil {
		return nil
	}

	choices := lo.Map(m.Session.GetEvent().Choices, func(item game.EventChoice, index int) string {
		if m.selectedChoice == index {
			return choiceSelectedStyle.Width(util.Min(m.Size.Width, 100)).Render(wordwrap.String(fmt.Sprintf("%d. %s", index+1, item.Description), util.Min(m.Size.Width, 100-choiceStyle.GetHorizontalFrameSize())))
		}
		return choiceStyle.Width(util.Min(m.Size.Width, 100)).Render(wordwrap.String(fmt.Sprintf("%d. %s", index+1, item.Description), util.Min(m.Size.Width, 100-choiceStyle.GetHorizontalFrameSize())))
	})

	return lo.Map(choices, func(item string, index int) string {
		return m.zones.Mark(fmt.Sprintf("%s%d", ZoneEventChoice, index), item)
	})
}

func (m Model) eventChoiceHeight() int {
	return lo.SumBy(m.eventChoices(), func(item string) int {
		return lipgloss.Height(item) + 5
	})
}
