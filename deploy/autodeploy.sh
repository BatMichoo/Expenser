#!/bin/sh
# Polls GitHub for new commits on the deploy branch. When a new commit has
# passed every CI check, fast-forwards the local checkout and rebuilds the
# docker compose stack. If the app does not come up healthy afterwards, rolls
# back to the previous commit and never retries the bad one. Never deploys
# commits with failing or pending CI, and never touches a checkout that has
# diverged from the remote branch.
set -eu

REPO="${AUTODEPLOY_REPO:-BatMichoo/Expenser}"
BRANCH="${AUTODEPLOY_BRANCH:-main}"
INTERVAL="${AUTODEPLOY_INTERVAL:-1800}"
SERVICES="${AUTODEPLOY_SERVICES:-db web nginx}"
HEALTH_PATH="${AUTODEPLOY_HEALTH_PATH:-/login}"
HEALTH_TIMEOUT="${AUTODEPLOY_HEALTH_TIMEOUT:-120}"
DRY_RUN="${AUTODEPLOY_DRY_RUN:-0}"
ONCE="${AUTODEPLOY_ONCE:-0}"

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') autodeploy: $*"; }

# Commits that failed CI or were rolled back. Stored in .git so the list
# survives deployer restarts and a bad commit is not retried in a loop.
skip_file() { echo "$(git rev-parse --git-dir)/autodeploy-skip"; }
is_skipped() { grep -qx "$1" "$(skip_file)" 2>/dev/null; }
mark_skipped() { echo "$1" >>"$(skip_file)"; }

# Prints success, pending or failure for the CI checks of a commit.
ci_status() {
	runs=$(curl -fsS -H "Accept: application/vnd.github+json" \
		"https://api.github.com/repos/$REPO/commits/$1/check-runs?per_page=100") || {
		echo pending
		return
	}
	# Only GitHub Actions jobs count, and only the latest attempt of each
	# (a successful re-run supersedes an earlier failure).
	echo "$runs" | jq -r '
		[.check_runs[] | select(.app.slug == "github-actions")]
		| group_by(.name) | map(max_by(.started_at)) as $r
		| if ($r | length) == 0 then "pending"
		  elif any($r[]; .status != "completed") then "pending"
		  elif all($r[]; .conclusion == "success" or .conclusion == "skipped" or .conclusion == "neutral") then "success"
		  else "failure" end'
}

# Rebuilds the stack for the checked-out commit. $1 is the commit that was
# running before, used to decide whether nginx needs a restart.
rebuild() {
	# shellcheck disable=SC2086 # SERVICES is a space-separated list
	if ! docker compose up -d --build $SERVICES; then
		log "docker compose up failed"
		return 1
	fi
	# Config files mounted into nginx are not picked up by a rebuild.
	if git diff --name-only "$1" HEAD -- nginx www | grep -q .; then
		log "nginx files changed, restarting nginx"
		docker compose restart nginx || return 1
	fi
}

# Waits until the web app answers HEALTH_PATH with a success status.
healthy() {
	deadline=$(($(date +%s) + HEALTH_TIMEOUT))
	while [ "$(date +%s)" -lt "$deadline" ]; do
		if docker compose exec -T web sh -c \
			"wget -q -O /dev/null \"http://127.0.0.1:\${APP_PORT:-8080}$HEALTH_PATH\"" \
			>/dev/null 2>&1; then
			return 0
		fi
		sleep 5
	done
	return 1
}

deploy() {
	old="$1"
	new="$2"

	if ! git merge --ff-only --quiet "$new"; then
		log "fast-forward to ${new} failed (local changes?); will retry"
		return 2
	fi

	if [ "$DRY_RUN" = "1" ]; then
		log "dry run: would rebuild: ${SERVICES}"
		return 0
	fi

	if rebuild "$old" && healthy; then
		if git diff --name-only "$old" "$new" -- deploy docker-compose.yaml | grep -q .; then
			log "deployer files changed; run 'make autodeploy' on this machine to update the deployer itself"
		fi
		docker image prune -f >/dev/null
		return 0
	fi

	log "${new} is unhealthy, rolling back to ${old}"
	docker compose logs --tail 30 web || true
	if ! git reset --keep "$old"; then
		log "ROLLBACK FAILED: could not check out ${old}; fix the checkout by hand"
		return 1
	fi
	if rebuild "$new" && healthy; then
		log "rolled back to ${old}"
	else
		log "ROLLBACK FAILED: ${old} is not healthy either; check 'docker compose logs web'"
	fi
	return 1
}

main() {
	git config --global --add safe.directory "$PWD"
	log "watching ${REPO}@${BRANCH} every ${INTERVAL}s"

	while :; do
		current_branch=$(git symbolic-ref --short HEAD 2>/dev/null || echo detached)
		if [ "$current_branch" != "$BRANCH" ]; then
			log "checkout is on '${current_branch}', not '${BRANCH}'; waiting"
		elif ! git fetch --quiet "https://github.com/${REPO}.git" "$BRANCH"; then
			log "fetch failed; will retry"
		else
			local_sha=$(git rev-parse HEAD)
			remote_sha=$(git rev-parse FETCH_HEAD)

			if [ "$local_sha" != "$remote_sha" ] && ! is_skipped "$remote_sha"; then
				if ! git merge-base --is-ancestor "$local_sha" "$remote_sha"; then
					log "local ${BRANCH} has diverged from remote; not deploying"
				else
					case "$(ci_status "$remote_sha")" in
					success)
						log "deploying ${remote_sha} (was ${local_sha})"
						rc=0
						deploy "$local_sha" "$remote_sha" || rc=$?
						case $rc in
						0) log "deployed ${remote_sha}" ;;
						1) mark_skipped "$remote_sha" ;;
						esac
						;;
					failure)
						log "CI failed for ${remote_sha}; not deploying"
						mark_skipped "$remote_sha"
						;;
					*)
						log "CI still running for ${remote_sha}; waiting"
						;;
					esac
				fi
			fi
		fi

		[ "$ONCE" = "1" ] && exit 0
		sleep "$INTERVAL"
	done
}

# AUTODEPLOY_NO_MAIN=1 loads the functions without starting the loop (tests).
[ "${AUTODEPLOY_NO_MAIN:-0}" = "1" ] || main
