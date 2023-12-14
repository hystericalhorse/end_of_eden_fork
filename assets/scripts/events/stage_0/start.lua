register_event("START", {
    name = "Waking up...",
    description = [[!!cryo_start.png

You wake up in a dimly lit room, the faint glow of a red emergency light casting an eerie hue over the surroundings. The air is musty and stale, the metallic scent of the cryo-chamber still lingering in your nostrils. You feel groggy and disoriented, your mind struggling to process what's happening.

As you try to sit up, you notice that your body is stiff and unresponsive. It takes a few moments for your muscles to warm up and regain their strength. Looking around, you see that the walls are made of a dull gray metal, covered in scratches and scuff marks. There's a faint humming sound coming from somewhere, indicating that the facility is still operational.

You try to remember how you ended up here, but your memories are hazy and fragmented. The last thing you recall is a blinding flash of light and a deafening boom. You must have been caught in one of the nuclear explosions that devastated the world.

As you struggle to gather your bearings, you notice a blinking panel on the wall, with the words *"Cryo Sleep Malfunction"* displayed in bold letters. It seems that the system has finally detected the error that caused your prolonged slumber and triggered your awakening.

**Shortly after you realize that you are not alone...**]],
    choices = {
        {
            description = "Try to escape the facility before it finds you...",
            callback = function()
                -- Try to escape
                if math.random() < 0.5 then
                    set_event(stage_1_init_events[math.random(#stage_1_init_events)])
                    return GAME_STATE_EVENT
                end

                -- Let OnEnd handle the state change
                return nil
            end
        }, {
            description = "Gather your strength and attack it!",
            callback = function()
                return nil
            end
        }
    },
    on_enter = function()
        play_music("energetic_orthogonal_expansions")

        -- Give the player it's start cards
        give_card("MELEE_HIT", PLAYER_ID)
        give_card("MELEE_HIT", PLAYER_ID)
        give_card("MELEE_HIT", PLAYER_ID)
        give_card("MELEE_HIT", PLAYER_ID)
        give_card("MELEE_HIT", PLAYER_ID)

        give_card("RUPTURE", PLAYER_ID)

        give_card("BLOCK", PLAYER_ID)
        give_card("BLOCK", PLAYER_ID)
        give_card("BLOCK", PLAYER_ID)

        give_artifact(get_random_artifact_type(150), PLAYER_ID)
    end,
    on_end = function()
        return GAME_STATE_RANDOM
    end
})