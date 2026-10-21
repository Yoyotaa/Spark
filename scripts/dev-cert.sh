#!/bin/bash
# Creates the "Spark Dev" code-signing identity (self-signed, login keychain).
#
# Why: macOS ties privacy permissions (microphone, speech recognition, screen
# recording) to the app's signature. Development builds are signed ad hoc,
# with a new signature at every rebuild, so macOS asks again each time. Once
# this identity exists, Spark signs itself with it when launched from a
# terminal (see internal/selfresp): the signature, and the permissions, stay.
#
# Run once: ./scripts/dev-cert.sh   (macOS asks for your password to trust it)
set -euo pipefail

NAME="Spark Dev"
KEYCHAIN="$HOME/Library/Keychains/login.keychain-db"

if security find-identity -v -p codesigning | grep -q "\"$NAME\""; then
  echo "L'identité « $NAME » existe déjà et est valide."
  exit 0
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

cat > "$tmp/cfg" <<CFG
[req]
distinguished_name=dn
x509_extensions=ext
prompt=no
[dn]
CN=$NAME
[ext]
basicConstraints=critical,CA:false
keyUsage=critical,digitalSignature
extendedKeyUsage=critical,codeSigning
CFG

/usr/bin/openssl req -x509 -newkey rsa:2048 -nodes -days 3650 -config "$tmp/cfg" \
  -keyout "$tmp/key.pem" -out "$tmp/cert.pem" 2>/dev/null
/usr/bin/openssl pkcs12 -export -inkey "$tmp/key.pem" -in "$tmp/cert.pem" \
  -out "$tmp/id.p12" -passout pass:spark -name "$NAME" 2>/dev/null

security import "$tmp/id.p12" -k "$KEYCHAIN" -P spark -T /usr/bin/codesign >/dev/null
echo "Approbation du certificat pour la signature de code (mot de passe demandé)…"
security add-trusted-cert -p codeSign -k "$KEYCHAIN" "$tmp/cert.pem"

if security find-identity -v -p codesigning | grep -q "\"$NAME\""; then
  echo "OK : « $NAME » est prête. Relance « wails dev » : macOS demandera les"
  echo "autorisations une dernière fois, puis s'en souviendra."
else
  echo "L'identité n'est pas reconnue comme valide : vérifie l'approbation dans Trousseaux d'accès." >&2
  exit 1
fi
