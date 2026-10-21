<p align="center"><img src="build/appicon.png" width="128" alt="Spark"></p>

# Spark

Copilote visuel multimodal : une surcouche transparente qui voit ton écran et
dialogue avec toi, par texte ou à la voix, grâce à **A.R.I.A** (Augmented
Reality & Intelligence Agent). En jeu, au travail ou en navigation : tu
demandes « que dois-je faire ici ? », elle regarde et répond en quelques
secondes, sans interrompre ce que tu fais.

macOS 14+ et Windows 10/11 · [Wails v2](https://wails.io) (Go + React) ·
Gemini, Claude ou tout modèle compatible OpenAI.

## Installation

Télécharge la dernière version dans les
[Releases](https://github.com/Yoyotaa/Spark/releases) :

- **macOS** : `Spark-…-macOS.zip`, à décompresser et glisser dans
  Applications. L'app n'est pas encore signée par Apple : au premier
  lancement, fais **clic droit › Ouvrir** (ou
  `xattr -dr com.apple.quarantine /Applications/spark.app`).
- **Windows** : `Spark-…-Windows-setup.exe` (installeur) ou
  `…-portable.exe`. SmartScreen peut avertir : « Informations
  complémentaires › Exécuter quand même ».

Au premier lancement, Spark ouvre ses paramètres : choisis un fournisseur
d'IA et colle sa clé (Gemini en propose une gratuite sur
[aistudio.google.com](https://aistudio.google.com/apikey)).

## Utilisation

Spark vit dans la **barre des menus** (icône ✦, ou zone de notification sous
Windows) : parler à A.R.I.A, coup d'œil, masquer, paramètres, quitter.

| Raccourci (macOS / Windows) | Action |
|---|---|
| **F8** maintenu | push-to-talk : parle, relâche, A.R.I.A répond |
| ⌥⇧A / Alt+Shift+A | ouvrir / fermer le chat |
| ⌥⇧D / Alt+Shift+D | coup d'œil : A.R.I.A décrit l'écran et conseille |
| ⌥⇧H / Alt+Shift+H | masquer / afficher Spark |
| ⌥⇧P / Alt+Shift+P | paramètres |
| Échap | fermer le chat |

Tous les raccourcis sont réassignables. Le bouton A.R.I.A se déplace par
glisser-déposer.

- **Vision** : chaque question part avec une capture de l'écran prise à
  l'instant (Spark n'apparaît jamais dessus). L'œil du chat la coupe.
- **Voix** : push-to-talk ou écoute continue avec mot d'activation
  (« Aria, … »). Sur macOS, la reconnaissance vocale est celle d'Apple, en
  direct et sur l'appareil quand c'est possible.
- **Réponses** : affichées au fil de l'eau et lues à voix haute (voix système
  ou voix neuronale Gemini). Une nouvelle question ou F8 coupe A.R.I.A.
- **Mode proactif** (option) : A.R.I.A jette un œil de temps en temps et ne
  parle que si elle voit quelque chose d'important.
- **Fournisseurs** : Gemini, Claude, OpenAI et compatibles (OpenRouter,
  Mistral, Groq, Ollama, LM Studio…). Clés rangées dans le trousseau du
  système. Si Gemini est saturé, Spark bascule seul sur un modèle plus léger.

> ⚠️ Sur le niveau gratuit de Gemini, Google peut utiliser les requêtes
> (captures comprises) pour améliorer ses produits. Coupe la vision sur les
> écrans sensibles.

## Développement

Prérequis : Go ≥ 1.25, Node ≥ 20, Wails CLI
(`go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`).

```bash
wails dev
```

**Une fois, sur macOS** : `./scripts/dev-cert.sh` crée l'identité de
signature « Spark Dev ». Sans elle, chaque recompilation change la signature
de l'app et macOS redemande micro, reconnaissance vocale et capture d'écran.

| Variable | Effet |
|---|---|
| `SPARK_DEBUG=1` | journal détaillé, zones cliquables en pointillés jaunes |
| `SPARK_MUTE=1` | coupe la voix |
| `SPARK_LLM_MODEL` | force le modèle du fournisseur actif |
| `SPARK_LLM_BASE_URL` | force l'adresse de l'API (proxy, faux serveur) |
| `GEMINI_API_KEY`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` | clés de repli si aucune n'est dans le trousseau |

- UI seule dans un navigateur (faux bureau, événements simulés) :
  `npm --prefix frontend run dev`.
- Tests : `go test ./internal/...` ; appels réels optionnels :
  `SPARK_AI_LIVE=1 go test -run Live ./internal/ai`,
  `SPARK_VISION_E2E=1 go test -run E2E ./internal/vision`.

### Publier une version

```bash
git tag v0.2.0 && git push origin v0.2.0
```

La CI construit le `.app` universel (macOS), l'installeur et l'exécutable
portable (Windows), et crée la release (notes : `docs/releases/v0.2.0.md`).

## Structure

```
main.go                 options de fenêtre Wails, relance responsable (macOS)
internal/app            pont Wails ↔ modules : questions, voix, paramètres
internal/overlay        fenêtre transparente, click-through, auto-réparation
internal/vision         capture d'écran (ScreenCaptureKit / GDI)
internal/ai             Gemini, Claude, OpenAI-compatible, transcription, TTS
internal/audio          micro, reconnaissance vocale, voix, lecteur PCM
internal/hotkey         raccourcis globaux (push-to-talk compris)
internal/tray           icône de barre des menus / zone de notification
internal/config         paramètres, clés (trousseau), variables d'env
internal/selfresp       autorisations macOS au nom de Spark
internal/startup        lancement à l'ouverture de session
internal/update         vérification des nouvelles versions
frontend/src            interface React + Tailwind
docs/ARCHITECTURE.md    choix techniques, mesures, feuille de route
```
