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

`SPARK_DEBUG=1 wails dev` entoure les zones cliquables en pointillés jaunes.

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
internal/ai             LLM multimodal         (à venir)
internal/audio          STT / TTS              (à venir)
frontend/src            UI React + Tailwind
docs/ARCHITECTURE.md    choix techniques et feuille de route
```
