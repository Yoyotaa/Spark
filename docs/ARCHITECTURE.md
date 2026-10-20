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
│ internal/vision   Capture d'écran (pool de buffers, JPEG, base64)   phase 2        │
│ internal/ai       Fournisseur LLM multimodal, réponses streamées    phase 3        │
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

## Pistes de performance déjà prévues

- **Vision** : capture à la demande plutôt qu'un flux continu (c'est le LLM qui
  limite le débit). Image réduite (≤ 1280 px), encodée en JPEG dans des buffers
  `sync.Pool`, puis `base64.StdEncoding.Encode` dans un buffer réutilisé (pas de
  copie en `string`). `Frame.Release()` rend le buffer au pool.
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
| 2 | Vision : capture (ScreenCaptureKit / DXGI Desktop Duplication), pool, JPEG | |
| 3 | IA : fournisseur multimodal, streaming, persona A.R.I.A | |
| 4 | Audio : STT push-to-talk, TTS voix féminine | |
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
- **Permissions macOS** : la capture d'écran (phase 2) et le micro (phase 4)
  demanderont des autorisations : il faudra les déclarer dans `Info.plist`.
