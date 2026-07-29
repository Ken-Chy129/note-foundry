#!/usr/bin/env bash
set -euo pipefail

api_base="${API_BASE:-http://localhost:8088}"
session_token="${SESSION_TOKEN:?SESSION_TOKEN is required}"
fixture_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/fixtures/v0.1" && pwd)"
cookie_header="Cookie: notefoundry_session=${session_token}"

owner_json() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  if [[ -n "$body" ]]; then
    curl -fsS -X "$method" -H "$cookie_header" -H 'Content-Type: application/json' --data "$body" "${api_base}${path}"
  else
    curl -fsS -X "$method" -H "$cookie_header" "${api_base}${path}"
  fi
}

assert_jq() {
  local json="$1"
  local message="$2"
  shift 2
  if ! jq -e "$@" >/dev/null <<<"$json"; then
    printf 'acceptance assertion failed: %s\n' "$message" >&2
    exit 1
  fi
}

space_list="$(owner_json GET '/api/v1/spaces?pageSize=100')"
space_id="$(jq -r '.data[] | select(.name == "AI Agent" and .visibility == "public") | .id' <<<"$space_list" | head -n1)"
if [[ -z "$space_id" ]]; then
  space_id="$(owner_json POST '/api/v1/spaces' '{"name":"AI Agent","visibility":"public"}' | jq -r '.id')"
fi

directory_list="$(owner_json GET "/api/v1/spaces/${space_id}/directories")"
studies_id="$(jq -r '.data[] | select(.name == "Studies" and .parentId == null) | .id' <<<"$directory_list" | head -n1)"
if [[ -z "$studies_id" ]]; then
  studies_id="$(owner_json POST "/api/v1/spaces/${space_id}/directories" '{"name":"Studies","parentId":null}' | jq -r '.id')"
fi
directory_list="$(owner_json GET "/api/v1/spaces/${space_id}/directories")"
directory_id="$(jq -r --arg parent "$studies_id" '.data[] | select(.name == "Hermes Agent" and .parentId == $parent) | .id' <<<"$directory_list" | head -n1)"
if [[ -z "$directory_id" ]]; then
  directory_id="$(owner_json POST "/api/v1/spaces/${space_id}/directories" "$(jq -nc --arg parent "$studies_id" '{name:"Hermes Agent",parentId:$parent}')" | jq -r '.id')"
fi

existing_notes="$(owner_json GET "/api/v1/notes?spaceId=${space_id}&pageSize=100")"
if [[ "${RESUME_EXISTING:-0}" == '1' ]]; then
  overview_id="$(jq -r '.data[] | select(.title == "Hermes Agent Overview" or .published.title == "Hermes Agent Overview") | .id' <<<"$existing_notes" | head -n1)"
  architecture_id="$(jq -r '.data[] | select(.title == "Architecture and Runtime") | .id' <<<"$existing_notes" | head -n1)"
  tool_loop_id="$(jq -r '.data[] | select(.title == "Tool Use and Agent Loop") | .id' <<<"$existing_notes" | head -n1)"
  memory_id="$(jq -r '.data[] | select(.title == "Memory and Context Management") | .id' <<<"$existing_notes" | head -n1)"
  reliability_id="$(jq -r '.data[] | select(.title == "Reliability and Evaluation") | .id' <<<"$existing_notes" | head -n1)"
  for note_id in "$overview_id" "$architecture_id" "$tool_loop_id" "$memory_id" "$reliability_id"; do
    [[ -n "$note_id" ]] || { printf 'cannot resume: one or more acceptance notes are missing\n' >&2; exit 1; }
  done
  overview="$(owner_json GET "/api/v1/notes/${overview_id}")"
  attachment_id="$(owner_json GET "/api/v1/notes/${architecture_id}/attachments" | jq -r '.data[] | select(.publishedAt != null) | .id' | head -n1)"
  [[ -n "$attachment_id" && "$attachment_id" != 'null' ]] || { printf 'cannot resume: acceptance attachment is missing\n' >&2; exit 1; }
else
for title in 'Hermes Agent Overview' 'Architecture and Runtime' 'Tool Use and Agent Loop' 'Memory and Context Management' 'Reliability and Evaluation'; do
  if jq -e --arg title "$title" '.data[] | select(.title == $title)' >/dev/null <<<"$existing_notes"; then
    printf 'acceptance fixture already exists: %s\n' "$title" >&2
    exit 1
  fi
done

create_note() {
  local title="$1"
  owner_json POST '/api/v1/notes' "$(jq -nc --arg space "$space_id" --arg directory "$directory_id" --arg title "$title" '{spaceId:$space,directoryId:$directory,title:$title,markdown:"Acceptance draft"}')"
}

overview="$(create_note 'Hermes Agent Overview')"
architecture="$(create_note 'Architecture and Runtime')"
tool_loop="$(create_note 'Tool Use and Agent Loop')"
memory="$(create_note 'Memory and Context Management')"
reliability="$(create_note 'Reliability and Evaluation')"

overview_id="$(jq -r '.id' <<<"$overview")"
architecture_id="$(jq -r '.id' <<<"$architecture")"
tool_loop_id="$(jq -r '.id' <<<"$tool_loop")"
memory_id="$(jq -r '.id' <<<"$memory")"
reliability_id="$(jq -r '.id' <<<"$reliability")"

attachment="$(curl -fsS -X POST -H "$cookie_header" -F "file=@${fixture_dir}/hermes-runtime.png;type=image/png" "${api_base}/api/v1/notes/${architecture_id}/attachments")"
attachment_id="$(jq -r '.id' <<<"$attachment")"

render_note_body() {
  local fixture="$1"
  local title="$2"
  jq -Rs \
    --arg title "$title" \
    --arg overview "$overview_id" \
    --arg architecture "$architecture_id" \
    --arg memory "$memory_id" \
    --arg reliability "$reliability_id" \
    --arg attachment "$attachment_id" \
    'gsub("\\{\\{OVERVIEW_ID\\}\\}";$overview)
      | gsub("\\{\\{ARCHITECTURE_ID\\}\\}";$architecture)
      | gsub("\\{\\{MEMORY_ID\\}\\}";$memory)
      | gsub("\\{\\{RELIABILITY_ID\\}\\}";$reliability)
      | gsub("\\{\\{ATTACHMENT_ID\\}\\}";$attachment)
      | {expectedVersion:1,title:$title,markdown:.}' "${fixture_dir}/${fixture}"
}

overview="$(owner_json PATCH "/api/v1/notes/${overview_id}" "$(render_note_body hermes-overview.md 'Hermes Agent Overview')")"
architecture="$(owner_json PATCH "/api/v1/notes/${architecture_id}" "$(render_note_body architecture-runtime.md 'Architecture and Runtime')")"
tool_loop="$(owner_json PATCH "/api/v1/notes/${tool_loop_id}" "$(render_note_body tool-loop.md 'Tool Use and Agent Loop')")"
memory="$(owner_json PATCH "/api/v1/notes/${memory_id}" "$(render_note_body memory-context.md 'Memory and Context Management')")"
reliability="$(owner_json PATCH "/api/v1/notes/${reliability_id}" "$(render_note_body reliability-evaluation.md 'Reliability and Evaluation')")"

tag_list="$(owner_json GET '/api/v1/tags?pageSize=100')"
tag_ids=()
for tag_name in 'hermes-agent' 'agent-systems' 'reliability'; do
  tag_id="$(jq -r --arg name "$tag_name" '.data[] | select(.name == $name) | .id' <<<"$tag_list" | head -n1)"
  if [[ -z "$tag_id" ]]; then
    tag_id="$(owner_json POST '/api/v1/tags' "$(jq -nc --arg name "$tag_name" '{name:$name}')" | jq -r '.id')"
    tag_list="$(owner_json GET '/api/v1/tags?pageSize=100')"
  fi
  tag_ids+=("$tag_id")
done
tag_body="$(printf '%s\n' "${tag_ids[@]}" | jq -Rsc 'split("\n")[:-1] | {tagIds:.}')"
for note_id in "$overview_id" "$architecture_id" "$tool_loop_id" "$memory_id" "$reliability_id"; do
  owner_json PUT "/api/v1/notes/${note_id}/tags" "$tag_body" >/dev/null
  owner_json POST "/api/v1/notes/${note_id}/publish" '{"expectedVersion":2}' >/dev/null
done
fi

public_notes="$(curl -fsS "${api_base}/api/v1/public/spaces/${space_id}/notes?pageSize=100")"
for note_id in "$overview_id" "$architecture_id" "$tool_loop_id" "$memory_id" "$reliability_id"; do
  if ! jq -e --arg id "$note_id" '.data[] | select(.id == $id)' >/dev/null <<<"$public_notes"; then
    printf 'acceptance assertion failed: published note %s is absent\n' "$note_id" >&2
    exit 1
  fi
done

forward_links="$(curl -fsS "${api_base}/api/v1/public/notes/${overview_id}/links")"
backlinks="$(curl -fsS "${api_base}/api/v1/public/notes/${architecture_id}/backlinks")"
assert_jq "$forward_links" 'forward Note Link was not derived' --arg id "$architecture_id" '.data | any(.id == $id)'
assert_jq "$backlinks" 'published backlink was not derived' --arg id "$overview_id" '.data | any(.id == $id)'

public_attachment_status="$(curl -sS -o /dev/null -w '%{http_code}' "${api_base}/api/v1/public/attachments/${attachment_id}")"
[[ "$public_attachment_status" == '200' ]] || { printf 'acceptance assertion failed: published attachment is unavailable\n' >&2; exit 1; }

if [[ "${RESUME_EXISTING:-0}" == '1' ]]; then
  draft_marker="$(jq -r '.markdown | capture("(?<marker>INCOMPLETE_DRAFT_ONLY_[0-9]+)").marker' <<<"$overview")"
else
  overview_markdown="$(jq -r '.markdown' <<<"$overview")"
  draft_marker="INCOMPLETE_DRAFT_ONLY_$(date +%s)"
  overview="$(owner_json PATCH "/api/v1/notes/${overview_id}" "$(jq -nc --arg title 'Hermes Agent Overview — draft refinement' --arg markdown "${overview_markdown}" --arg marker "$draft_marker" '{expectedVersion:2,title:$title,markdown:($markdown + "\n\n" + $marker)}')")"
fi
public_overview="$(curl -fsS "${api_base}/api/v1/public/notes/${overview_id}")"
assert_jq "$public_overview" 'draft title leaked into Published Content' '.title == "Hermes Agent Overview"'
if jq -e --arg marker "$draft_marker" '.markdown | contains($marker)' >/dev/null <<<"$public_overview"; then
  printf 'acceptance assertion failed: draft Markdown leaked into Published Content\n' >&2
  exit 1
fi

owner_chinese="$(curl -fsS -G -H "$cookie_header" --data-urlencode 'q=上下文管理' "${api_base}/api/v1/search")"
public_chinese="$(curl -fsS -G --data-urlencode 'q=上下文管理' "${api_base}/api/v1/public/search")"
public_english="$(curl -fsS -G --data-urlencode 'q=idempotency' "${api_base}/api/v1/public/search")"
assert_jq "$owner_chinese" 'owner Chinese search did not find the memory note' --arg id "$memory_id" '.data | any(.id == $id)'
assert_jq "$public_chinese" 'public Chinese search did not find the memory note' --arg id "$memory_id" '.data | any(.id == $id)'
assert_jq "$public_english" 'public English search did not find the tool-loop note' --arg id "$tool_loop_id" '.data | any(.id == $id)'

space_list="$(owner_json GET '/api/v1/spaces?pageSize=100')"
private_space_id="$(jq -r '.data[] | select(.name | startswith("Hermes Private Acceptance ")) | .id' <<<"$space_list" | head -n1)"
if [[ -z "$private_space_id" ]]; then
  private_space_id="$(owner_json POST '/api/v1/spaces' "$(jq -nc --arg name "Hermes Private Acceptance $(date +%s)" '{name:$name,visibility:"private"}')" | jq -r '.id')"
fi
private_notes="$(owner_json GET "/api/v1/notes?spaceId=${private_space_id}&pageSize=100")"
private_note_id="$(jq -r '.data[] | select(.title == "Private Hermes Evaluation") | .id' <<<"$private_notes" | head -n1)"
if [[ -z "$private_note_id" ]]; then
  private_note_id="$(owner_json POST '/api/v1/notes' "$(jq -nc --arg space "$private_space_id" '{spaceId:$space,directoryId:null,title:"Private Hermes Evaluation",markdown:"private-only acceptance evidence"}')" | jq -r '.id')"
fi
anonymous_owner_status="$(curl -sS -o /dev/null -w '%{http_code}' "${api_base}/api/v1/notes/${overview_id}")"
private_public_status="$(curl -sS -o /dev/null -w '%{http_code}' "${api_base}/api/v1/public/notes/${private_note_id}")"
[[ "$anonymous_owner_status" == '401' ]] || { printf 'acceptance assertion failed: owner API is not protected\n' >&2; exit 1; }
[[ "$private_public_status" == '404' ]] || { printf 'acceptance assertion failed: private note is anonymously visible\n' >&2; exit 1; }

owner_json POST "/api/v1/notes/${reliability_id}/trash" '{"expectedVersion":2}' >/dev/null
trashed_public_status="$(curl -sS -o /dev/null -w '%{http_code}' "${api_base}/api/v1/public/notes/${reliability_id}")"
[[ "$trashed_public_status" == '404' ]] || { printf 'acceptance assertion failed: trashed note remains public\n' >&2; exit 1; }
restored="$(owner_json POST "/api/v1/trash/notes/${reliability_id}/restore" '{"confirmPublish":true}')"
assert_jq "$restored" 'Trash restore changed identity or did not republish' --arg id "$reliability_id" '.id == $id and .published != null'
curl -fsS "${api_base}/api/v1/public/notes/${reliability_id}" >/dev/null

jq -n \
  --arg spaceId "$space_id" \
  --arg directoryId "$directory_id" \
  --arg overviewId "$overview_id" \
  --arg architectureId "$architecture_id" \
  --arg toolLoopId "$tool_loop_id" \
  --arg memoryId "$memory_id" \
  --arg reliabilityId "$reliability_id" \
  --arg attachmentId "$attachment_id" \
  --arg draftMarker "$draft_marker" \
  '{status:"passed",spaceId:$spaceId,directoryId:$directoryId,noteIds:[$overviewId,$architectureId,$toolLoopId,$memoryId,$reliabilityId],attachmentId:$attachmentId,draftMarker:$draftMarker}'
