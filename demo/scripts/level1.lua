-- scripts/level1.lua — Mission logic for Level 1
-- Handles zone clearing, boss spawn, and HP warnings.

local phase = 1

function onGameStart()
    showMessage("Oczysc wszystkie strefy!")
    setVar("kills", 0)
    setVar("zone", 1)
end

function onZone1Clear()
    showMessage("Strefa 1 oczyszczona! Znajdz bossa!")
    -- In a real game, this would call JS to add a minimap marker.
    unlockDoor("door_zone2")
    playSound("checkpoint.ogg")
end

function onEnemyDead(id, enemyType)
    local kills = getVar("kills")
    if kills == nil then kills = 0 end
    kills = kills + 1
    setVar("kills", kills)

    if kills >= 10 then
        triggerEvent("wave_clear")
    end
end

function onBossSpawn()
    changeMusic("boss_theme.ogg")
    showMessage("KAIJU PRZYBYWA!")
    shakeCamera(0.8, 2.0)
end

function onPlayerDamage(amount, part)
    if part == "HEAD" then
        playSound("head_hit.ogg")
    elseif part == "LEG_LEFT" or part == "LEG_RIGHT" then
        setPlayerSpeed(0.5)
    end
end

function onPlayerDead()
    stopMusic()
    playSound("death.ogg")
    showMessage("GAME OVER")
    loseGame()
end

-- Register events.
registerEvent("onStart", onGameStart)
registerEvent("onEnemyDead", onEnemyDead)
registerEvent("onPlayerDamage", onPlayerDamage)
registerEvent("onPlayerDead", onPlayerDead)
