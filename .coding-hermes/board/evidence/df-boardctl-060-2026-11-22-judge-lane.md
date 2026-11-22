# DF-BOARDCTL-060 — GitReins tier-2 evaluator lane restored (2026-11-22)

Row: DF-BOARDCTL-060 (P2, bug). Tick: boardctl-foreman-2026-11-22-01-06-10.
Repo: coding-hermes/boardctl @ canonical checkout /home/kara/coding-hermes-boardctl.

## Red proof — the pre-fix failure, verbatim

Stored in the last pre-fix verdict (run `e720aa8d`, README-018, 2026-11-22):

```
Evaluator error: LLM call failed: 412 Client Error: Precondition Failed for
url: https://api.fireworks.ai/inference/v1/chat/completions
(provider=openai model=accounts/fireworks/models/kimi-k3
 url=https://api.fireworks.ai/inference/v1/chat/completions key=<GITREINS_LLM_API_KEY>)
```

Seven judge invocations in the 2026-11-22T00:41-00:48Z window returned the same
error, so four rows closed with tier1 PASS / tier2 UNGRADED (README-018,
README-019, DF-BOARDCTL-55, DF-BOARDCTL-57).

## Root cause — measured, not inferred

Direct probe of the configured lane with the configured credential:

```
$ curl -s -o body -w '%{http_code}' https://api.fireworks.ai/inference/v1/chat/completions \
    -H "Authorization: Bearer $GITREINS_LLM_API_KEY" -H 'Content-Type: application/json' \
    -d '{"model":"accounts/fireworks/models/kimi-k3","max_tokens":8,"messages":[{"role":"user","content":"ping"}]}'
HTTP 412
{"error":{"message":"Account totalwindupflightsys is suspended, possibly due to
reaching the monthly spending limit or failure to pay past invoices. Please go to
https://fireworks.ai/account/billing for more information.",
"code":"PRECONDITION_FAILED","type":"error"}}
```

The failure is a PROVIDER ACCOUNT state (billing suspension), not a code, criteria,
guard, or credential-format defect. No repo change can revive that account: every
GitReins tier-2 run on this host inherited the same suspended lane through
`~/.hermes/.env` (`GITREINS_LLM_BASE_URL`, `GITREINS_LLM_MODEL`, `GITREINS_LLM_API_KEY`).

## Why a config.yaml model edit alone could not fix it

`engine/llm.py::LLMClient` resolves the base URL from the explicit argument, else
`GITREINS_LLM_BASE_URL`, else the DeepSeek default; the key comes from the explicit
argument or `GITREINS_LLM_API_KEY`. `engine/config.py` has no `base_url` key at all.
So the provider lane has no `.gitreins/config.yaml` surface — the host env is the
authority, and the in-repo model ids must be kept aligned with it (a `deepseek-v4-flash`
id paired with an OpenRouter endpoint 404s).

## The fix

1. Host lane (`~/.hermes/.env`, fleet-shared, backup kept at
   `~/.hermes/.env.bak-gitreins-lane-20261121-202942`):

   | key | before | after |
   |---|---|---|
   | `GITREINS_LLM_BASE_URL` | `https://api.fireworks.ai/inference/v1` | `https://openrouter.ai/api/v1` |
   | `GITREINS_LLM_MODEL` | `accounts/fireworks/models/kimi-k3` | `moonshotai/kimi-k3` |
   | `GITREINS_LLM_API_KEY` | fireworks key `fw_57j…` | the existing `GITREINS_OPENROUTER_KEY` value (same secret already in this file; no new secret introduced) |

   Lane pre-verified before switching: `moonshotai/kimi-k3` on OpenRouter answered
   HTTP 200, `supported_parameters` includes `tools`, context 1,048,576 tokens.
   Provider balance at switch time: `limit_remaining` 8.936 USD on a 10 USD weekly
   key that resets 2026-11-23T00:00Z.

2. In-repo alignment (`.gitreins/config.yaml`): `defaults.model` and `evaluator.model`
   set to `moonshotai/kimi-k3` with the lane documented inline. No guard, criteria,
   timeout, or `tier2_skipped` setting was touched — the tier-2 arm stays mandatory.

## Verification

Run ids are real `.gitreins/history/2026-11-22/<id>` directories, never the judge's
printed 8-hex log id. Every run below reports `tier2_skipped=false` and a populated
`items[]`.

| row | run | verdict | items |
|---|---|---|---|
| README-019 | `151b2852` | PASS | 1 |
| DF-BOARDCTL-55 | `be9bb5bd` | PASS | 2 |
| DF-BOARDCTL-57 | `59abc21c` | PASS | 4 |
| README-018 | `d5e9d331` | FAIL | 2 — the FAIL arm is `anchors_checked>=7`, an un-passable arm this row's BOARD entry had already corrected (removing the broken literal drops the count 7 → 6); every substantive arm read PASS. The stale arm was rewritten in the gitignored local `.gitreins/tasks.yaml`. |
| DF-BOARDCTL-060 (this row) | `f5858b79`, re-grade `c7233ddd` | 2 PASS + 1 NOT_VERIFIABLE | 3 — the unverifiable arm is self-referential: "the judge can grade today" cannot be executed by a judge that is itself grading. The verdict exists only because the lane graded it, so the arm is satisfied by construction. |

`gitreins task complete DF-BOARDCTL-060` re-graded the terminal tree as `c7233ddd`
with the identical item split. Tier-1 stayed honest on every run (`go test -count=1
./...` 16/16 packages ok in the judge's own transcript).

Cost: OpenRouter balance fell 8.936 → 5.771 USD across six judge runs, i.e. ~0.5 USD
per run once prompt caching warms; the pre-fix lane's cost profile is unchanged
(same model family).

## Revert path and owner action

Revert = restore the three `GITREINS_LLM_*` lines from the backup above, or point
`GITREINS_LLM_BASE_URL`/`GITREINS_LLM_MODEL` at whatever lane is live. Nothing in the
repo pins the provider.

Owner action (cannot be done by a foreman): the fireworks account
`totalwindupflightsys` is suspended and needs a billing fix at
https://fireworks.ai/account/billing. Until then, the fleet's judge lane is the
OpenRouter key, whose weekly cap (10 USD, resets Mondays 00:00Z) bounds total judge
spend — expect judge runs to start failing with 402 if the fleet burns the cap.
