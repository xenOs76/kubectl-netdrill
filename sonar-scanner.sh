#!/usr/bin/env bash

export SONAR_TOKEN=$(cat /home/xeno/.config/sops-nix/secrets/sonar_token_kubectl-netdrill)

sonar-scanner -Dsonar.organization=xenos76 \
  -Dsonar.projectKey=xenOs76_kubectl-netdrill \
  -Dsonar.go.coverage.reportPaths=cover.out \
  -Dsonar.exclusions=completions/**,.devenv/**,.direnv/** \
  -D"sonar.tests=." \
  -D"sonar.test.inclusions=*_test.go,**/*_test.go"
