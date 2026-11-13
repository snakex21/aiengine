-- scripts/krypta.lua — logika gry "Kryształowa Krypta" (wspólna dla poziomów)
-- Liczy kryształy, odmierza czas, zadaje obrażenia od kwasu,
-- obsługuje mikstury, wygraną i przegraną.
--
-- Parametry poziomu przychodzą z mapy (silnik ustawia zmienne):
--   items_crystal  – liczba kryształów na mapie (PLACE item crystal ...)
--   time_limit     – limit czasu w sekundach (VAR_TIME_LIMIT)
--   music          – utwór tła (VAR_MUSIC, dźwięk SOUND z mapy)

local TOTAL      = getVar("items_crystal") or 6
local TIME_LIMIT = getVar("time_limit") or 180
local ACID_DPS   = 20    -- obrażeń na sekundę w kwasie
local POTION_HP  = 35

local crystals = 0
local timeLeft = TIME_LIMIT
local inAcid   = {}      -- id strefy -> true
local acidAcc  = 0
local flash    = 0

local function clock(t)
    if t < 0 then t = 0 end
    return string.format("%d:%02d", math.floor(t / 60), math.floor(t % 60))
end

local function updateHUD()
    setHUDText("1_krysztaly", "Kryształy " .. crystals .. "/" .. TOTAL)
    setHUDText("2_czas", "Czas " .. clock(timeLeft))
    if crystals >= TOTAL then
        setHUDText("3_cel", "Cel: portal za bramą")
    else
        setHUDText("3_cel", "Cel: zbierz kryształy")
    end
end

local function anyAcid()
    for _, v in pairs(inAcid) do
        if v then return true end
    end
    return false
end

registerEvent("onStart", function()
    setVar("krysztaly", 0)
    updateHUD()
    local music = getVar("music")
    if music then playMusic(music) end
    showMessage("Zbierz " .. TOTAL .. " kryształów i uciekaj portalem", 5)
end)

registerEvent("onItemPickup", function(id, kind)
    if kind == "crystal" then
        crystals = crystals + 1
        setVar("krysztaly", crystals)   -- trigger brama_otwarta w mapie
        playSound("ding")
        if crystals < TOTAL then
            showMessage("Kryształ " .. crystals .. "/" .. TOTAL, 2)
        end
    elseif kind == "potion" then
        setPlayerHP(getPlayerHP() + POTION_HP)
        playSound("heal")
        showMessage("+" .. POTION_HP .. " HP", 2)
    end
    updateHUD()
end)

registerEvent("onZoneEnter", function(id, kind)
    if kind == "acid" then
        inAcid[id] = true
        playSound("sizzle")
    elseif kind == "exit" then
        if crystals >= TOTAL then
            local used = TIME_LIMIT - timeLeft
            winGame("Ucieczka udana!\nCzas: " .. clock(used) .. "   HP: " .. math.floor(getPlayerHP()))
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
    if anyAcid() then
        acidAcc = acidAcc + dt
        if acidAcc >= 0.25 then
            acidAcc = acidAcc - 0.25
            setPlayerHP(getPlayerHP() - ACID_DPS * 0.25)
        end
    else
        acidAcc = 0
    end

    -- Wygaszanie czerwonego błysku po obrażeniach.
    if flash > 0 then
        flash = flash - dt * 2.5
        if flash < 0 then flash = 0 end
        setFilter("red_flash", flash)
    end

    -- Ostrzeżenie o końcu czasu.
    if timeLeft < 30 then
        setFilter("vignette", 0.5)
    end
    updateHUD()
end)
