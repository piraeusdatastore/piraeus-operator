#!/usr/bin/env sh

if [ -z "${1}" ] ; then
  echo "Expected SDK version"
  exit 1
fi

if [ ! -x "${HOME}/operator-sdk/bin/operator-sdk" ] ; then
  mkdir -p ${HOME}/operator-sdk/bin
  curl --output "${HOME}/operator-sdk/bin/operator-sdk" --location "https://github.com/operator-framework/operator-sdk/releases/download/${1}/operator-sdk-${1}-x86_64-linux-gnu"
  chmod +x "${HOME}/operator-sdk/bin/operator-sdk"
fi

echo "${HOME}/operator-sdk/bin/" >> "${GITHUB_PATH}"
