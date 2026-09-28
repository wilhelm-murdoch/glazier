#!/usr/bin/env bash
# Runs the glaze release-binary E2E sweep inside one container.
# ONLY=regex limits the test groups. The exit code is 1 when an assertion fails.
cd "$(dirname "$0")" || exit 1
source ./lib.sh
for f in t[0-9]*.sh; do source "./$f"; done

GROUPS_ALL=(t_cli_basics t_resolution t_structure t_base_index t_directories t_layouts t_focus
  t_size t_commands t_options_hooks t_envs t_variables t_idempotence t_down t_ls t_format
  t_save t_hostile_names t_sockets t_attach t_scale t_malformed t_extra t_extra2)

init_results
for grp in "${GROUPS_ALL[@]}"; do
  [[ -n ${ONLY:-} && ! $grp =~ $ONLY ]] && continue
  echo "== $grp"
  "$grp"
  end 2>/dev/null
done

echo
awk -F'\t' '{c[$3]++} END {printf "PASS=%d FAIL=%d INFO=%d\n", c["PASS"], c["FAIL"], c["INFO"]}' "$RES"
! grep -q $'\tFAIL\t' "$RES"
