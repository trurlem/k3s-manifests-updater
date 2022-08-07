#!/bin/bash

set -e

eval $(ssh-agent) > /dev/null
ssh-add ~/.ssh/id_ed25519

/usr/local/bin/k3s-manifests-updater
