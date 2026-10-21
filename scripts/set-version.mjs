// Sets the version everywhere from a tag (v1.2.3): wails.json (Info.plist,
// Windows resources, installer) and prints the -ldflags for the Go binary.
// Usage: node scripts/set-version.mjs v1.2.3
import {readFileSync, writeFileSync} from 'node:fs'

const tag = process.argv[2] ?? ''
const version = tag.replace(/^v/, '')
if (!/^\d+\.\d+\.\d+([-.][0-9A-Za-z.]+)?$/.test(version)) {
  console.error(`version invalide : "${tag}" (attendu : v1.2.3)`)
  process.exit(1)
}
const config = JSON.parse(readFileSync('wails.json', 'utf8'))
config.info = {...config.info, productVersion: version}
writeFileSync('wails.json', JSON.stringify(config, null, 2) + '\n')
console.log(`-X spark/internal/app.Version=${version}`)
