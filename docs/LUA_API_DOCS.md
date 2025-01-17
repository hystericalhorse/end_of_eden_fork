# End Of Eden Lua Docs
## Index

- [Game Constants](#game-constants)
- [Utility](#utility)
- [Styling](#styling)
- [Logging](#logging)
- [Audio](#audio)
- [Game State](#game-state)
- [Actor Operations](#actor-operations)
- [Artifact Operations](#artifact-operations)
- [Status Effect Operations](#status-effect-operations)
- [Card Operations](#card-operations)
- [Damage & Heal](#damage--heal)
- [Player Operations](#player-operations)
- [Merchant Operations](#merchant-operations)
- [Random Utility](#random-utility)
- [Localization](#localization)
- [Content Registry](#content-registry)

## Game Constants

General game constants.

### Globals
<details> <summary><b><code>DECAY_ALL</code></b> </summary> <br/>

Status effect decays by all stacks per turn.

</details>

<details> <summary><b><code>DECAY_NONE</code></b> </summary> <br/>

Status effect never decays.

</details>

<details> <summary><b><code>DECAY_ONE</code></b> </summary> <br/>

Status effect decays by 1 stack per turn.

</details>

<details> <summary><b><code>GAME_STATE_EVENT</code></b> </summary> <br/>

Represents the event game state.

</details>

<details> <summary><b><code>GAME_STATE_FIGHT</code></b> </summary> <br/>

Represents the fight game state.

</details>

<details> <summary><b><code>GAME_STATE_MERCHANT</code></b> </summary> <br/>

Represents the merchant game state.

</details>

<details> <summary><b><code>GAME_STATE_RANDOM</code></b> </summary> <br/>

Represents the random game state in which the active story teller will decide what happens next.

</details>

<details> <summary><b><code>PLAYER_ID</code></b> </summary> <br/>

Player actor id for use in functions where the guid is needed, for example: ``deal_damage(PLAYER_ID, enemy_guid, 10)``.

</details>

### Functions

None

## Utility

General game constants.

### Globals

None

### Functions
<details> <summary><b><code>fetch</code></b> </summary> <br/>

Fetches a value from the persistent store

**Signature:**

```
fetch(key : string) -> any
```

</details>

<details> <summary><b><code>guid</code></b> </summary> <br/>

returns a new random guid.

**Signature:**

```
guid() -> guid
```

</details>

<details> <summary><b><code>store</code></b> </summary> <br/>

Stores a persistent value for this run that will be restored after a save load. Can store any lua basic value or table.

**Signature:**

```
store(key : string, value : any) -> None
```

</details>

## Styling

Helper functions for text styling.

### Globals

None

### Functions
<details> <summary><b><code>text_bg</code></b> </summary> <br/>

Makes the text background colored. Takes hex values like #ff0000.

**Signature:**

```
text_bg(color : string, value : any) -> string
```

</details>

<details> <summary><b><code>text_bold</code></b> </summary> <br/>

Makes the text bold.

**Signature:**

```
text_bold(value : any) -> string
```

</details>

<details> <summary><b><code>text_italic</code></b> </summary> <br/>

Makes the text italic.

**Signature:**

```
text_italic(value : any) -> string
```

</details>

<details> <summary><b><code>text_red</code></b> </summary> <br/>

Makes the text colored red.

**Signature:**

```
text_red(value : any) -> string
```

</details>

<details> <summary><b><code>text_underline</code></b> </summary> <br/>

Makes the text underlined.

**Signature:**

```
text_underline(value : any) -> string
```

</details>

## Logging

Various logging functions.

### Globals

None

### Functions
<details> <summary><b><code>log_d</code></b> </summary> <br/>

Log at **danger** level to player log.

**Signature:**

```
log_d(value : any) -> None
```

</details>

<details> <summary><b><code>log_i</code></b> </summary> <br/>

Log at **information** level to player log.

**Signature:**

```
log_i(value : any) -> None
```

</details>

<details> <summary><b><code>log_s</code></b> </summary> <br/>

Log at **success** level to player log.

**Signature:**

```
log_s(value : any) -> None
```

</details>

<details> <summary><b><code>log_w</code></b> </summary> <br/>

Log at **warning** level to player log.

**Signature:**

```
log_w(value : any) -> None
```

</details>

<details> <summary><b><code>print</code></b> </summary> <br/>

Log to session log.

**Signature:**

```
print(...) -> None
```

</details>

## Audio

Audio helper functions.

### Globals

None

### Functions
<details> <summary><b><code>play_audio</code></b> </summary> <br/>

Plays a sound effect. If you want to play ``button.mp3`` you call ``play_audio("button")``.

**Signature:**

```
play_audio(sound : string) -> None
```

</details>

<details> <summary><b><code>play_music</code></b> </summary> <br/>

Start a song for the background loop. If you want to play ``song.mp3`` you call ``play_music("song")``.

**Signature:**

```
play_music(sound : string) -> None
```

</details>

## Game State

Functions that modify the general game state.

### Globals

None

### Functions
<details> <summary><b><code>get_event_history</code></b> </summary> <br/>

Gets the ids of all the encountered events in the order of occurrence.

**Signature:**

```
get_event_history() -> string[]
```

</details>

<details> <summary><b><code>get_fight</code></b> </summary> <br/>

Gets the fight state. This contains the player hand, used, exhausted and round information.

**Signature:**

```
get_fight() -> fight_state
```

</details>

<details> <summary><b><code>get_fight_round</code></b> </summary> <br/>

Gets the fight round.

**Signature:**

```
get_fight_round() -> number
```

</details>

<details> <summary><b><code>get_stages_cleared</code></b> </summary> <br/>

Gets the number of stages cleared.

**Signature:**

```
get_stages_cleared() -> number
```

</details>

<details> <summary><b><code>had_event</code></b> </summary> <br/>

Checks if the event happened at least once.

**Signature:**

```
had_event(event_id : type_id) -> boolean
```

</details>

<details> <summary><b><code>had_events</code></b> </summary> <br/>

Checks if all the events happened at least once.

**Signature:**

```
had_events(event_ids : type_id[]) -> boolean
```

</details>

<details> <summary><b><code>had_events_any</code></b> </summary> <br/>

Checks if any of the events happened at least once.

**Signature:**

```
had_events_any(eventIds : string[]) -> boolean
```

</details>

<details> <summary><b><code>set_event</code></b> </summary> <br/>

Set event by id.

**Signature:**

```
set_event(event_id : type_id) -> None
```

</details>

<details> <summary><b><code>set_fight_description</code></b> </summary> <br/>

Set the current fight description. This will be shown on the top right in the game.

**Signature:**

```
set_fight_description(desc : string) -> None
```

</details>

<details> <summary><b><code>set_game_state</code></b> </summary> <br/>

Set the current game state. See globals.

**Signature:**

```
set_game_state(state : next_game_state) -> None
```

</details>

## Actor Operations

Functions that modify or access the actors. Actors are either the player or enemies.

### Globals

None

### Functions
<details> <summary><b><code>actor_add_hp</code></b> </summary> <br/>

Increases the hp value of a actor by a number. Can be negative value to decrease it. This won't trigger any on_damage callbacks

**Signature:**

```
actor_add_hp(guid : guid, amount : number) -> None
```

</details>

<details> <summary><b><code>actor_add_max_hp</code></b> </summary> <br/>

Increases the max hp value of a actor by a number. Can be negative value to decrease it.

**Signature:**

```
actor_add_max_hp(guid : guid, amount : number) -> None
```

</details>

<details> <summary><b><code>actor_set_hp</code></b> </summary> <br/>

Sets the hp value of a actor to a number. This won't trigger any on_damage callbacks

**Signature:**

```
actor_set_hp(guid : guid, amount : number) -> None
```

</details>

<details> <summary><b><code>actor_set_max_hp</code></b> </summary> <br/>

Sets the max hp value of a actor to a number.

**Signature:**

```
actor_set_max_hp(guid : guid, amount : number) -> None
```

</details>

<details> <summary><b><code>add_actor_by_enemy</code></b> </summary> <br/>

Creates a new enemy fighting against the player. Example ``add_actor_by_enemy("RUST_MITE")``.

**Signature:**

```
add_actor_by_enemy(enemy_guid : type_id) -> string
```

</details>

<details> <summary><b><code>get_actor</code></b> </summary> <br/>

Get a actor by guid.

**Signature:**

```
get_actor(guid : guid) -> actor
```

</details>

<details> <summary><b><code>get_opponent_by_index</code></b> </summary> <br/>

Get opponent (actor) by index of a certain actor. ``get_opponent_by_index(PLAYER_ID, 2)`` would return the second alive opponent of the player.

**Signature:**

```
get_opponent_by_index(guid : guid, index : number) -> actor
```

</details>

<details> <summary><b><code>get_opponent_count</code></b> </summary> <br/>

Get the number of opponents (actors) of a certain actor. ``get_opponent_count(PLAYER_ID)`` would return 2 if the player had 2 alive enemies.

**Signature:**

```
get_opponent_count(guid : guid) -> number
```

</details>

<details> <summary><b><code>get_opponent_guids</code></b> </summary> <br/>

Get the guids of opponents (actors) of a certain actor. If the player had 2 enemies, ``get_opponent_guids(PLAYER_ID)`` would return a table with 2 strings containing the guids of these actors.

**Signature:**

```
get_opponent_guids(guid : guid) -> guid[]
```

</details>

<details> <summary><b><code>get_player</code></b> </summary> <br/>

Get the player actor. Equivalent to ``get_actor(PLAYER_ID)``

**Signature:**

```
get_player() -> actor
```

</details>

<details> <summary><b><code>remove_actor</code></b> </summary> <br/>

Deletes a actor by id.

**Signature:**

```
remove_actor(guid : guid) -> None
```

</details>

## Artifact Operations

Functions that modify or access the artifacts.

### Globals

None

### Functions
<details> <summary><b><code>get_artifact</code></b> </summary> <br/>

Returns the artifact definition. Can take either a guid or a typeId. If it's a guid it will fetch the type behind the instance.

**Signature:**

```
get_artifact(id : string) -> artifact
```

</details>

<details> <summary><b><code>get_artifact_instance</code></b> </summary> <br/>

Returns the artifact instance by guid.

**Signature:**

```
get_artifact_instance(guid : guid) -> artifact_instance
```

</details>

<details> <summary><b><code>get_artifacts</code></b> </summary> <br/>

Returns all the artifacts guids from the given actor.

**Signature:**

```
get_artifacts(actor_guid : string) -> guid[]
```

</details>

<details> <summary><b><code>give_artifact</code></b> </summary> <br/>

Gives a actor a artifact. Returns the guid of the newly created artifact.

**Signature:**

```
give_artifact(type_id : type_id, actor : guid) -> string
```

</details>

<details> <summary><b><code>remove_artifact</code></b> </summary> <br/>

Removes a artifact.

**Signature:**

```
remove_artifact(guid : guid) -> None
```

</details>

## Status Effect Operations

Functions that modify or access the status effects.

### Globals

None

### Functions
<details> <summary><b><code>add_status_effect_stacks</code></b> </summary> <br/>

Adds to the stack count of a status effect. Negative values are also allowed.

**Signature:**

```
add_status_effect_stacks(guid : guid, count : number) -> None
```

</details>

<details> <summary><b><code>get_actor_status_effects</code></b> </summary> <br/>

Returns the guids of all status effects that belong to a actor.

**Signature:**

```
get_actor_status_effects(actor_guid : string) -> guid[]
```

</details>

<details> <summary><b><code>get_status_effect</code></b> </summary> <br/>

Returns the status effect definition. Can take either a guid or a typeId. If it's a guid it will fetch the type behind the instance.

**Signature:**

```
get_status_effect(id : string) -> status_effect
```

</details>

<details> <summary><b><code>get_status_effect_instance</code></b> </summary> <br/>

Returns the status effect instance.

**Signature:**

```
get_status_effect_instance(effect_guid : guid) -> status_effect_instance
```

</details>

<details> <summary><b><code>give_status_effect</code></b> </summary> <br/>

Gives a status effect to a actor. If count is not specified a stack of 1 is applied.

**Signature:**

```
give_status_effect(type_id : string, actor_guid : string, (optional) count : number) -> None
```

</details>

<details> <summary><b><code>remove_status_effect</code></b> </summary> <br/>

Removes a status effect.

**Signature:**

```
remove_status_effect(guid : guid) -> None
```

</details>

<details> <summary><b><code>set_status_effect_stacks</code></b> </summary> <br/>

Sets the stack count of a status effect by guid.

**Signature:**

```
set_status_effect_stacks(guid : guid, count : number) -> None
```

</details>

## Card Operations

Functions that modify or access the cards.

### Globals

None

### Functions
<details> <summary><b><code>cast_card</code></b> </summary> <br/>

Tries to cast a card with a guid and optional target. If the cast isn't successful returns false.

**Signature:**

```
cast_card(card_guid : guid, (optional) target_actor_guid : guid) -> boolean
```

</details>

<details> <summary><b><code>get_card</code></b> </summary> <br/>

Returns the card type definition. Can take either a guid or a typeId. If it's a guid it will fetch the type behind the instance.

**Signature:**

```
get_card(id : type_id) -> card
```

</details>

<details> <summary><b><code>get_card_instance</code></b> </summary> <br/>

Returns the instance object of a card.

**Signature:**

```
get_card_instance(card_guid : guid) -> card_instance
```

</details>

<details> <summary><b><code>get_cards</code></b> </summary> <br/>

Returns all the card guids from the given actor.

**Signature:**

```
get_cards(actor_guid : string) -> guid[]
```

</details>

<details> <summary><b><code>give_card</code></b> </summary> <br/>

Gives a card.

**Signature:**

```
give_card(card_type_id : type_id, owner_actor_guid : guid) -> string
```

</details>

<details> <summary><b><code>remove_card</code></b> </summary> <br/>

Removes a card.

**Signature:**

```
remove_card(card_guid : string) -> None
```

</details>

<details> <summary><b><code>upgrade_card</code></b> </summary> <br/>

Upgrade a card without paying for it.

**Signature:**

```
upgrade_card(card_guid : guid) -> boolean
```

</details>

<details> <summary><b><code>upgrade_random_card</code></b> </summary> <br/>

Upgrade a random card without paying for it.

**Signature:**

```
upgrade_random_card(actor_guid : guid) -> boolean
```

</details>

## Damage & Heal

Functions that deal damage or heal.

### Globals

None

### Functions
<details> <summary><b><code>deal_damage</code></b> </summary> <br/>

Deal damage from one source to a target. If flat is true the damage can't be modified by status effects or artifacts. Returns the damage that was dealt.

**Signature:**

```
deal_damage(source : guid, target : guid, damage : number, (optional) flat : boolean) -> number
```

</details>

<details> <summary><b><code>deal_damage_multi</code></b> </summary> <br/>

Deal damage to multiple enemies from one source. If flat is true the damage can't be modified by status effects or artifacts. Returns a array of damages for each actor hit.

**Signature:**

```
deal_damage_multi(source : guid, targets : guid[], damage : number, (optional) flat : boolean) -> number[]
```

</details>

<details> <summary><b><code>heal</code></b> </summary> <br/>

Heals the target triggered by the source.

**Signature:**

```
heal(source : guid, target : guid, amount : number) -> None
```

</details>

<details> <summary><b><code>simulate_deal_damage</code></b> </summary> <br/>

Simulate damage from a source to a target. If flat is true the damage can't be modified by status effects or artifacts. Returns the damage that would be dealt.

**Signature:**

```
simulate_deal_damage(source : guid, target : guid, damage : number, (optional) flat : boolean) -> number
```

</details>

## Player Operations

Functions that are related to the player.

### Globals

None

### Functions
<details> <summary><b><code>finish_player_turn</code></b> </summary> <br/>

Finishes the player turn.

**Signature:**

```
finish_player_turn() -> None
```

</details>

<details> <summary><b><code>give_player_gold</code></b> </summary> <br/>

Gives the player gold.

**Signature:**

```
give_player_gold(amount : number) -> None
```

</details>

<details> <summary><b><code>player_buy_artifact</code></b> </summary> <br/>

Let the player buy the artifact with the given id. This will deduct the price form the players gold and return true if the buy was successful.

**Signature:**

```
player_buy_artifact(card_id : type_id) -> boolean
```

</details>

<details> <summary><b><code>player_buy_card</code></b> </summary> <br/>

Let the player buy the card with the given id. This will deduct the price form the players gold and return true if the buy was successful.

**Signature:**

```
player_buy_card(card_id : type_id) -> boolean
```

</details>

<details> <summary><b><code>player_draw_card</code></b> </summary> <br/>

Let the player draw additional cards for this turn.

**Signature:**

```
player_draw_card(amount : number) -> None
```

</details>

<details> <summary><b><code>player_give_action_points</code></b> </summary> <br/>

Gives the player more action points for this turn.

**Signature:**

```
player_give_action_points(points : number) -> None
```

</details>

## Merchant Operations

Functions that are related to the merchant.

### Globals

None

### Functions
<details> <summary><b><code>add_merchant_artifact</code></b> </summary> <br/>

Adds another random artifact to the merchant

**Signature:**

```
add_merchant_artifact() -> None
```

</details>

<details> <summary><b><code>add_merchant_card</code></b> </summary> <br/>

Adds another random card to the merchant

**Signature:**

```
add_merchant_card() -> None
```

</details>

<details> <summary><b><code>get_merchant</code></b> </summary> <br/>

Returns the merchant state.

**Signature:**

```
get_merchant() -> merchant_state
```

</details>

<details> <summary><b><code>get_merchant_gold_max</code></b> </summary> <br/>

Returns the maximum value of artifacts and cards that the merchant will sell. Good to scale ``random_card`` and ``random_artifact``.

**Signature:**

```
get_merchant_gold_max() -> number
```

</details>

## Random Utility

Functions that help with random generation.

### Globals

None

### Functions
<details> <summary><b><code>gen_face</code></b> </summary> <br/>

Generates a random face.

**Signature:**

```
gen_face((optional) category : number) -> string
```

</details>

<details> <summary><b><code>random_artifact</code></b> </summary> <br/>

Returns the type id of a random artifact.

**Signature:**

```
random_artifact(max_price : number) -> type_id
```

</details>

<details> <summary><b><code>random_card</code></b> </summary> <br/>

Returns the type id of a random card.

**Signature:**

```
random_card(max_price : number) -> type_id
```

</details>

## Localization

Functions that help with localization.

### Globals

None

### Functions
<details> <summary><b><code>l</code></b> </summary> <br/>

Returns the localized string for the given key. Examples on locals definition can be found in `/assets/locals`. Example: ``
l('cards.MY_CARD.name', "English Default Name")``

**Signature:**

```
l(key : string, (optional) default : string) -> string
```

</details>

## Content Registry

These functions are used to define new content in the base game and in mods.

### Globals

None

### Functions
<details> <summary><b><code>delete_base_game</code></b> </summary> <br/>

Deletes all base game content. Useful if you don't want to include base game content in your mod.

```lua
delete_base_game() -- delete all base game content
delete_base_game("artifact") -- deletes all artifacts
delete_base_game("card") -- deletes all cards
delete_base_game("enemy") -- deletes all enemies
delete_base_game("event") -- deletes all events
delete_base_game("status_effect") -- deletes all status effects
delete_base_game("story_teller") -- deletes all story tellers

```

**Signature:**

```
delete_base_game((optional) type : string) -> None
```

</details>

<details> <summary><b><code>delete_card</code></b> </summary> <br/>

Deletes a card.

```lua
delete_card("SOME_CARD")
```

**Signature:**

```
delete_card(id : type_id) -> None
```

</details>

<details> <summary><b><code>delete_enemy</code></b> </summary> <br/>

Deletes an enemy.

```lua
delete_enemy("SOME_ENEMY")
```

**Signature:**

```
delete_enemy(id : type_id) -> None
```

</details>

<details> <summary><b><code>delete_event</code></b> </summary> <br/>

Deletes an event.

```lua
delete_event("SOME_EVENT")
```

**Signature:**

```
delete_event(id : type_id) -> None
```

</details>

<details> <summary><b><code>delete_status_effect</code></b> </summary> <br/>

Deletes a status effect.

```lua
delete_status_effect("SOME_STATUS_EFFECT")
```

**Signature:**

```
delete_status_effect(id : type_id) -> None
```

</details>

<details> <summary><b><code>delete_story_teller</code></b> </summary> <br/>

Deletes a story teller.

```lua
delete_story_teller("SOME_STORY_TELLER")
```

**Signature:**

```
delete_story_teller(id : type_id) -> None
```

</details>

<details> <summary><b><code>register_artifact</code></b> </summary> <br/>

Registers a new artifact.

```lua
register_artifact("REPULSION_STONE",
    {
        name = "Repulsion Stone",
        description = "For each damage taken heal for 2",
        price = 100,
        order = 0,
        callbacks = {
            on_damage = function(ctx)
                if ctx.target == ctx.owner then
                    heal(ctx.owner, 2)
                end
                return nil
            end,
        }
    }
)
```

**Signature:**

```
register_artifact(id : type_id, definition : artifact) -> None
```

</details>

<details> <summary><b><code>register_card</code></b> </summary> <br/>

Registers a new card.

```lua
register_card("MELEE_HIT",
    {
        name = "Melee Hit",
        description = "Use your bare hands to deal 5 (+3 for each upgrade) damage.",
        state = function(ctx)
            return "Use your bare hands to deal " .. highlight(5 + ctx.level * 3) .. " damage."
        end,
        max_level = 1,
        color = "#2f3e46",
        need_target = true,
        point_cost = 1,
        price = 30,
        callbacks = {
            on_cast = function(ctx)
                deal_damage(ctx.caster, ctx.target, 5 + ctx.level * 3)
                return nil
            end,
        }
    }
)
```

**Signature:**

```
register_card(id : type_id, definition : card) -> None
```

</details>

<details> <summary><b><code>register_enemy</code></b> </summary> <br/>

Registers a new enemy.

```lua
register_enemy("RUST_MITE",
    {
        name = "Rust Mite",
        description = "Loves to eat metal.",
        look = "/v\\",
        color = "#e6e65a",
        initial_hp = 22,
        max_hp = 22,
        gold = 10,
        callbacks = {
            on_turn = function(ctx)
                if ctx.round % 4 == 0 then
                    give_status_effect("RITUAL", ctx.guid)
                else
                    deal_damage(ctx.guid, PLAYER_ID, 6)
                end

                return nil
            end
        }
    }
)
```

**Signature:**

```
register_enemy(id : type_id, definition : enemy) -> None
```

</details>

<details> <summary><b><code>register_event</code></b> </summary> <br/>

Registers a new event.

```lua
register_event("SOME_EVENT",
	{
		name = "Event Name",
		description = "Flavor Text... Can include **Markdown** Syntax!",
		choices = {
			{
				description = "Go...",
				callback = function()
					-- If you return nil on_end will decide the next game state
					return nil 
				end
			},
			{
				description = "Other Option",
				callback = function() return GAME_STATE_FIGHT end
			}
		},
		on_enter = function()
			play_music("energetic_orthogonal_expansions")
	
			give_card("MELEE_HIT", PLAYER_ID)
			give_card("MELEE_HIT", PLAYER_ID)
			give_card("MELEE_HIT", PLAYER_ID)
			give_card("RUPTURE", PLAYER_ID)
			give_card("BLOCK", PLAYER_ID)
			give_artifact(get_random_artifact_type(150), PLAYER_ID)
		end,
		on_end = function(choice)
			-- Choice will be nil or the index of the choice taken
			return GAME_STATE_RANDOM
		end,
	}
)
```

**Signature:**

```
register_event(id : type_id, definition : event) -> None
```

</details>

<details> <summary><b><code>register_status_effect</code></b> </summary> <br/>

Registers a new status effect.

```lua
register_status_effect("BLOCK", {
    name = "Block",
    description = "Decreases incoming damage for each stack",
    look = "Blk",
    foreground = "#219ebc",
    state = function(ctx)
        return "Takes " .. highlight(ctx.stacks) .. " less damage"
    end,
    can_stack = true,
    decay = DECAY_ALL,
    rounds = 1,
    order = 100,
    callbacks = {
        on_damage_calc = function(ctx)
            if ctx.target == ctx.owner then
                add_status_effect_stacks(ctx.guid, -ctx.damage)
                return ctx.damage - ctx.stacks
            end
            return ctx.damage
        end
    }
})
```

**Signature:**

```
register_status_effect(id : type_id, definition : status_effect) -> None
```

</details>

<details> <summary><b><code>register_story_teller</code></b> </summary> <br/>

Registers a new story teller.

```lua
register_story_teller("STORY_TELLER_XYZ", {
    active = function(ctx)
        if not had_events_any({ "A", "B", "C" }) then
            return 1
        end
        return 0
    end,
    decide = function(ctx)
        local stage = get_stages_cleared()

        if stage >= 3 then
            set_event("SOME_EVENT")
            return GAME_STATE_EVENT
        end

        -- Fight against rust mites or clean bots
        local d = math.random(2)
        if d == 1 then
            add_actor_by_enemy("RUST_MITE")
        elseif d == 2 then
            add_actor_by_enemy("CLEAN_BOT")
        end

        return GAME_STATE_FIGHT
    end
})
```

**Signature:**

```
register_story_teller(id : type_id, definition : story_teller) -> None
```

</details>

