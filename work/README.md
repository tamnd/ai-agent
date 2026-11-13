# work

The agent pipeline scripts of ai.tamnd.com (spec 14). They make the catalog, write the stubs, write the pages with a ladder of models, audit each page with a model of another family, and ship the pages to tamnd/ai. They are a port of the scripts of algorithm-work.

| Script | What it does | Command line |
| --- | --- | --- |
| `lib.py` | Shared helpers: paths, the model backends, the model register `registers/models.yaml`, the `auditor_for` rule, the event log, JSON extraction and calls to `aik`. It is a module, not a command. | (imported) |
| `kinds.py` | For each of the 28 kinds, the sections the writer must produce and the gold pages to read. It is a module, not a command. | (imported) |
| `catalog.py` | Lists the ideas of each sub-area that the universe does not have yet, with each model of the cataloguer role (spec 14.3 step 1). | `python3 catalog.py [--backend claude --model <id>] [--jobs 8] [--only top/sub ...] [--force]` |
| `curate.py` | Merges the raw catalog and the pending universe rows, removes duplicates and decides the form, the level and the outcome of each item (step 2). | `python3 curate.py [--model <id>]` |
| `canon.py` | Lists the named items of each canon work and frontier source (step 3). | `python3 canon.py [--jobs 6] [--only <work id>] [--force] [--model <id>]` |
| `match.py` | Matches the canon items to the curated catalog and writes the additions (step 4). | `python3 match.py [--model <id>]` |
| `enrich.py` | Drops the additions that are not items and writes a summary for the others (step 5). | `python3 enrich.py [--model <id>]` |
| `finalize.py` | Merges the additions, sets the core flag by the rule of spec 5.10 and writes `registers/core.yaml` (step 6). | `python3 finalize.py merge` and `python3 finalize.py core` |
| `stub.py` | Writes a stub with `aik new` for each node outcome, then runs `aik fmt` and `aik mint` (step 7). | `python3 stub.py [--dry-run]` |
| `writer.py` | Writes the pages with the writer ladder, the gate, the fix loop and the cross-family audit, and writes the provenance (spec 14.6). | `python3 writer.py --slugs <slug> ...` or `python3 writer.py --queue --limit 50 --jobs 6 [--writers <ref>,<ref>]` |
| `gate.py` | Runs the gate on nodes: the local checks, then `aik gate -json`. | `python3 gate.py [--final] <slug> ...` |
| `audit.py` | Audits pages with a model of another family. A page waits when no such model is available. | `python3 audit.py [--model <ref>] [--jobs 4] <slug> ...` |
| `prov.py` | Appends a provenance line for each file of a node, for interactive sessions (spec 14.7). | `python3 prov.py [--models <ref>,...] [--audit <ref>] [--reviewer <login>] [--prompt writer/v1] [--note <text>] <slug> ...` |
| `pr.py` | Opens a pull request in tamnd/ai from files of the content checkout, merges it on request and deploys (spec 14.10). | `python3 pr.py --branch <b> --title <t> --body-file <f> (--paths <p> ... \| --slugs <slug> ...) [--labels <l>] [--milestone <m>] [--merge]` |
| `report.py` | Ticks checklist lines in an issue, posts a comment, cuts a release or prints the next patch version. | `python3 report.py tick <issue> <text> ...`, `comment <issue> --body-file <f>`, `release <YYYY.MM> --notes-file <f> [--latest]`, `next-patch` |
| `mkarea.py` | Creates the milestone, the area issue and one issue for each sub-area, with page checklists. | `python3 mkarea.py <top area id> [--dry]` |
| `show.py` | Prints a short view of raw catalog files. | `python3 show.py catalog/raw/<top>--<sub>.json ...` |
| `ship.sh` | Ships one group of pages: provenance, `pr.py --merge` with up to 3 tries, `deploy.sh` when the deploy failed, then ticks the issue. | `bash ship.sh <issue> <branch> <title> <bodyfile> <prov:0\|1> <slug> ...` |
| `deploy.sh` | Deploys origin/main of tamnd/ai with `aik site deploy`, then checks that the live page /dropout/ shows the commit. | `bash deploy.sh` |
| `test_lib.py` | Tests of `lib.py`, `kinds.py` and `report.py`. They call no model and no network. | `python3 -m unittest discover -s work -p 'test_*.py'` |
| `prompts/` | The prompts: `writer/v1.md`, `fix/v1.md` and `audit/v1.md`. Set `AI_PROMPT_VERSION` to use another version. | (read by `writer.py` and `audit.py`) |

## Before you run

- Install Python 3.10 or later and PyYAML (`python3 -m pip install pyyaml`). The scripts stop with a message when PyYAML is not there.
- Clone tamnd/ai next to ai-agent, so that the content checkout is `../../ai` from this directory. Else set `AI_REPO`.
- Put `aik` on your PATH (`go install ./cmd/aik` in ai-agent), or set `AIK`. Some scripts need `aik` commands that are not built yet. Then they stop and name the command and the spec section.
- The model backends are the `claude` and `codex` command lines. The model names come only from `registers/models.yaml` of the content checkout, or from the file of `AI_MODELS`.
- The deploy reads the Cloudflare credentials only from the environment file `AI_ENV_FILE` (default `$HOME/data/.env`). The scripts source it with `set -a` and never print its values. Do not put a secret in a file of this directory.
- `state/`, `catalog/` and `logs/` are local. They are in `.gitignore`. The scripts make them when they need them.

| Variable | Default | Use |
| --- | --- | --- |
| `AI_REPO` | `../../ai` | The content checkout of tamnd/ai. |
| `AI_SPEC` | `../spec` | The specification, for the prompts. |
| `AI_MODELS` | `$AI_REPO/registers/models.yaml` | The model register. |
| `AIK` | `aik` | The aik binary. |
| `AI_SITE` | `https://ai.tamnd.com` | The site, for links and the live check. |
| `AI_GH_REPO` | `tamnd/ai` | The GitHub repository of the issues, pull requests and releases. |
| `AI_PR_TREE` | `ai-pr` next to the content checkout | The clean worktree for pull requests. |
| `AI_MAIN` | `$AI_REPO` | The checkout that the PR tree is made from. |
| `AI_ENV_FILE` | `$HOME/data/.env` | The environment file with the deploy credentials. |
| `AI_AUTHOR` | none | Passed to `aik mint -author`. |
| `AI_REVIEWER` | none | The reviewer in the provenance lines of `prov.py`. |
| `AI_RUN_ID` | a new id | The run id in the provenance lines of `writer.py`. |
| `AI_PROMPT_VERSION` | `v1` | The prompt version in `prompts/<role>/`. |
| `AI_PARENT_ISSUE` | none | The parent issue of the area issue of `mkarea.py`. |
| `MS` | none | The milestone of the pull request of `ship.sh`. |
| `NODEPLOY` | none | When set, `pr.py --merge` does not deploy. |
