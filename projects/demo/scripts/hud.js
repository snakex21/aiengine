// scripts/hud.js — HUD and UI logic for AIEngine
// Controls HP display, ammo counter, damage effects, and menus.

let kills = 0;

// Called when the game starts.
Engine.on("onStart", function() {
    console.log("HUD initialized.");
    UI.setHP(100, 100);
    UI.setAmmo(15, 30);
    UI.setCrosshair(1.0);
});

// Player takes damage — flash red and shake crosshair.
Engine.on("onPlayerDamage", function(amount) {
    UI.setFilter("red_flash", 0.8);
    setTimeout(function() {
        UI.setFilter("red_flash", 0.0);
    }, 100);

    let hp = Engine.getPlayerHP();
    UI.setHP(hp, 100);
    UI.setCrosshair(1.5); // spread crosshair
    setTimeout(function() {
        UI.setCrosshair(1.0);
    }, 200);
});

// Enemy killed — update counter.
Engine.on("onEnemyDead", function(id, enemyType) {
    kills++;
    console.log("Kill #" + kills + ": " + enemyType + " (" + id + ")");
});

// Low HP warning.
Engine.on("onPlayerDamage", function(amount) {
    let hp = Engine.getPlayerHP();
    if (hp <= 20) {
        UI.setFilter("vignette", 0.8);
        UI.setFilter("red_flash", 0.3);
        UI.setMessage("Ostrzezenie: niskie HP!");
    }
});

// Player death screen.
Engine.on("onPlayerDead", function() {
    UI.setFilter("vignette", 1.0);
    UI.setFilter("desaturate", 1.0);
    UI.setFilter("blur", 0.8);
    UI.showDeathScreen();
});

// Door opened.
Engine.on("onDoorOpen", function(id) {
    console.log("Door opened: " + id);
});

// Win game.
Engine.on("onWin", function() {
    UI.showWinScreen();
});

// --- Pause handling ---
let isPaused = false;

Engine.on("onPause", function(paused) {
    isPaused = paused;
    if (isPaused) {
        console.log("Game paused.");
        UI.showPauseMenu();
    } else {
        console.log("Game resumed.");
        UI.hideMenu();
    }
});
