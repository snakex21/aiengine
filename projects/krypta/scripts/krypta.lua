-- scripts/krypta.lua — logika gry "Krysztalowa Krypta"
-- Liczy krysztaly, odmierza czas, zadaje obrazenia od kwasu,
-- obsluguje mikstury, wygrana i przegrana.

local TOTAL      = 6     -- krysztalow do zebrania
local TIME_LIMIT = 180   -- sekund
local ACID_DPS   = 20    -- obrazen na sekunde w kwasie
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
    setHUDText("1_krysztaly", "KRYSZTALY " .. crystals .. "/" .. TOTAL)
    setHUDText("2_czas", "CZAS " .. clock(timeLeft))
    if crystals >= TOTAL then
        setHUDText("3_cel", "CEL: PORTAL ZA BRAMA")
    else
        setHUDText("3_cel", "CEL: ZBIERZ KRYSZTALY")
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
    showMessage("ZBIERZ " .. TOTAL .. " KRYSZTALOW I UCIEKNIJ PORTALEM", 5)
end)

registerEvent("onItemPickup", function(id, kind)
    if kind == "crystal" then
        crystals = crystals + 1
        setVar("krysztaly", crystals)   -- trigger brama_otwarta w mapie
        playSound("ding")
        if crystals < TOTAL then
            showMessage("KRYSZTAL " .. crystals .. "/" .. TOTAL, 2)
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
            winGame("UCIECZKA UDANA!\nCZAS: " .. clock(used) .. "   HP: " .. math.floor(getPlayerHP()))
        else
            showMessage("PORTAL JEST USPIONY", 2)
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
    loseGame("POLEGLES W KRYPCIE\nKRYSZTALY: " .. crystals .. "/" .. TOTAL)
end)

registerEvent("onUpdate", function(dt)
    timeLeft = timeLeft - dt
    if timeLeft <= 0 then
        updateHUD()
        setFilter("desaturate", 1)
        loseGame("KONIEC CZASU\nKRYSZTALY: " .. crystals .. "/" .. TOTAL)
        return
    end

    -- Kwas zadaje obrazenia porcjami co 0.25 s.
    if anyAcid() then
        acidAcc = acidAcc + dt
        if acidAcc >= 0.25 then
            acidAcc = acidAcc - 0.25
            setPlayerHP(getPlayerHP() - ACID_DPS * 0.25)
        end
    else
        acidAcc = 0
    end

    -- Wygaszanie czerwonego blysku po obrazeniach.
    if flash > 0 then
        flash = flash - dt * 2.5
        if flash < 0 then flash = 0 end
        setFilter("red_flash", flash)
    end

    -- Ostrzezenie o koncu czasu.
    if timeLeft < 30 then
        setFilter("vignette", 0.5)
    end
    updateHUD()
end)
