# Kryształowa Krypta

Mała, kompletna gra FPP na silniku AIEngine (raycaster 2.5D). Jedna rozgrywka
trwa 2–5 minut.

Jesteś złodziejem zamkniętym w krypcie. Zbierz **6 kryształów**, omijaj
**kałuże kwasu** i **strażników**, a gdy runiczna **brama** się otworzy,
uciekaj przez **portal**. Masz na to **3 minuty**.

## Uruchomienie

Jedna komenda z katalogu głównego repozytorium (zbuduje silnik, jeśli trzeba):

```powershell
powershell -ExecutionPolicy Bypass -File projects\krypta\graj.ps1
```

albo ręcznie:

```powershell
go build -o bin/aiengine.exe .
./bin/aiengine.exe --project krypta
```

Wymagania jak dla silnika: Go 1.26+, Windows 10/11, OpenGL 3.3.

## Sterowanie

| Klawisz | Akcja |
|---|---|
| `W` `A` `S` `D` | ruch |
| mysz | rozglądanie się (także góra/dół) |
| lewy przycisk myszy | strzał różdżką (25 obrażeń, strażnik ma 50 HP) |
| `Spacja` / `C` | skok / kucanie |
| `Enter` | zagraj ponownie (na ekranie końca gry) |
| `Esc` | pauza |
| `F11` | pełny ekran |
| `~` | konsola (np. `god`, `fps`, `noclip`, `restart`) |

## Zasady

- **Kryształy** (niebieskie, unoszą się) – zbierasz wchodząc na nie. Licznik
  `KRYSZTALY x/6` jest w lewym górnym rogu.
- **Mikstury** (czerwone flaszki) – +35 HP.
- **Kwas** (zielone kałuże) – 20 HP/s, dopóki w nim stoisz. Kałuża w dolnym
  korytarzu to skrót, który kosztuje zdrowie.
- **Strażnicy** (fioletowe zjawy) – gonią cię, gdy cię widzą (5 pól, nie widzą
  przez ściany), zadają 10 obrażeń co sekundę z bliska. Można ich zestrzelić.
- **Brama** (drewniane wrota z runą) otwiera się po zebraniu 6 kryształów;
  za nią czeka **portal** – wejście = wygrana.
- **Przegrana**: HP spada do 0 albo kończy się czas.

## Jak to jest zrobione

Gra używa wyłącznie formatów silnika – nie ma w niej kodu Go poza testami:

| Plik | Zawartość |
|---|---|
| `manifest.aie` | `RUNTIME level` włącza silnikowy runtime poziomów; wskazuje mapę, skrypt, atlas strażnika |
| `config.aie` | klawisze (`BIND restart ENTER` itd.) |
| `maps/krypta.aie` | `GRID` (mapa 16×14), `PLACE` (kryształy, mikstury, strefy kwasu, strażnicy, brama, portal), `SOUND` (proceduralne dźwięki), `TRIGGER` (brama otwiera się przy `krysztaly >= 6`) |
| `scripts/krypta.lua` | logika: licznik, czas, obrażenia od kwasu, mikstury, wygrana/przegrana, HUD (`setHUDText`) |
| `assets/textures/*.png` | własne tekstury ścian 64×64 (zastępują proceduralne `brick/stone/metal/wood`) |
| `assets/sprites/*.png` | kryształ, mikstura, kwas, portal; `player/` – klatki ręki z różdżką |
| `straznik.png` | atlas 3×2 strażnika (idle, chód, atak) |
| `krypta_test.go` | testy bez okna: mapa się wczytuje, wszystko jest osiągalne, wygrana, kwas, mikstura, śmierć, koniec czasu |

Wszystkie grafiki są oryginalne, wygenerowane skryptem
`narzedzia/generuj_grafiki.py` (Pillow): `python -I projects/krypta/narzedzia/generuj_grafiki.py projects/krypta`.

### Testy

```powershell
go test ./projects/krypta/ ./level/
```

### Zrzuty ekranu bez grania

Silnik potrafi sam zrobić zrzut i zamknąć się (przydatne w CI / do dokumentacji):

```powershell
./bin/aiengine.exe --project krypta --screenshot krypta.png --frames 60 --exec "tp 13.5 5.4 -90; lua unlockDoor('brama')"
```

## Znane ograniczenia (wynikają z silnika)

- Czcionka HUD ma tylko ASCII, więc teksty w grze są bez polskich znaków.
- Brak oświetlenia punktowego, muzyki i dźwięku przestrzennego.
- Strażnicy idą prosto do gracza (bez wyszukiwania ścieżki) i mogą się
  zablokować na rogu.
- Sprite'y (kryształy, wrogowie) rysują się na wierzchu napisów HUD.
