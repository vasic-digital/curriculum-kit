#!/usr/bin/env bash
#
# verify-curriculum.sh — the module's gate.
#
#   (no flag)         validate every document under testdata/good/ (and any
#                     paths given on the command line). Three-valued:
#                       0  every document is coherent
#                       1  at least one real finding
#                       2  could not determine — a toolchain is missing, a file
#                          is unreadable, or NOTHING WAS CHECKED. Never a pass.
#
#   --prove-failure   the §1.1 paired mutation proof. Every mutation is a DATA
#                     file under testdata/mutations/ with a sibling .expect
#                     naming the exit code and the rule code it must produce.
#                     No source is edited to run this. Exits 0 when every
#                     mutation was caught, 1 when one was not, 2 when the proof
#                     itself could not run.
#
# The gate is written so that its own vacuity is a finding: if the good-document
# set or the mutation set is EMPTY, that is an rc 2, not a quiet success. A
# check that iterates nothing and reports a pass is the defect this file exists
# to avoid, and it has shipped in sibling repositories.

set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"
BIN="$ROOT/bin/curriculum-validate"

OK=0; FAIL=1; UNDET=2

say()  { printf '%s\n' "$*"; }
pass() { printf 'PASS %s\n' "$*"; }
fail() { printf 'FAIL %s\n' "$*"; }
undet(){ printf 'UNDET %s\n' "$*"; }

build() {
  if ! command -v go >/dev/null 2>&1; then
    undet "go toolchain is not on PATH — nothing was checked"
    return $UNDET
  fi
  mkdir -p "$ROOT/bin" || return $UNDET
  if ! ( cd "$ROOT" && go build -o "$BIN" ./cmd/curriculum-validate ) ; then
    undet "the validator did not build — nothing was checked"
    return $UNDET
  fi
  return $OK
}

# ---------------------------------------------------------------- normal run

run_gate() {
  build || return $?

  local -a docs=()
  if [ "$#" -gt 0 ]; then
    docs=( "$@" )
  else
    while IFS= read -r f; do docs+=( "$f" ); done \
      < <(find "$ROOT/testdata/good" -maxdepth 1 -name '*.json' -type f 2>/dev/null | sort)
  fi

  if [ "${#docs[@]}" -eq 0 ]; then
    undet "no document to check — an empty input set is not a pass"
    return $UNDET
  fi

  local out rc
  out="$( "$BIN" "${docs[@]}" 2>&1 )"; rc=$?
  printf '%s\n' "$out"
  case "$rc" in
    0) pass  "${#docs[@]} document(s) coherent" ;;
    1) fail  "at least one document holds a finding" ;;
    *) undet "at least one document could not be judged" ;;
  esac
  return "$rc"
}

# ------------------------------------------------------------ mutation proof

# expect_field <file> <key>
expect_field() { sed -n "s/^$2=//p" "$1" | head -1; }

prove() {
  build || return $?

  local dir="$ROOT/testdata/mutations"
  local -a muts=()
  while IFS= read -r f; do muts+=( "$f" ); done \
    < <(find "$dir" -maxdepth 1 -name '*.json' -type f 2>/dev/null | sort)

  if [ "${#muts[@]}" -eq 0 ]; then
    undet "no mutation fixtures found under $dir — a proof that exercises nothing proves nothing"
    return $UNDET
  fi

  local n=0 passed=0 failed=0
  for m in "${muts[@]}"; do
    n=$((n+1))
    local base exp want_rc want_code out rc label
    base="${m%.json}"
    exp="$base.expect"
    label="M$n $(basename "$base")"

    if [ ! -f "$exp" ]; then
      fail "$label — no .expect sibling; the fixture states no expectation"
      failed=$((failed+1)); continue
    fi
    want_rc="$(expect_field "$exp" rc)"
    want_code="$(expect_field "$exp" code)"

    out="$( "$BIN" "$m" 2>&1 )"; rc=$?

    if [ "$rc" != "$want_rc" ]; then
      fail "$label — exit $rc, want $want_rc"
      printf '%s\n' "$out" | sed 's/^/      /'
      failed=$((failed+1)); continue
    fi
    # A mutation must be caught for the RIGHT REASON. Matching only the exit
    # code would let an unrelated defect in the fixture stand in for the one the
    # mutation was written to seed.
    if [ "$want_code" != "parse" ] && ! printf '%s' "$out" | grep -q "$want_code"; then
      fail "$label — exit $want_rc as wanted, but rule $want_code was not reported"
      printf '%s\n' "$out" | sed 's/^/      /'
      failed=$((failed+1)); continue
    fi
    pass "$label — rc $rc, $want_code"
    passed=$((passed+1))
  done

  # --- controls -------------------------------------------------------------
  # These assert the gate cannot pass on nothing. They are the reason the count
  # below is larger than the fixture count.

  n=$((n+1))
  out="$( "$BIN" 2>&1 )"; rc=$?
  if [ "$rc" = "2" ]; then
    pass "M$n control:no-input — validating nothing is rc 2, not rc 0"
    passed=$((passed+1))
  else
    fail "M$n control:no-input — exit $rc, want 2; the validator passed on an empty input set"
    failed=$((failed+1))
  fi

  n=$((n+1))
  local empty; empty="$(mktemp -d)"
  out="$( "$BIN" "$empty"/*.json 2>&1 )"; rc=$?
  rmdir "$empty" 2>/dev/null
  if [ "$rc" = "2" ]; then
    pass "M$n control:empty-dir — an unmatched glob is rc 2, not rc 0"
    passed=$((passed+1))
  else
    fail "M$n control:empty-dir — exit $rc, want 2"
    failed=$((failed+1))
  fi

  n=$((n+1))
  if [ -f "$ROOT/testdata/good/minimal.json" ]; then
    out="$( "$BIN" "$ROOT/testdata/good/minimal.json" 2>&1 )"; rc=$?
    if [ "$rc" = "0" ]; then
      pass "M$n control:good-still-passes — the detector is not simply always red"
      passed=$((passed+1))
    else
      fail "M$n control:good-still-passes — exit $rc, want 0; every mutation above proves nothing if the good document also fails"
      failed=$((failed+1))
    fi
  else
    undet "M$n control:good-still-passes — testdata/good/minimal.json is missing"
    failed=$((failed+1))
  fi

  say ""
  say "$passed passed / $failed failed / $n mutations"
  [ "$failed" -eq 0 ] && return $OK
  return $FAIL
}

# ---------------------------------------------------------------------- main

case "${1:-}" in
  --prove-failure) shift; prove "$@"; exit $? ;;
  --help|-h)       sed -n '2,30p' "${BASH_SOURCE[0]}"; exit 0 ;;
  *)               run_gate "$@"; exit $? ;;
esac
