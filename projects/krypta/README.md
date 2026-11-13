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

Gra startuje od **menu głównego**: Nowa gra, Wybór poziomu (Kryształowa
Krypta albo krótki Trening), Jak grać, Ustawienia, Wyjście.

## Sterowanie

| Klawisz | Akcja |
|---|---|
| `W` `A` `S` `D` | ruch |
| mysz | rozglądanie się (także góra/dół) |
| lewy przycisk myszy | strzał różdżką (25 obrażeń, strażnik ma 50 HP) |
| `Spacja` / `C` | skok / kucanie |
| `Enter` | zagraj ponownie (na ekranie końca gry) |
| `Esc` | pauza (Wznów / Restart / Ustawienia / Menu główne); na ekranie końca gry – menu główne |
| `F11` | pełny ekran |
| menu | strzałki lub WASD, Enter, Esc, mysz (klik, PPM zmniejsza wartość), pad (d-pad, A, B, Start) |
| `~` | konsola (np. `god`, `fps`, `noclip`, `restart`) |

## Ustawienia

Ekran ustawień pozwala zmienić czułość myszy, odwrócenie osi Y, głośność
(ogólną, muzyki i efektów), rozdzielczość, pełny ekran i pole widzenia. Zmiany
działają od razu i zapisują się w `projects/krypta/settings.aie` (obok gry,
plik jest ignorowany przez git). Usunięcie pliku przywraca domyślne wartości.

Pauza i menu zatrzymują czas gry: licznik czasu, timery skryptów i wrogów.

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
| `manifest.aie` | `RUNTIME level` włącza silnikowy runtime poziomów, `MENU menu.aie` – menu, `LEVEL` – lista poziomów; wskazuje skrypt i atlas strażnika |
| `menu.aie` | ekrany: tytułowy (tło: widok krypty), Jak grać, wybór poziomu, pauza, ustawienia (polskie etykiety), dźwięki i muzyka menu |
| `maps/trening.aie` | krótki poziom treningowy (3 kryształy, limit 2:00) |
| `config.aie` | klawisze (`BIND restart ENTER` itd.) |
| `maps/krypta.aie` | `GRID` (mapa 16×14), `PLACE` (kryształy, mikstury, strefy kwasu, strażnicy, brama, portal), `SOUND` (proceduralne dźwięki), `TRIGGER` (brama otwiera się przy `krysztaly >= 6`) |
| `scripts/krypta.lua` | logika wspólna dla obu poziomów (liczba kryształów i limit czasu przychodzą z mapy: `items_crystal`, `VAR_TIME_LIMIT`): licznik, czas, kwas, mikstury, wygrana/przegrana, HUD, muzyka |
| `assets/textures/*.png` | własne tekstury ścian 64×64 (zastępują proceduralne `brick/stone/metal/wood`) |
| `assets/sprites/*.png` | kryształ, mikstura, kwas, portal; `player/` – klatki ręki z różdżką |
| `straznik.png` | atlas 3×2 strażnika (idle, chód, atak) |
| `krypta_test.go`, `menu_test.go` | testy bez okna: mapa, osiągalność, wygrana, kwas, mikstura, śmierć, koniec czasu, trening, nawigacja menu, zapis ustawień, pauza zatrzymująca czas i timery |

Wszystkie grafiki są oryginalne, wygenerowane skryptem
`narzedzia/generuj_grafiki.py` (Pillow): `python -I projects/krypta/narzedzia/generuj_grafiki.py projects/krypta`.

### Testy

```powershell
go test ./projects/krypta/ ./level/
```

### Zrzuty ekranu bez grania

Silnik potrafi sam zrobić zrzut i zamknąć się (przydatne w CI / do dokumentacji):

```powershell
./bin/aiengine.exe --project krypta --screenshot krypta.png --frames 60 --exec "menu close; tp 13.5 5.4 -90; lua unlockDoor('brama')"
./bin/aiengine.exe --project krypta --screenshot ustawienia.png --frames 30 --exec "menu settings"
```

## Znane ograniczenia (wynikają z silnika)

- Brak oświetlenia punktowego i dźwięku przestrzennego; muzyka to proste proceduralne pętle.
- Strażnicy idą prosto do gracza (bez wyszukiwania ścieżki) i mogą się
  zablokować na rogu.
- Czcionka ma tylko wielkie litery (także polskie).
- Pad steruje menu i pauzą, ale nie ruchem gracza.
