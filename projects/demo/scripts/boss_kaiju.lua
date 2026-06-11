-- scripts/boss_kaiju.lua — Boss Kaiju encounter logic
-- Multi-phase boss fight with health-triggered behaviors.

local phase = 1

function onBossSpawn(id)
    changeMusic("boss_theme.ogg")
    showMessage("KAIJU PRZYBYWA!")
    shakeCamera(0.8, 2.0)
end

function onBossDamage(id, amount, part)
    if part == "HEAD" then
        playSound("head_hit.ogg")
        shakeCamera(0.3, 0.5)
    end

    local hp = getEnemyHP(id)

    if hp <= 3000 and phase == 1 then
        phase = 2
        showMessage("FAZA 2!")
        changeMusic("boss_theme_intense.ogg")
        setEnemyHP(id, 3000) -- cap at phase threshold
    end

    if hp <= 1000 and phase == 2 then
        phase = 3
        showMessage("FAZA OSTATECZNA!")
        spawnEnemy("grunt", 5.0, 3.0, 0.0)
        spawnEnemy("grunt", 6.0, 3.0, 0.0)
    end
end

function onBossDead(id)
    stopMusic()
    playSound("victory.ogg")
    shakeCamera(1.0, 3.0)
    showMessage("ZWYCIESTWO!")
    winGame()
end

registerEvent("onEnemySpawn", onBossSpawn)
registerEvent("onEnemyDamage", onBossDamage)
registerEvent("onEnemyDead", onBossDead)
