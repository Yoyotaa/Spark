<p align="center"><img src="build/appicon.png" width="128" alt="Spark"></p>

# Spark

Copilote visuel multimodal : une surcouche transparente qui voit ton écran et
dialogue avec toi, par texte ou à la voix, grâce à **A.R.I.A** (Augmented
Reality & Intelligence Agent).

Stack : [Wails v2](https://wails.io) (Go + React/TypeScript), Tailwind CSS v4.
Plateformes : macOS, Windows.

## Prérequis

- Go ≥ 1.25
- Node ≥ 20
- Wails CLI : `go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`
- Windows : WebView2 (inclus dans Windows 10/11)

## Développement

```bash
wails dev
```

Tout se règle dans la **page de paramètres** (roue dentée du chat ou ⌥⇧P) :
fournisseur d'IA et clé, modèle, réflexion, langue, voix, vision, micro,
raccourcis, apparence… Les changements s'appliquent immédiatement.

Variables d'environnement utiles en développement (elles ne remplacent pas les
paramètres, sauf mention) :

| Variable | Effet |
|---|---|
| `SPARK_DEBUG=1` | logs détaillés, zones cliquables en pointillés jaunes |
| `SPARK_MUTE=1` | coupe la voix quels que soient les paramètres |
| `SPARK_LLM_MODEL` | force le modèle du fournisseur actif |
| `SPARK_LLM_BASE_URL` | force l'adresse de l'API (proxy, faux serveur de test) |
| `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` | clés de repli si aucune n'est dans le trousseau |

Itérer sur l'UI dans un navigateur (faux fond d'écran, accueil simulé) :
`npm --prefix frontend run dev` puis http://localhost:5173 (`?debug` pour les zones).

### IA

Trois fournisseurs, au choix dans Paramètres › Intelligence :

| Fournisseur | Clé | Remarque |
|---|---|---|
| Gemini | [Google AI Studio](https://aistudio.google.com/apikey) (gratuite) | rapide ; défaut `gemini-3.6-flash` |
| Claude | [console.anthropic.com](https://console.anthropic.com) | analyse la plus fine ; défaut `claude-opus-5-5` |
| OpenAI & compatibles | [platform.openai.com](https://platform.openai.com/api-keys) | aussi OpenRouter, Mistral, Groq, Ollama, LM Studio… via l'adresse de l'API |

Les clés sont rangées dans le **trousseau du système** (Trousseau macOS,
Gestionnaire d'identification Windows), jamais en clair sur le disque. Le
bouton « Lister » récupère les modèles de ton compte, « Tester » mesure la
latence.

Chaque question part avec une capture prise à l'instant (œil du chat pour
couper la vision). ⚠️ Sur le niveau gratuit de Gemini, Google peut utiliser
les requêtes (captures comprises) pour améliorer ses produits.

### Parler à A.R.I.A

- **Push-to-talk** (défaut, touche F8) : maintiens, parle, relâche. La capture
  d'écran est prise à l'appui ; appuyer coupe A.R.I.A si elle parle.
- **Écoute continue** : « Aria, que dois-je faire ? ». Le mot d'activation
  évite qu'elle réagisse aux discussions (Discord…).
- macOS : reconnaissance vocale d'Apple, en direct et sur l'appareil quand
  c'est possible. Windows : transcription par Gemini (clé Gemini requise).

### Voix

Voix système (instantanée) ou **voix neuronale Gemini** (naturelle, ~2,5 s de
latence en plus, quota gratuit limité, repli automatique sur la voix système).

### Mode proactif

Option : A.R.I.A regarde l'écran à intervalle régulier et ne parle que si elle
voit quelque chose d'important. Désactivé par défaut (consomme du quota).

### Raccourcis

Tous réassignables dans les paramètres.

| | |
|---|---|
| ⌥⇧A / Alt+Shift+A | ouvrir/fermer A.R.I.A (prend puis rend le focus clavier) |
| ⌥⇧H / Alt+Shift+H | masquer/afficher Spark |
| ⌥⇧P / Alt+Shift+P | paramètres |
| Échap | fermer le chat |

Le bouton se déplace par glisser-déposer ; sa position est mémorisée
(`~/Library/Application Support/Spark/settings.json`, `%AppData%\Spark` sous Windows).

## Build

```bash
wails build                           # plateforme courante
wails build -platform windows/amd64   # .exe Windows, cross-compilé depuis macOS
```

La CI GitHub (`.github/workflows/build.yml`) produit les builds macOS et Windows
à chaque push.

## Structure

```
main.go                 options de fenêtre Wails (frameless, transparente)
internal/app            pont Wails ↔ modules
internal/overlay        click-through natif (darwin / windows)
internal/vision         capture d'écran        (à venir)
internal/ai             LLM multimodal : Gemini (REST), Claude (SDK officiel), OpenAI-compatible
internal/audio          micro, reconnaissance vocale, voix système, lecteur PCM natif
internal/hotkey         raccourcis globaux (Carbon / RegisterHotKey)
internal/config         paramètres persistants, clés (trousseau), variables d'env
internal/startup        lancement à l'ouverture de session
frontend/src            UI React + Tailwind
build/brand             logo et icône (SVG sources)
docs/ARCHITECTURE.md    choix techniques et feuille de route
```
