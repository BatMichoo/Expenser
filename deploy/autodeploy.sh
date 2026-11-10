#!/bin/sh
# Polls GitHub for new commits on the deploy branch. When a new commit has
# passed every CI check, fast-forwards the local checkout and rebuilds the
# docker compose stack. Never deploys commits with failing or pending CI,
# and never touches a checkout that has diverged from the remote branch.
set -eu

REPO="${AUTODEPLOY_REPO:-BatMichoo/Expenser}"
BRANCH="${AUTODEPLOY_BRANCH:-main}"
INTERVAL="${AUTODEPLOY_INTERVAL:-1800}"
SERVICES="${AUTODEPLOY_SERVICES:-db web nginx}"
DRY_RUN="${AUTODEPLOY_DRY_RUN:-0}"
ONCE="${AUTODEPLOY_ONCE:-0}"

log() { echo "$(date '+%Y-%m-%d %H:%M:%S') autodeploy: $*"; }

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

deploy() {
	old="$1"
	new="$2"

	if ! git merge --ff-only --quiet "$new"; then
		log "fast-forward to ${new} failed (local changes?), skipping"
		return 1
	fi

	if [ "$DRY_RUN" = "1" ]; then
		log "dry run: would rebuild: ${SERVICES}"
		return 0
	fi

	# shellcheck disable=SC2086 # SERVICES is a space-separated list
	if ! docker compose up -d --build $SERVICES; then
		log "docker compose up failed for ${new}"
		return 1
	fi

	# Config files mounted into nginx are not picked up by a rebuild.
	if git diff --name-only "$old" "$new" -- nginx www | grep -q .; then
		log "nginx files changed, restarting nginx"
		docker compose restart nginx
	fi

	if git diff --name-only "$old" "$new" -- deploy docker-compose.yaml | grep -q .; then
		log "deployer files changed; run 'make autodeploy' on this machine to update the deployer itself"
	fi

	docker image prune -f >/dev/null
}

git config --global --add safe.directory "$PWD"

log "watching ${REPO}@${BRANCH} every ${INTERVAL}s"
last_skipped=""

while :; do
	current_branch=$(git symbolic-ref --short HEAD 2>/dev/null || echo detached)
	if [ "$current_branch" != "$BRANCH" ]; then
		log "checkout is on '${current_branch}', not '${BRANCH}'; waiting"
	elif ! git fetch --quiet "https://github.com/${REPO}.git" "$BRANCH"; then
		log "fetch failed; will retry"
	else
		local_sha=$(git rev-parse HEAD)
		remote_sha=$(git rev-parse FETCH_HEAD)

		if [ "$local_sha" != "$remote_sha" ] && [ "$remote_sha" != "$last_skipped" ]; then
			if ! git merge-base --is-ancestor "$local_sha" "$remote_sha"; then
				log "local ${BRANCH} has diverged from remote; not deploying"
				last_skipped="$remote_sha"
			else
				case "$(ci_status "$remote_sha")" in
				success)
					log "deploying ${remote_sha} (was ${local_sha})"
					if deploy "$local_sha" "$remote_sha"; then
						log "deployed ${remote_sha}"
					else
						last_skipped="$remote_sha"
					fi
					;;
				failure)
					log "CI failed for ${remote_sha}; not deploying"
					last_skipped="$remote_sha"
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
