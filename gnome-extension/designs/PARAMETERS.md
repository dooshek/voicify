# Parametry designów - przewodnik

Każdy design to plik JSON w `gnome-extension/designs/`. Składa się z 3 sekcji:
**container** (pudełko), **bars** (słupki equalizera), **layers** (warstwy efektów).

---

## Container - pudełko/tło

To prostokąt (zaokrąglony) pod słupkami equalizera.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `borderRadius` | Zaokrąglenie rogów pudełka (px). 0 = ostre rogi, 40 = pigułka | `8` = lekko zaokrąglone |
| `bgColor` | Kolor tła pudełka [R, G, B]. | `[2, 8, 3]` = prawie czarny z zielonkawym odcieniem |
| `bgOpacity` | Przezroczystość tła. 1.0 = pełne krycie, 0.0 = niewidoczne | `0.92` = prawie nieprzezroczyste (8% prześwituje) |
| `defaultWidth` | Domyślna szerokość widgetu (%). Resetuje się przy zmianie designu | `100` = bazowa, `200` = podwójna |
| `defaultHeight` | Domyślna wysokość widgetu (%) | `100` = bazowa |

**Efekt bgOpacity**: Jeśli < 1.0, to tło prześwituje. Shadow (cień) rysowany jest POD pudełkiem, więc przy niskim opacity cień jest widoczny przez pudełko = brudny efekt. Przy 1.0 pudełko jest solidne i cień widać tylko na zewnątrz.

---

## Bars - słupki equalizera

To kolorowe słupki, które się animują w rytm głosu.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `borderRadius` | Zaokrąglenie rogów słupka. 0 = kwadratowe piksele, 4 = zaokrąglone | `0` = pikselowy retro look |
| `pivotY` | Punkt, od którego słupek rośnie. 0.0 = od góry, 0.5 = od środka, 1.0 = od dołu | `1.0` = rośnie w górę |
| `scaleMin` | Minimalna wysokość słupka (ułamek). Gdy cisza | `0.04` = prawie niewidoczny |
| `scaleMax` | Maksymalna wysokość słupka | `1.0` = pełna wysokość |
| `widthAdjust` | Korekta szerokości słupka (px). Dodaje/odejmuje od bazowej | `1` = trochę szerszy |

### Kolory słupków

| Parametr | Co robi |
|----------|---------|
| `colorOverride` | Wymusza stałe kolory zamiast theme. `null` = używaj theme | `{"center": [50,255,80], "edge": [20,160,40]}` |
| `colorMute` | Wyciszenie kolorów (0-1). Miesza z szarym | `0.25` = lekko przygaszony |

### Opacity słupków

| Parametr | Co robi |
|----------|---------|
| `opacityMode` | `"uniform"` = wszystkie tak samo, `"gradient"` = środek jaśniejszy, brzegi ciemniejsze |
| `opacityUniform` | Przy uniform: opacity 0-255 | `255` = pełne krycie |
| `opacityMin` / `opacityMax` | Przy gradient: zakres opacity | `120`-`255` |

### Glow (poświata) słupków

| Parametr | Co robi |
|----------|---------|
| `glowFromTheme` | `true` = poświata w kolorze theme, `false` = brak lub z `shadow` |
| `glowRadius` | Rozmiar poświaty (px) | `6` = widoczny glow |
| `glowAlpha` | Intensywność poświaty (0-1) | `0.7` = mocny |
| `shadow` | Alternatywa: ręczny CSS box-shadow | `"0 0 4px rgba(0,0,0,0.4)"` |
| `highlight` | `true` = jasna linia na górze słupka (efekt 3D) |

### Wave mode

| Parametr | Co robi |
|----------|---------|
| `waveMode` | `"wave"` = fala biegnąca od lewej do prawej. Bez tego = Gaussian envelope od środka |
| `waveSpeed` | Prędkość fali | `0.15` |

---

## Layers - warstwy efektów wizualnych

Layers to tablica `[]` efektów rysowanych w kolejności. Dwa typy:

### Warstwy Cairo (rysowane w layerPainter.js)

Rysowane bezpośrednio na canvas. Kolejność: shadow → tło → border → highlight → specular → innerShadow.

#### `shadow` - cień zewnętrzny

Ciemna poświata wokół pudełka. Rysowana POD pudełkiem.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `blur` | Rozmiar rozmycia cienia (px). Większy = bardziej rozlany | `24` = duży, miękki cień |
| `alpha` | Intensywność cienia (0-1). | `0.3` = widoczny, `0.1` = subtelny |
| `color` | Kolor cienia [R,G,B] | `[0,0,0]` = czarny |
| `x`, `y` | Przesunięcie cienia (px). y>0 = cień w dół | `"y": 4` = cień pod spodem |

**Uwaga**: Duży `blur` + wysoka `alpha` = cień zajmuje dużo miejsca i jest ciężki wizualnie. Na retro `blur: 24, alpha: 0.3` daje dużą poświatę.

#### `border` - obramowanie

Linia wokół pudełka.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `width` | Grubość obramowania (px) | `2` = widoczna ramka |
| `color` | Kolor [R,G,B] | `[255,255,255]` = biała |
| `source` | `"theme"` = kolor z aktualnego theme zamiast stałego | border w kolorze equalizera |
| `alpha` | Przezroczystość (0-1) | `0.3` = subtelna |
| `gradient` | `true` = alpha zanika od góry do dołu | frosted glass efekt |
| `alphaBottom` | Alpha na dole przy gradient | `0.10` |

#### `innerHighlight` - jasna krawędź u góry

Biały gradient od góry pudełka. Symuluje światło padające z góry.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `alpha` | Intensywność (0-1) | `0.15` = subtelny |
| `height` | Jaka część pudełka jest oświetlona (0-1) | `0.4` = górne 40% |

#### `specularHighlight` - eliptyczny blask

Owalna jasna plama (jak odbicie światła na szkle).

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `x`, `y` | Pozycja plamy (0-1, ułamek pudełka) | `0.5, 0.10` = góra-środek |
| `width`, `height` | Rozmiar plamy (0-1) | `0.8, 0.3` |
| `alpha` | Intensywność | `0.12` = ledwo widoczny |

#### `innerShadow` - wewnętrzny cień

Ciemnienie wzdłuż krawędzi wewnątrz pudełka. Efekt zagłębienia.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `blur` | Rozmiar (px) | `10` = miękki |
| `alpha` | Intensywność | `0.10` = subtelny |
| `color` | Kolor [R,G,B] | `[0,0,0]` = czarny |

### Warstwy Widget (tworzone w extension.js)

To osobne St.Widget nakładane na pudełko.

#### `pixelGrid` - siatka pikseli

Grid małych kwadratów (efekt CRT/retro). Kolory biorą z theme.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `position` | `"background"` = za słupkami, inaczej = przed | |
| `alpha` | Intensywność siatki | `0.12` = subtelna |
| `cellSize` | Rozmiar piksela (domyślnie = szerokość słupka) | |
| `cellGap` | Przerwa między pikselami (domyślnie = barSpacing) | |

#### `scanlines` - linie CRT

Poziome paski symulujące monitor CRT.

| Parametr | Co robi | Przykład |
|----------|---------|----------|
| `lineHeight` | Grubość linii (px) | `1` |
| `lineSpacing` | Przerwa między liniami (px) | `3` |
| `color` | Kolor [R,G,B] | `[0,0,0]` = czarny |
| `alpha` | Intensywność | `0.15` = subtelne |
| `borderRadius` | Zaokrąglenie kontenera scanlines | `8` |

#### `frame` - ramka dekoracyjna

Dodatkowe obramowanie (np. skeuomorphic look).

| Parametr | Co robi |
|----------|---------|
| `borderWidth` | Grubość |
| `borderRadius` | Zaokrąglenie |
| `color`, `alpha` | Kolor i przezroczystość |
| `inset` | Wcięcie od krawędzi pudełka |
| `shadow` | CSS box-shadow na ramce |

---

## Jak parametry się łączą - rendering pipeline

```
1. Shadow (cień)         ← rysowany NA DOLE, pod wszystkim
2. Background fill       ← bgColor + bgOpacity (jeśli <1.0, cień prześwituje!)
3. Border                ← obramowanie na krawędzi pudełka
4. Inner highlight       ← biały gradient u góry
5. Specular highlight    ← eliptyczny blask
6. Inner shadow          ← ciemnienie na krawędziach wewnątrz
7. Pixel grid widget     ← siatka pikseli (jeśli jest)
8. Trail bars            ← cienie słupków (modifier > 0)
9. Wave bars             ← główne słupki equalizera
10. Decorations          ← scanlines, frame itp.
```

## Typowe problemy i jak je naprawić

| Problem | Przyczyna | Fix |
|---------|-----------|-----|
| Cień prześwituje przez pudełko | `bgOpacity` < 1.0 | Zwiększ bgOpacity do 1.0 |
| Cień za duży/jasny | `shadow.blur` za duży lub `shadow.alpha` za wysoka | Zmniejsz blur i/lub alpha |
| Słupki wyglądają mdło | `glowAlpha` za niska lub `glowRadius` za mały | Zwiększ glow |
| Pudełko wydaje się "płaskie" | Brak innerHighlight/specularHighlight | Dodaj warstwy |
| Krawędzie nieostre | `bgOpacity` < 1.0 powoduje mieszanie z tłem | bgOpacity = 1.0 |
| Prostokątne rogi na blur | Shell.BlurEffect jest zawsze prostokątny | Nie używaj blur na borderRadius > 8 |
