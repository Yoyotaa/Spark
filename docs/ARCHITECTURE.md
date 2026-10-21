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
│ internal/ai       LLM multimodal streamé : Gemini, Claude, OpenAI-compatible  ✅      │
│ internal/audio    Micro, reconnaissance vocale, voix, lecteur PCM   ✅             │
│ internal/config   Paramètres persistants, clés API (trousseau), overrides d'env     │
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

## Paramètres

- **Modèle** : `config.Settings` (Go) est la source de vérité, reflétée en
  TypeScript (`frontend/src/lib/settings.ts`). Le fichier JSON est relu par-dessus
  `Defaults()` : un champ absent (ancienne version) garde sa valeur par défaut,
  puis `Normalize()` borne chaque valeur. Un fichier corrompu ne bloque jamais
  le démarrage.
- **Écriture atomique** (fichier temporaire + renommage) ; `Get()` renvoie une
  copie, les maps internes ne fuient jamais.
- **Application à chaud** : `App.apply` ne reconstruit que ce qui change
  (fournisseur IA, voix, raccourcis, vision, fréquence de survol, debug).
  Les raccourcis sont désinscrits de façon synchrone avant d'être réinscrits,
  ce qui permet de réutiliser la même combinaison.
- **UI** : édition optimiste et enregistrement automatique (anti-rebond de
  250 ms) ; Go renvoie les valeurs normalisées, qui font foi.
- **Clés API** : trousseau du système via `zalando/go-keyring` (sur macOS, le
  secret passe par l'entrée standard de `security -i`, jamais par la ligne de
  commande). L'interface n'en voit que la source et les 4 derniers caractères.
- **Raccourcis et dispositions** : les lettres suivent la disposition active ;
  sur macOS ⌥+lettre produit un caractère spécial dans le webview, la lettre
  est alors retrouvée côté Go depuis la touche physique (`KeyLabel`).

## IA

```
question ─► capture (≈110-200 ms) ─► Gemini streamGenerateContent (SSE)
                                         │ tokens
                         ┌───────────────┴───────────────┐
                    EventDelta (UI)            découpe en phrases ─► TTS
```

- **Fournisseurs** : interface `ai.Provider` (`Stream`, `ListModels`).
  Gemini en REST pur (clé dans l'en-tête `x-goog-api-key`) ; Claude via le SDK
  Go officiel (effort `low` par défaut, repli serveur `fallbacks: "default"`
  quand le modèle le permet, 1 nouvelle tentative max) ; OpenAI et compatibles
  via Chat Completions (`reasoning_effort` retiré si le modèle le refuse).
- **Consigne système** générée selon la langue, la longueur et les
  instructions personnalisées (`ai.SystemPrompt`).
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

## Micro et voix

```
touche ↓ / voix détectée ─► capture d'écran (en parallèle)
          │
          ▼ micro (AVAudioEngine | waveIn) ─► reconnaissance ─► texte ─► IA
                                              (Apple, en direct | Gemini)
```

- **Reconnaissance locale sur macOS** (SFSpeechRecognizer, sur l'appareil
  quand le modèle est installé) : transcription en direct, voix qui ne quitte
  pas le Mac, et texte utilisable par n'importe quel fournisseur (Claude
  n'accepte pas d'audio). Windows : waveIn natif, phrase transcrite par
  `gemini-3.5-flash-lite`.
- **Push-to-talk** : appui et relâchement détectés (événement Carbon
  `kEventHotKeyReleased` ; sous Windows, `GetAsyncKeyState` après le
  `WM_HOTKEY`). La capture est prise à l'appui, et l'appui interrompt A.R.I.A.
- **Écoute continue** : détection de voix sur le niveau RMS (seuil réglé par
  la sensibilité, 120 ms pour démarrer, 900 ms de silence pour finir), 0,5 s
  de pré-enregistrement pour ne pas couper « Aria », mot d'activation tolérant
  (Levenshtein, accents), détection ignorée pendant qu'A.R.I.A parle (+400 ms).
- **Threads** : le callback audio ne prend jamais de verrou bloquant
  (`TryLock`) et les résultats de reconnaissance sont traités dans des
  goroutines. Sans ça, arrêter le micro (qui attend le thread audio) depuis
  une section verrouillée pourrait s'interbloquer.
- **Voix neuronale** : API Interactions de Gemini (`gemini-3.8-flash-tts`)
  en SSE ; le PCM 24 kHz est joué dès le premier morceau (AVAudioPlayerNode /
  waveOut). Mesuré : premier son ~2,3 s, génération ~2× plus rapide que la
  lecture. Une seule requête par réponse (le niveau gratuit n'en permet que
  quelques-unes par minute) ; échec avant le premier son ⇒ voix système.
- Machine à états testée avec un faux micro et une fausse IA
  (`internal/app/voice_test.go`).

## Mode proactif

Toutes les N secondes (15 s à 10 min), si rien d'autre ne se passe (chat
fermé, pas de parole, pas de réponse en cours) : capture, comparaison de la
taille JPEG avec la précédente (écran inchangé ⇒ rien n'est envoyé), puis
requête avec une consigne « ne parle que si c'est important, sinon réponds
RAS ». Vérifié sur Gemini : écran neutre ⇒ « RAS » ; alerte batterie ⇒ une
phrase d'avertissement.

## Pistes de performance

- **Latence vocale** : la voix système démarre dès la première phrase
  streamée ; la voix neuronale attend la réponse complète (quota). Avec une
  clé payante, passer à une requête TTS par phrase en pipeline.
- **Écoute continue** : le micro reste ouvert (indicateur orange de macOS) ;
  le coût CPU est un calcul RMS par bloc de 1024 échantillons.
- **React** : peu d'état global, animations en CSS (GPU), niveau du micro et
  déplacement du bouton écrits directement dans le DOM (pas de rendu à 20 Hz).

## Feuille de route

| Phase | Contenu | État |
|---|---|---|
| 1 | Fenêtre transparente + click-through natif mac/Windows | ✅ |
| 1.1 | Raccourcis globaux, focus clavier rendu à l'app précédente | ✅ |
| 1.2 | Identité visuelle, bouton déplaçable, accueil vocal | ✅ |
| 1.3 | Page de paramètres complète, multi-fournisseurs, clés dans le trousseau | ✅ |
| 1.4 | Lancement à l'ouverture de session | ✅ |
| 1.5 | Icône dans la barre de menus / zone de notification | |
| 2 | Vision : capture à la demande (ScreenCaptureKit / GDI), pool, JPEG | ✅ |
| 3 | IA : Gemini, capture à chaque question, streaming texte + voix | ✅ |
| 4 | Micro : push-to-talk, écoute continue + mot d'activation | ✅ |
| 4.1 | Voix neuronale (Gemini TTS en streaming) | ✅ |
| 5 | Mode proactif | ✅ |
| 6 | Packaging : signature Developer ID, installeur NSIS, mises à jour | |
| 7 | Gemini Live API (conversation vocale temps réel, interruption à la voix) | |

## Points d'attention connus

- **Focus clavier vs jeux** : le chat et les paramètres prennent le focus
  et le rendent à l'app précédente à la fermeture ; le push-to-talk ne le
  prend jamais.
- **Jeux en plein écran exclusif (Windows)** : aucun overlay de fenêtre ne
  s'affiche au-dessus d'un jeu en plein écran exclusif DirectX. Il faut jouer
  en mode « fenêtré sans bordure » (le cas par défaut de la plupart des jeux
  récents).
- **Multi-écran** : pour l'instant, l'overlay couvre l'écran sur lequel la
  fenêtre s'ouvre.
- **Permissions macOS** : la capture demande « Enregistrement de l'écran ». En
  dev, chaque build est signé ad hoc avec une nouvelle empreinte : macOS peut
  redemander l'autorisation après un rebuild (retirer puis rajouter Spark dans
  Réglages). Une signature stable (certificat Developer ID) règlera ça. Micro
  et reconnaissance vocale sont déclarés dans `Info.plist`
  (`NSMicrophoneUsageDescription`, `NSSpeechRecognitionUsageDescription`).
- **Windows non testé sur machine réelle** : overlay, capture GDI, micro
  waveIn, lecteur waveOut et voix SAPI compilent et suivent la doc Win32, mais
  n'ont jamais tourné sur un PC. Premier test à faire.
- **macOS minimum** : 12.3 pour lancer l'app (ScreenCaptureKit est lié), 14 pour
  la capture.
