# Kryształowa Krypta

Mała kampania FPP na silniku AIEngine (raycaster 2.5D): **5 poziomów**, każdy
na kilka minut (całość ok. 20–30 minut).

Jesteś złodziejem w krypcie. Na każdym poziomie zbierasz **kryształy**. Unikasz
**kałuż kwasu**, **strażników** i **pająków**. Szukasz **kluczy** do
zamkniętych drzwi i **sekretów** z relikwiami. Gdy runiczna **brama** się
otworzy, uciekasz przez **portal** na kolejny poziom. Na końcu czeka walka ze
**Strażnikiem Krypty**.

| # | Poziom | Motyw | Co nowego |
|---|---|---|---|
| 1 | Przedsionek | kamienna krypta | drzwi (E), czerwony klucz, pierwszy sekret, 4 kryształy |
| 2 | Kryształowa Krypta | kamienna krypta | kwas, strażnicy, 6 kryształów, limit 3:00 |
| 3 | Lodowe Groby | lód (własne tekstury ścian/podłogi/sufitu) | dwa klucze, szybkie pająki, ukryta nisza, 5 kryształów |
| 4 | Zalana Kopalnia | rdza i drewno | dużo kwasu, strażnicy w zasadzkach, sekretna komora, 6 kryształów |
| 5 | Serce Krypty | obsydian i czerwone kryształy | boss przywołujący pająki; brama otwiera się po jego śmierci |

Między poziomami przechodzą **HP, punkty i relikwie**. Postęp zapisuje się
automatycznie w `projects/krypta/save.aie` (obok gry). W menu pojawia się
wtedy **Kontynuuj**, a w **Wyborze poziomu** widać odblokowane poziomy i
rekordy czasu.

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

Gra startuje od **menu głównego**: Kontynuuj (gdy jest zapis), Nowa gra,
Wybór poziomu, Jak grać, Ustawienia, Wyjście. Każdy poziom zaczyna się od
ekranu z opisem (Enter – start), a kończy ekranem z czasem i rekordem. Potem
widać ekran ładowania następnego poziomu.

## Sterowanie

| Klawisz | Akcja |
|---|---|
| `W` `A` `S` `D` | ruch |
| mysz | rozglądanie się (także góra/dół) |
| lewy przycisk myszy | strzał: różdżka (świetlisty pocisk, 25 obrażeń, bez amunicji) albo kusza (bełt 50 obrażeń, amunicja) |
| `1` / `2`, kółko myszy, `Tab` / `Q` | wybór broni |
| `R` | przeładowanie kuszy |
| `E` | otwieranie drzwi (z kluczem) i sprawdzanie ścian (sekrety) |
| `Spacja` / `C` | skok / kucanie |
| `Enter` | zagraj ponownie (na ekranie końca gry) |
| `Esc` | pauza (Wznów / Restart / Ustawienia / Menu główne); na ekranie końca gry – menu główne |
| `F11` / `Alt+Enter` | pełny ekran (domyślnie bez ramki – Alt+Tab działa od razu) |
| menu | strzałki lub WASD, Enter, Esc, mysz (klik, PPM zmniejsza wartość), pad (d-pad, A, B, Start) |
| `~` | konsola (np. `god`, `fps`, `noclip`, `restart`) |

## Ustawienia

W ustawieniach zmienisz:
- czułość myszy i odwrócenie osi Y,
- głośność: ogólną, muzyki, efektów oraz „w tle” (gdy okno nie ma fokusu,
  domyślnie wyciszone),
- rozdzielczość: od 960×540 do 3840×2160, także 3440×1440 i „Natywna
  (monitor)”,
- skalę renderowania (50–200%; np. 4K przy 50–75% ma płynność 1080p/1440p),
- pełny ekran i jego tryb („Bez ramki” albo „Wyłączny”),
- pole widzenia (ustawiane dla 16:9; na ultrawide widać więcej na boki).

Zmiany działają od razu i zapisują się w `projects/krypta/settings.aie` (obok
gry, plik ignorowany przez git). Usunięcie pliku przywraca wartości domyślne.

Pauza, menu i ekrany między poziomami zatrzymują czas gry: licznik czasu,
timery skryptów i wrogów. Gdy przełączysz się na inne okno (Alt+Tab), gra sama
się pauzuje, zwalnia kursor i wycisza dźwięk.

## Zasady

- **Kryształy** (niebieskie, unoszą się) – zbierasz, wchodząc na nie. Licznik
  jest w lewym górnym rogu; po zebraniu wszystkich otwiera się **brama**, a za
  nią czeka **portal** na następny poziom.
- **Klucze** (czerwony, niebieski) otwierają drzwi w tym samym kolorze
  klawiszem E; klucz się przy tym zużywa.
- **Sekrety** – niektóre ściany da się otworzyć klawiszem E. Kryją
  **relikwie** (+250 punktów, przechodzą między poziomami).
- **Mikstury** (czerwone flaszki) – +35 HP.
- **Kwas** (zielone kałuże) – 20 HP/s, dopóki w nim stoisz.
- **Bronie** – różdżka (świetlisty pocisk, 25 obrażeń, bez limitu) i kusza
  z Kryształowej Krypty (bełt 50 obrażeń ze smugą; bełty leżą na poziomach
  i przechodzą między nimi razem z kuszą).
- **Przeciwnicy**:
  - **strażnicy** (fioletowe zjawy, 50–70 HP) gonią cię, gdy cię zobaczą;
    niektórzy pilnują posterunku, inni czekają w zasadzce;
  - **pająki** (20 HP) są szybkie i krążą po okolicy;
  - **upiór** (60 HP, Zalana Kopalnia) trzyma dystans i rzuca wybuchające
    ognistymi kulami;
  - wszyscy omijają ściany i narożniki (wyszukiwanie ścieżki A*).
- **Strażnik Krypty** (boss, 300 HP) co 9 s przywołuje pająka. Kryj się za
  filarami.
- **Punkty**: kryształ 100, wróg 50, sekret 100, relikwia 250, boss 1000 oraz
  premia za pozostały czas.
- **Przegrana**: HP spada do 0 albo kończy się czas. Enter powtarza poziom z
  takim stanem, z jakim na niego wszedłeś.

## Ruchome mechanizmy

- **Drzwi** we wszystkich poziomach przesuwają się w górę (E), cofają się,
  gdy ktoś stoi w przejściu; bramy runiczne otwierają się same po spełnieniu
  warunku.
- **Winda** w Przedsionku zawozi na wysoki podest z kryształem (E przy
  windzie albo stojąc na niej).
- **Zgniatacz** w południowym korytarzu Krypty miażdży co kilka sekund;
  dźwignia obok go zatrzymuje.
- **Wrota niszy** w Sanktuarium (sektor) otwierają się po zebraniu kryształów.
- **Animowana woda** w zalanej komorze kopalni i sadzawce sanktuarium,
  **lawa** w narożnikach Serca Krypty (`assets/textures/textures.aie`,
  klatki z `narzedzia/generuj_animacje.go`).
- Wrogowie mają animacje chodu, ataku, bólu i śmierci oraz 8 kątów widzenia
  (zastępcze rotacje generowane z atlasów).

## Jak to jest zrobione

Gra używa wyłącznie formatów silnika – nie ma w niej kodu Go poza testami:

| Plik | Zawartość |
|---|---|
| `manifest.aie` | `RUNTIME level`, `MENU menu.aie`, `CAMPAIGN yes`, `CARRY hp score items`, lista `LEVEL` (kolejność kampanii), polskie teksty ekranów `TEXT_*` |
| `menu.aie` | ekrany: tytułowy (tło: widok krypty), Kontynuuj, Jak grać, wybór poziomu, pauza, ustawienia (polskie etykiety), dźwięki i muzyka menu |
| `config.aie` | klawisze (`BIND interact E`, `BIND restart ENTER` itd.) |
| `maps/*.aie` | 5 poziomów: `GRID`, `PLACE` (kryształy, klucze z `give=`, drzwi z `key=`/`secret=yes`, strefy kwasu, typy wrogów `enemy_type`, wrogowie z `behavior=`, portal), `TEXTURES` (motyw), `INTRO`/`OUTRO`, `VAR_*`, `SOUND`, `TRIGGER` |
| `scripts/krypta.lua` | logika wspólna dla wszystkich poziomów: kryształy, czas, kwas, mikstury, klucze, sekrety, relikwie, punkty, przywoływanie pająków przez bossa, `exitLevel()` |
| `assets/textures/` | zestawy tekstur 64×64 (ściany, podłoga, sufit): domyślny (krypta), `lod/`, `kopalnia/`, `serce/` |
| `assets/sprites/` | kryształ, mikstura, kwas, portal, klucze, relikwia, atlasy `pajak.png`, `straznik_krypty.png` i `upior.png`, kusza i bełty (`kusza.png`, `belty.png`); `player/` – klatki ręki z różdżką i widok kuszy |
| `straznik.png` | atlas 3×2 strażnika (stoi, chód, atak) |
| `*_test.go` | testy bez okna (opis niżej) |

Wszystkie grafiki są oryginalne i wygenerowane skryptem
`narzedzia/generuj_grafiki.py` (Pillow), a mapy skryptem
`narzedzia/generuj_mapy.py`:

```powershell
python -I projects/krypta/narzedzia/generuj_grafiki.py projects/krypta
python -I projects/krypta/narzedzia/generuj_mapy.py projects/krypta
```

### Testy

```powershell
go test ./projects/krypta/ ./level/ ./progress/ ./nav/
```

Testy sprawdzają między innymi:
- czy każdy poziom da się przejść (klucze leżą przed swoimi drzwiami, portal
  jest dostępny dopiero po otwarciu bramy),
- przejście całej kampanii z prawdziwym skryptem Lua, przenoszeniem stanu i
  zapisem,
- przywoływanie pająków przez bossa,
- menu z „Kontynuuj” i zablokowanymi poziomami,
- pauzę zatrzymującą czas,
- zapis ustawień.

### Zrzuty ekranu bez grania

Silnik potrafi sam zrobić zrzut i zamknąć się (przydatne w CI / do dokumentacji):

```powershell
./bin/aiengine.exe --project krypta --screenshot poziom3.png --frames 40 --exec "level lodowe_groby play; tp 13.5 7.5 0"
./bin/aiengine.exe --project krypta --screenshot ladowanie.png --frames 12 --exec "level zalana_kopalnia play; lua exitLevel(); @5 advance"
./bin/aiengine.exe --project krypta --screenshot ustawienia.png --frames 30 --exec "menu settings"
```

## Znane ograniczenia (wynikają z silnika)

- Brak oświetlenia punktowego i dźwięku przestrzennego; muzyka to proste proceduralne pętle.
- Czcionka ma tylko wielkie litery (także polskie).
- Pad steruje menu i pauzą, ale nie ruchem gracza.
