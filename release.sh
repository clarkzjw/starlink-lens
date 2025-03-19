#!/usr/bin/env bash

CF_R2_BUCKET=$(printenv CF_R2_BUCKET)
CF_KEY_ID=$(printenv CF_KEY_ID)
CF_KEY_SECRET=$(printenv CF_KEY_SECRET)
CF_ACCOUNT_ID=$(printenv CF_ACCOUNT_ID)
CF_RELEASE_TAG=$(printenv CF_RELEASE_TAG)
GH_RELEASE_TAG=$(echo $CF_RELEASE_TAG | cut -c 2-)
GPG_PRIVATE_KEY=$(printenv GPG_PRIVATE_KEY)
GPG_PUBLIC_KEY=$(printenv GPG_PUBLIC_KEY)
GPG_KEY_ID=$(printenv GPG_KEY_ID)
PROJECT="lens"

echo -n "$GPG_PRIVATE_KEY" | base64 --decode | gpg2 --import

poetry run python3 release_pkgs.py --bucket $CF_R2_BUCKET --id $CF_KEY_ID --secret $CF_KEY_SECRET --account $CF_ACCOUNT_ID --binary $PROJECT --release-tag $GH_RELEASE_TAG --gpg-key-id $GPG_KEY_ID
