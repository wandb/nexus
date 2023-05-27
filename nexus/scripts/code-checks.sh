#/usr/bin/env bash

set -e
PROG="code-check.sh"

usage()
{
    echo "Usage: $PROG [COMMANDS] [OPTIONS]"
    echo "  COMMANDS:"
    echo "    check   - run hooks"
    echo "    install - install hooks"
    echo "  OPTIONS:"
    echo "    --all-files        - check all files (not just changed files)"
    echo "    --hook HOOK_ID     - specify hook to run"
    echo "    --skip HOOK_IDS    - specify hooks to skip (comma separated)"
    echo "    --stage HOOK_STAGE - specify hook stage"
}

PARAMS=""
CHECK=false
CHECK_ALL=false
NOCOMMAND=true
CHECK_HOOK=""
HOOK_STAGE="pre-push"

while (( "$#" )); do
  case "$1" in
    check)
      CHECK=true
      NOCOMMAND=false
      shift
      ;;
    install)
      pre-commit install -t pre-push
      NOCOMMAND=false
      shift
      ;;
    -a|--all-files)
      CHECK_ALL=true
      shift
      ;;
    --hook)
      CHECK_HOOK="$CHECK_HOOK $2"
      shift 2
      ;;
    --skip)
      export SKIP="$2"
      shift 2
      ;;
    --stage)
      HOOK_STAGE="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 2
      ;;
    -*|--*)
      echo "Error: Unsupported flag $1" >&2
      usage
      exit 1
      ;;
    *)
      echo "Error: Unknown command $1" >&2
      usage
      exit 1
      ;;
  esac
done

if $NOCOMMAND; then
  usage
  exit 1
fi

if $CHECK; then
  extra=""
  if $CHECK_ALL; then
      extra="--all-files"
  fi
  if [ ${#CHECK_HOOK} -gt 0 ]; then
    for hook in $CHECK_HOOK; do
      pre-commit run $hook --hook-stage $HOOK_STAGE $extra
    done
  else
    pre-commit run --hook-stage $HOOK_STAGE $extra
  fi
fi
