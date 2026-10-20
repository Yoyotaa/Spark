# Architecture de Spark

Spark est une surcouche (overlay) transparente qui voit l'écran et converse avec
l'utilisateur via A.R.I.A. Cibles : **macOS** et **Windows**.

## Vue d'ensemble

```
┌──────────────────────────── Frontend (React + Tailwind) ───────────────────────────┐
│  App.tsx ── AriaOrb / ChatPanel (éléments [data-interactive])                      │
│  overlay/regions.ts ── observe les zones cliquables → SetInteractiveRegions(rects) │
│  lib/bridge.ts ── appels Go typés (bindings Wails générés dans wailsjs/)           │
└──────────────────────────────────────┬─────────────────────────────────────────────┘
                                       │ Wails IPC (bindings + events)
┌──────────────────────────────────────┴───────────────── Backend (Go) ──────────────┐
│ internal/app      Pont Wails : cycle de vie, méthodes exposées au front            │
│ internal/overlay  Fenêtre transparente + click-through natif   ✅ POC              │
│ internal/vision   Capture d'écran (pool de buffers, JPEG, base64)   ✅             │
│ internal/ai       Fournisseur LLM multimodal, réponses streamées    ✅ Gemini      │
│ internal/audio    STT (commandes vocales) / TTS (voix d'A.R.I.A)    phase 4        │
│ internal/config   Réglages, secrets via variables d'environnement                  │
└────────────────────────────────────────────────────────────────────────────────────┘
```

## Overlay et click-through

**Problème.** Quand la fenêtre laisse passer les clics, le webview ne reçoit plus
aucun événement souris : il ne peut donc pas savoir que le curseur revient sur un
bouton.

**Solution.** Le frontend déclare ses zones cliquables, et Go interroge l'OS :

1. Tout élément avec `data-interactive` est suivi par `regions.ts`
   (`MutationObserver` + `ResizeObserver` + fin des transitions CSS). Les
   déclencheurs sont regroupés dans un seul `requestAnimationFrame`, et Go n'est
   appelé que si les rectangles ont changé.
2. `overlay.Controller` lit la position du curseur à ~60 Hz (un appel natif) et la
   teste contre les zones. Les zones sont échangées via un `atomic.Pointer`, donc
   la boucle chaude n'alloue rien et ne prend aucun verrou (0 alloc, test de zone
   en dizaines de ns).
3. L'état natif n'est modifié **que lorsqu'il change**.

| | macOS (`native_darwin.go`, cgo/Obj-C) | Windows (`native_windows.go`, Win32 pur, sans cgo) |
|---|---|---|
| Trouver la fenêtre | `[NSApp mainWindow]` | `FindWindowW("SparkOverlay")` |
| Click-through | `setIgnoresMouseEvents:` | style `WS_EX_TRANSPARENT` (+ `WS_EX_LAYERED`) |
| Position curseur | `mouseLocationOutsideOfEventStream` | `GetCursorPos` + `ScreenToClient` ÷ DPI |
| Au premier plan | `NSStatusWindowLevel`, tous les Spaces, apps plein écran | `HWND_TOPMOST`, `WS_EX_TOOLWINDOW` |
| Exclu des captures | `NSWindowSharingNone` | `WDA_EXCLUDEFROMCAPTURE` |
| Plein écran | frame de l'`NSScreen` | `rcMonitor` du moniteur |

Threads : AppKit n'est pas thread-safe, donc tout accès à la fenêtre sous macOS
passe par la file principale (main queue). `Shutdown` se contente d'annuler le
contexte et n'attend jamais la goroutine, pour éviter un interblocage avec le
thread principal.

Exclure la fenêtre des captures est essentiel : le module vision ne doit pas
« se voir » lui-même. Sur macOS récent, il faudra aussi exclure l'application
dans le `SCContentFilter` de ScreenCaptureKit.

## Interface

- **Identité** : étincelle à 4 branches dans un anneau HUD de trois arcs, dégradé
  teal → cyan → violet. Sources dans `build/brand/` ; `build/appicon.png` en
  dérive (Wails génère `.icns` et `.ico`). Polices Space Grotesk et JetBrains
  Mono embarquées (aucun appel réseau).
- **Sobriété GPU** : au repos, rien n'est animé. L'anneau ne tourne que quand
  A.R.I.A parle ou que le chat est ouvert. Toutes les animations ne touchent
  que `transform` et `opacity` (compositeur), et `prefers-reduced-motion` est
  respecté.
- **Drag** : pendant le glisser, l'ancre est déplacée directement dans le DOM
  (pas de rendu React par frame) et la fenêtre garde la souris
  (`SetMouseCaptured`) pour ne pas perdre le geste. La position est validée
  au lâcher puis persistée côté Go.
- **Accueil** : le frontend appelle `Boot()` une fois abonné aux événements
  (pas de course au démarrage) ; Go le joue une seule fois par processus
  (`sync.Once`, robuste au rechargement à chaud).

## Vision

Capture **à la demande** (bouton 👁 du chat), jamais en flux continu : c'est le
LLM qui limite le débit, pas la capture.

| | macOS | Windows |
|---|---|---|
| API | ScreenCaptureKit `SCScreenshotManager` (macOS 14+) | GDI `StretchBlt` (HALFTONE) |
| Réduction | faite par le système (GPU) à la taille cible | faite par GDI à la taille cible |
| JPEG | ImageIO (accéléré matériel) | `image/jpeg`, image RGBA réutilisée |
| Exclusion de Spark | `SCContentFilter` excluant notre processus | `WDA_EXCLUDEFROMCAPTURE` |
| Permission | Enregistrement de l'écran (TCC) | aucune |

- Côté longue ≤ 1568 px (au-delà, les LLM vision réduisent de toute façon),
  JPEG qualité 72 : ~175 Ko, ~110 ms à chaud sur un M3.
- Les octets arrivent dans un buffer `sync.Pool` (une seule copie depuis le
  natif) : une capture coûte 1 allocation de 84 o côté Go. `Frame.Release()` rend
  le buffer ; `AppendBase64` encode directement dans le buffer de l'appelant.
- La dernière capture est conservée côté Go (`App.lastFrame`) pour la requête
  IA de la phase 3 ; le frontend n'en reçoit qu'une data URL pour la miniature.
- Test réel optionnel : `SPARK_VISION_E2E=1 go test -run E2E -v ./internal/vision`.

## IA

```
question ─► capture (≈110-200 ms) ─► Gemini streamGenerateContent (SSE)
                                         │ tokens
                         ┌───────────────┴───────────────┐
                    EventDelta (UI)            découpe en phrases ─► TTS
```

- **Fournisseur** : interface `ai.Provider` ; implémentation Gemini en REST pur
  (pas de SDK), clé dans l'en-tête `x-goog-api-key`.
- **Modèle et latence** : `gemini-3.6-flash` par défaut, avec
  `thinking_config.thinking_level = low`. Mesures sur le niveau gratuit
  (oct. 2026), question + capture, premier mot :

  | Configuration | 1er mot | Tokens de réflexion |
  |---|---|---|
  | 3.6 Flash, réflexion par défaut | 5,5-10 s | 650-1000 |
  | **3.6 Flash, réflexion `low`** | **1,8-4,7 s** | ~110 |
  | 3.5 Flash-Lite (aucune réflexion) | 0,9 s | 0 |
  | 3.8 Flash | ~40 s (file d'attente) | — |

  Sur une capture, c'est la réflexion qui coûte, pas l'image : 1568 px ou
  1024 px donnent le même nombre de tokens (1116), on garde donc la pleine
  résolution (minimap, barres de vie). Un 503 « high demand » est retenté une
  fois ; un modèle qui refuse `thinking_config` est rappelé sans.
- **Test réel** : `SPARK_AI_LIVE=1 [SPARK_AI_LIVE_IMAGE=capture.jpg] go test
  -run Live -v ./internal/ai` (une requête, consomme le quota).
- **Vision dynamique** : chaque question part avec une capture prise à
  l'instant, sans action de l'utilisateur ; l'œil du chat coupe la vision.
- **Historique** : 12 tours max, en texte seul. Seule la question courante
  porte une image : le coût en tokens reste borné.
- **Voix au fil de l'eau** : la réponse est découpée en phrases (`ai.Sentences`,
  qui ne coupe pas « 3.5 » ni « A.R.I.A ») ; A.R.I.A lit la première pendant
  que la suite arrive.
- **Interruption** : une nouvelle question annule la précédente (requête HTTP,
  voix) et attend sa fin pour garder l'alternance user/model.
- **Test sans clé** : `SPARK_LLM_BASE_URL` pointe le client vers un faux
  serveur local ; `go test ./internal/ai` couvre SSE, erreurs et repli.

### Prochaine étape : la voix (phase 4)

Push-to-talk : une touche maintenue enregistre le micro, la capture est prise
à l'appui (la situation au moment où tu parles). Au relâchement, l'audio et
l'image partent **dans la même requête** : Gemini comprend directement la
parole, sans module de transcription séparé, ce qui économise un aller-retour.
Le mode « toujours à l'écoute » (Gemini Live API, voix native) viendra après :
il envoie le micro en continu, ce qui pose plus de questions de vie privée et
de quota.

## Pistes de performance déjà prévues
- **IA** : streaming des tokens vers l'interface **et** vers la synthèse vocale,
  phrase par phrase : A.R.I.A commence à parler avant la fin de la génération.
- **Audio** : push-to-talk d'abord (pas de micro ouvert en permanence), trames
  PCM transmises par channels.
- **React** : peu d'état global, animations en CSS (GPU) plutôt qu'en JS, aucun
  re-render lié au mouvement de la souris.

## Feuille de route

| Phase | Contenu | État |
|---|---|---|
| 1 | Fenêtre transparente + click-through natif mac/Windows | ✅ |
| 1.1 | Raccourcis globaux, focus clavier rendu à l'app précédente | ✅ |
| 1.2 | Identité visuelle, bouton déplaçable, accueil vocal | ✅ |
| 1.3 | Icône dans la barre de menus / zone de notification, réglages | |
| 2 | Vision : capture à la demande (ScreenCaptureKit / GDI), pool, JPEG | ✅ |
| 3 | IA : Gemini, capture à chaque question, streaming texte + voix | ✅ |
| 4 | Voix : push-to-talk, audio envoyé directement à Gemini | |
| 4.1 | Voix neuronale d'A.R.I.A (Gemini TTS ou Live API) | |
| 5 | Mode proactif, réglages, packaging (signature, installeur NSIS) | |

## Points d'attention connus

- **Focus clavier vs jeux** : la fenêtre doit pouvoir prendre le focus pour la
  saisie du chat, mais sans voler le focus au jeu le reste du temps. À traiter
  avec le raccourci global (phase 1.1), éventuellement `WS_EX_NOACTIVATE` hors
  chat.
- **Jeux en plein écran exclusif (Windows)** : aucun overlay de fenêtre ne
  s'affiche au-dessus d'un jeu en plein écran exclusif DirectX. Il faut jouer
  en mode « fenêtré sans bordure » (le cas par défaut de la plupart des jeux
  récents).
- **Multi-écran** : pour l'instant, l'overlay couvre l'écran sur lequel la
  fenêtre s'ouvre.
- **Permissions macOS** : la capture demande « Enregistrement de l'écran ». En
  dev, chaque build est signé ad hoc avec une nouvelle empreinte : macOS peut
  redemander l'autorisation après un rebuild (retirer puis rajouter Spark dans
  Réglages). Une signature stable (certificat Developer ID) règlera ça. Le micro
  (phase 4) demandera `NSMicrophoneUsageDescription` dans `Info.plist`.
- **macOS minimum** : 12.3 pour lancer l'app (ScreenCaptureKit est lié), 14 pour
  la capture.
