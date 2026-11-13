-- scripts/krypta.lua — logika gry "Kryształowa Krypta" (wspólna dla wszystkich poziomów)
-- Kryształy, czas, kwas, mikstury, klucze, sekrety, relikwie, punkty, boss,
-- wyjście z poziomu (exitLevel → kampania przechodzi do następnego poziomu).
--
-- Parametry poziomu przychodzą z mapy (silnik ustawia zmienne):
--   items_crystal  – liczba kryształów na mapie
--   time_limit     – limit czasu w sekundach (VAR_TIME_LIMIT)
--   music          – utwór tła (VAR_MUSIC)
--   boss, summon   – id bossa i co ile sekund przywołuje pająki (VAR_BOSS, VAR_SUMMON)
--   secrets_total  – liczba ukrytych przejść
-- Między poziomami przechodzą (CARRY w manifeście): HP, score i ekwipunek.

local TOTAL      = getVar("items_crystal") or 0
local TIME_LIMIT = getVar("time_limit") or 180
local BOSS       = getVar("boss")
local SUMMON     = getVar("summon") or 9
local SECRETS    = getVar("secrets_total") or 0
local ACID_DPS   = 20
local POTION_HP  = 35

local crystals  = 0
local secrets   = 0
local timeLeft  = TIME_LIMIT
local inAcid    = {}
local acidAcc   = 0
local flash     = 0
local bossAlive = BOSS ~= nil
local summonT   = SUMMON
local spawned   = 0

local KEY_NAMES = { klucz_czerwony = "czerwony klucz", klucz_niebieski = "niebieski klucz" }

local function clock(t)
    if t < 0 then t = 0 end
    return string.format("%d:%02d", math.floor(t / 60), math.floor(t % 60))
end

local function addScore(n)
    setVar("score", (getVar("score") or 0) + n)
end

local function updateHUD()
    if TOTAL > 0 then
        setHUDText("1_krysztaly", "Kryształy " .. crystals .. "/" .. TOTAL)
    elseif bossAlive then
        setHUDText("1_krysztaly", "Pokonaj Strażnika Krypty")
    else
        setHUDText("1_krysztaly", "Strażnik pokonany")
    end
    setHUDText("2_czas", "Czas " .. clock(timeLeft))
    setHUDText("3_punkty", "Punkty " .. math.floor(getVar("score") or 0))
    local keys = {}
    if hasItem("klucz_czerwony") then table.insert(keys, "czerwony") end
    if hasItem("klucz_niebieski") then table.insert(keys, "niebieski") end
    if #keys > 0 then
        setHUDText("4_klucze", "Klucze: " .. table.concat(keys, ", "))
    else
        setHUDText("4_klucze")
    end
    local relics = itemCount("relikwia")
    if relics > 0 or SECRETS > 0 then
        setHUDText("5_sekrety", "Sekrety " .. secrets .. "/" .. SECRETS .. "   Relikwie " .. relics)
    end
end

local function canLeave()
    return crystals >= TOTAL and not bossAlive
end

registerEvent("onStart", function()
    setVar("krysztaly", 0)
    if getVar("score") == nil then setVar("score", 0) end
    updateHUD()
    local music = getVar("music")
    if music then playMusic(music) end
    if TOTAL > 0 then
        showMessage("Zbierz " .. TOTAL .. " kryształów i uciekaj portalem", 5)
    else
        showMessage("Pokonaj Strażnika Krypty!", 5)
    end
end)

registerEvent("onItemPickup", function(id, kind)
    if kind == "crystal" then
        crystals = crystals + 1
        setVar("krysztaly", crystals)   -- trigger brama_otwarta w mapie
        addScore(100)
        playSound("ding")
        if crystals < TOTAL then
            showMessage("Kryształ " .. crystals .. "/" .. TOTAL, 2)
        end
    elseif kind == "potion" then
        setPlayerHP(getPlayerHP() + POTION_HP)
        playSound("heal")
        showMessage("+" .. POTION_HP .. " HP", 2)
    elseif KEY_NAMES[kind] then
        playSound("klucz")
        showMessage("Masz " .. KEY_NAMES[kind] .. "!", 3)
    elseif kind == "relikwia" then
        addScore(250)
        playSound("sekret")
        showMessage("Relikwia! +250 punktów", 3)
    end
    updateHUD()
end)

registerEvent("onDoorLocked", function(id, key)
    playSound("zamek")
    if key ~= "" then
        showMessage("Zamknięte - potrzebny " .. (KEY_NAMES[key] or key), 2)
    elseif BOSS then
        showMessage("Brama otworzy się po pokonaniu Strażnika", 2)
    else
        showMessage("Brama zamknięta runą - zbierz wszystkie kryształy", 2)
    end
end)

registerEvent("onSecretFound", function(id)
    secrets = secrets + 1
    addScore(100)
    playSound("sekret")
    showMessage("Sekret odkryty! (" .. secrets .. "/" .. SECRETS .. ")", 3)
    updateHUD()
end)

registerEvent("onEnemyDead", function(id)
    if id == BOSS then
        bossAlive = false
        addScore(1000)
    else
        addScore(50)
    end
    updateHUD()
end)

registerEvent("onZoneEnter", function(id, kind)
    if kind == "acid" then
        inAcid[id] = true
        playSound("sizzle")
    elseif kind == "exit" then
        if canLeave() then
            addScore(math.floor(timeLeft) * 2)
            exitLevel("next", "Ucieczka udana!\nCzas: " .. clock(TIME_LIMIT - timeLeft) .. "   HP: " .. math.floor(getPlayerHP()))
        else
            showMessage("Portal jest uśpiony", 2)
        end
    end
end)

registerEvent("onZoneExit", function(id, kind)
    if kind == "acid" then inAcid[id] = nil end
end)

registerEvent("onPlayerDamage", function(amount)
    flash = 0.7
end)

registerEvent("onPlayerDead", function()
    setFilter("red_flash", 0)
    setFilter("desaturate", 1)
    loseGame("Poległeś w krypcie\nKryształy: " .. crystals .. "/" .. TOTAL)
end)

registerEvent("onUpdate", function(dt)
    timeLeft = timeLeft - dt
    if timeLeft <= 0 then
        updateHUD()
        setFilter("desaturate", 1)
        loseGame("Koniec czasu\nKryształy: " .. crystals .. "/" .. TOTAL)
        return
    end

    -- Kwas zadaje obrażenia porcjami co 0.25 s.
    if next(inAcid) then
        acidAcc = acidAcc + dt
        if acidAcc >= 0.25 then
            acidAcc = acidAcc - 0.25
            setPlayerHP(getPlayerHP() - ACID_DPS * 0.25)
        end
    else
        acidAcc = 0
    end

    -- Boss przywołuje pająki (najwyżej 6 naraz przez całą walkę na fale).
    if bossAlive then
        summonT = summonT - dt
        if summonT <= 0 then
            summonT = SUMMON
            if spawned < 12 then
                local x, y = 4.5 + (spawned % 2) * 8, 5.5
                spawnEnemy("pajak", x, y)
                spawned = spawned + 1
                showMessage("Strażnik przywołuje pająka!", 2)
            end
        end
    end

    if flash > 0 then
        flash = flash - dt * 2.5
        if flash < 0 then flash = 0 end
        setFilter("red_flash", flash)
    end
    if timeLeft < 30 then
        setFilter("vignette", 0.5)
    end
    updateHUD()
end)
