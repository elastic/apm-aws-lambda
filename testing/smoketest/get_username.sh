#!/bin/bash -eu

# The gh command is faster and not as hacky as the fallback solution
if which gh >/dev/null; then
  user_name=$(gh api user -q ".login")
else
  user_name=$(ssh -T git@github.com 2>&1|cut -d'!' -f1|cut -d' ' -f2)
fi

echo "{\"user_name\": \"${user_name}\"}"
